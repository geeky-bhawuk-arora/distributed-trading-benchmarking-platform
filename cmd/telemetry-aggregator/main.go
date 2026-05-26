package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("Starting Telemetry Aggregator...")

	// Connect to TimescaleDB
	pgConnStr := os.Getenv("PG_CONN_STR")
	if pgConnStr == "" {
		pgConnStr = "postgres://postgres:postgres@localhost:5432/benchmark?sslmode=disable"
	}
	
	db, err := sql.Open("postgres", pgConnStr)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Printf("Could not ping database: %v", err)
	} else {
		log.Println("Successfully connected to TimescaleDB!")
		
		// Create tables
		_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS benchmarking_runs (
				run_id UUID PRIMARY KEY,
				contestant_id VARCHAR(255) NOT NULL,
				started_at TIMESTAMPTZ NOT NULL,
				ended_at TIMESTAMPTZ,
				status VARCHAR(50) NOT NULL
			);
			
			CREATE TABLE IF NOT EXISTS order_latencies (
				time TIMESTAMPTZ NOT NULL,
				run_id UUID NOT NULL,
				latency_ns BIGINT NOT NULL,
				is_success BOOLEAN NOT NULL
			);
		`)
		if err != nil {
			log.Printf("Failed to create tables: %v", err)
		} else {
			log.Println("TimescaleDB tables verified.")
		}
	}

	// Connect to Redis
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Could not ping Redis: %v", err)
	} else {
		log.Println("Successfully connected to Redis!")
	}

	log.Println("Telemetry Aggregator setup complete. Waiting for incoming data (Event Bus pending Phase 7)...")
	select {} // Block forever
}
