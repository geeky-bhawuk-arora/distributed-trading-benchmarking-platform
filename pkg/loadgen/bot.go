package loadgen

import (
	"context"
	"net/http"
	"time"
)

// Bot defines the interface for different load generator bot profiles.
type Bot interface {
	Start(ctx context.Context, endpoint string, httpClient *http.Client, metricsChan chan<- LatencyMetric)
}

// LatencyMetric holds the telemetry data for a single request.
type LatencyMetric struct {
	Latency   time.Duration
	IsSuccess bool
}

// NewHTTPClient creates a tuned HTTP client for high concurrency.
func NewHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false, // Keep connections alive to prevent port exhaustion
	}

	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
}
