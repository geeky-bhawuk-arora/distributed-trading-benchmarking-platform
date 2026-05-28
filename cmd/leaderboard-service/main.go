package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"distributed-trading-benchmarking-platform/pkg/leaderboard"
	"distributed-trading-benchmarking-platform/pkg/queue"

	"github.com/redis/go-redis/v9"
)

func main() {
	port := flag.String("port", "8282", "Port to run the leaderboard service on")
	redisAddr := flag.String("redis-addr", "localhost:6379", "Redis address")
	flag.Parse()

	log.Printf("Starting Leaderboard Service on port %s...", *port)

	// Initialize Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: "", // No password
		DB:       0,  // Default DB
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully.")

	// Start WebSocket Hub
	hub := leaderboard.NewHub()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	go hub.Run(hubCtx, rdb)

	// Start Redpanda background consumer to process telemetry runs asynchronously
	go func() {
		// Give Redpanda broker time to warm up if recently started
		time.Sleep(3 * time.Second)
		brokers := []string{"localhost:9092"}
		consumer := queue.NewConsumer(brokers, "leaderboard-processors")
		defer consumer.Close()

		consumer.StartListening(hubCtx, func(event *queue.TelemetryEvent) error {
			log.Printf("[event-bus] Received telemetry run from Redpanda: %s for team %s", event.SubmissionID, event.ContestantID)
			score, err := leaderboard.UpdateScore(hubCtx, rdb, event.SubmissionID, event.ContestantID, event.TPS, event.P99LatencyMS, event.SuccessRate)
			if err != nil {
				return err
			}
			log.Printf("[event-bus] Successfully updated standings. Score: %f", score)
			return nil
		})
	}()

	// Router setup
	mux := http.NewServeMux()

	// 1. API: Fetch current leaderboard
	mux.HandleFunc("GET /api/v1/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		entries, err := leaderboard.GetLeaderboard(r.Context(), rdb, 50)
		if err != nil {
			log.Printf("[api] Leaderboard fetch error: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(entries)
	})

	// 2. WebSocket: Live streaming updates
	mux.HandleFunc("GET /ws/leaderboard/live", func(w http.ResponseWriter, r *http.Request) {
		leaderboard.ServeWs(hub, w, r)
	})

	// 3. API: Health Check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	// 4. API: Debug tool to insert a dummy run (helps testing without a full run)
	mux.HandleFunc("POST /api/v1/debug/run", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ContestantID string  `json:"contestant_id"`
			SubmissionID string  `json:"submission_id"`
			TPS          float64 `json:"tps"`
			P99Latency   float64 `json:"p99_latency_ms"`
			SuccessRate  float64 `json:"success_rate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		score, err := leaderboard.UpdateScore(r.Context(), rdb, req.SubmissionID, req.ContestantID, req.TPS, req.P99Latency, req.SuccessRate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "success",
			"score":  score,
		})
	})

	// 5. Frontend: Serve Static dashboard files
	fileServer := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fileServer)

	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown logic
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Leaderboard Dashboard available at http://localhost:%s/", *port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	<-shutdownSig
	log.Println("Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}

	rdb.Close()
	log.Println("Server stopped successfully.")
}
