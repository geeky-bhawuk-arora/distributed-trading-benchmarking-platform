package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"distributed-trading-benchmarking-platform/pkg/api"
)

func main() {
	port := flag.String("port", "8080", "Port to run the matching engine on")
	flag.Parse()

	log.Printf("Starting Matching Engine on port %s...", *port)

	// Create and start the API Server
	server := api.NewServer()
	server.Start()

	// Register routes
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	// Define HTTP server settings
	srv := &http.Server{
		Addr:         ":" + *port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel to listen for OS interrupt signals to shutdown gracefully
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v", err)
		}
	}()

	log.Printf("Server is running. Press CTRL+C to stop.")

	// Wait for terminate signal
	<-shutdownSig
	log.Println("Shutting down gracefully...")

	// Create context with 5 second timeout for shutdown process
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}

	log.Println("Server stopped successfully.")
}
