package telemetry

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Matching Engine Metrics
	OrdersProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trading_orders_processed_total",
		Help: "The total number of processed orders",
	}, []string{"side", "type"})

	MatchingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "trading_matching_duration_seconds",
		Help:    "Histogram of order matching durations",
		Buckets: prometheus.DefBuckets,
	})

	// Load Generator Metrics
	LoadGenRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trading_loadgen_requests_total",
		Help: "Total HTTP requests made by the load generator",
	}, []string{"status"})

	LoadGenLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "trading_loadgen_latency_seconds",
		Help:    "Histogram of load generator request latencies",
		Buckets: prometheus.DefBuckets,
	})
)

// StartMetricsServer starts an HTTP server specifically for exposing Prometheus metrics.
func StartMetricsServer(port string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	log.Printf("Starting telemetry metrics server on %s", port)
	go func() {
		if err := http.ListenAndServe(port, mux); err != nil {
			log.Fatalf("Metrics server failed: %v", err)
		}
	}()
}
