package main

import (
	"flag"
	"time"

	"distributed-trading-benchmarking-platform/pkg/loadgen"
	"distributed-trading-benchmarking-platform/pkg/telemetry"
)

func main() {
	endpoint := flag.String("endpoint", "http://localhost:8080", "Target API endpoint")
	tps := flag.Int("tps", 1000, "Total Target TPS")
	duration := flag.Duration("duration", 10*time.Second, "Run duration")
	bots := flag.Int("bots", 10, "Number of concurrent bots")
	telemetryPort := flag.String("telemetry-port", ":2113", "Port to expose Prometheus metrics")

	flag.Parse()
	
	telemetry.StartMetricsServer(*telemetryPort)

	orchestrator := &loadgen.Orchestrator{
		Endpoint: *endpoint,
		TotalTPS: *tps,
		Duration: *duration,
		NumBots:  *bots,
	}

	orchestrator.Run()
}
