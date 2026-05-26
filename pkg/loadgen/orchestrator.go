package loadgen

import (
	"context"
	"fmt"
	"sync"
	"time"

	"distributed-trading-benchmarking-platform/pkg/telemetry"
)

type Orchestrator struct {
	Endpoint    string
	TotalTPS    int
	Duration    time.Duration
	NumBots     int
}

func (o *Orchestrator) Run() {
	fmt.Printf("Starting Load Generator. Target: %s, TPS: %d, Bots: %d, Duration: %s\n", o.Endpoint, o.TotalTPS, o.NumBots, o.Duration)

	ctx, cancel := context.WithTimeout(context.Background(), o.Duration)
	defer cancel()

	client := NewHTTPClient()
	metricsChan := make(chan LatencyMetric, o.TotalTPS*10)

	var wg sync.WaitGroup
	tpsPerBot := o.TotalTPS / o.NumBots
	if tpsPerBot < 1 {
		tpsPerBot = 1
	}

	// Spawn bots
	for i := 0; i < o.NumBots; i++ {
		wg.Add(1)
		bot := &NoiseTraderBot{TPS: tpsPerBot}
		go func() {
			defer wg.Done()
			bot.Start(ctx, o.Endpoint, client, metricsChan)
		}()
	}

	// Metrics collector
	var (
		totalRequests int
		successCount  int
		totalLatency  time.Duration
	)

	metricsDone := make(chan struct{})
	go func() {
		for m := range metricsChan {
			totalRequests++
			status := "error"
			if m.IsSuccess {
				successCount++
				status = "success"
			}
			totalLatency += m.Latency

			telemetry.LoadGenRequests.WithLabelValues(status).Inc()
			telemetry.LoadGenLatency.Observe(m.Latency.Seconds())
		}
		close(metricsDone)
	}()

	wg.Wait()
	close(metricsChan)
	<-metricsDone

	fmt.Println("--- Load Test Completed ---")
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Successful: %d\n", successCount)
	if totalRequests > 0 {
		fmt.Printf("Average Latency: %v\n", totalLatency/time.Duration(totalRequests))
		fmt.Printf("Actual TPS: %.2f\n", float64(totalRequests)/o.Duration.Seconds())
	}
}
