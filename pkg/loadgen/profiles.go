package loadgen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type placeOrderReq struct {
	ID       string `json:"id"`
	Side     int    `json:"side"` // 0 = Buy, 1 = Sell
	Type     int    `json:"type"` // 0 = Limit, 1 = Market
	Price    uint64 `json:"price"`
	Quantity uint64 `json:"quantity"`
}

// NoiseTraderBot submits random market and limit orders.
type NoiseTraderBot struct {
	TPS int
}

func (b *NoiseTraderBot) Start(ctx context.Context, endpoint string, httpClient *http.Client, metricsChan chan<- LatencyMetric) {
	ticker := time.NewTicker(time.Second / time.Duration(b.TPS))
	defer ticker.Stop()

	url := fmt.Sprintf("%s/api/v1/orders", endpoint)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Generate random order
			side := rng.Intn(2)
			orderType := rng.Intn(2)
			price := uint64(10000 + rng.Intn(2000)) // Random price around 10000
			if orderType == 1 {
				price = 0 // Market order doesn't need price
			}
			qty := uint64(1 + rng.Intn(100))

			reqBody := placeOrderReq{
				ID:       uuid.New().String(),
				Side:     side,
				Type:     orderType,
				Price:    price,
				Quantity: qty,
			}

			payload, _ := json.Marshal(reqBody)

			start := time.Now()
			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := httpClient.Do(req)
			latency := time.Since(start)

			success := false
			if err == nil {
				if resp.StatusCode == http.StatusOK {
					success = true
				}
				resp.Body.Close()
			}

			metricsChan <- LatencyMetric{
				Latency:   latency,
				IsSuccess: success,
			}
		}
	}
}

// MarketMakerBot posts bid-ask spreads.
type MarketMakerBot struct {
	TPS int
}

func (b *MarketMakerBot) Start(ctx context.Context, endpoint string, httpClient *http.Client, metricsChan chan<- LatencyMetric) {
	// A simple market maker implementation. For now, it delegates to noise trader behavior
	// but can be extended to maintain a spread.
	noiseTrader := &NoiseTraderBot{TPS: b.TPS}
	noiseTrader.Start(ctx, endpoint, httpClient, metricsChan)
}
