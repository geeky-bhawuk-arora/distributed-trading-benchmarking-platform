package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"distributed-trading-benchmarking-platform/pkg/orderbook"
	"distributed-trading-benchmarking-platform/pkg/telemetry"
)

// RequestType identifies the action requested from the engine.
type RequestType int

const (
	PlaceRequest RequestType = iota
	CancelRequest
)

// OrderRequest represents a payload sent to the matching engine thread.
type OrderRequest struct {
	Type     RequestType
	Order    *orderbook.Order
	CancelID string
	Result   chan Response
}

// Response is returned back to the API client after processing.
type Response struct {
	Success bool              `json:"success"`
	Trades  []orderbook.Trade `json:"trades,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// WSMessage represents the payload wrapper sent over WebSockets.
type WSMessage struct {
	Type string      `json:"type"` // "trade", "depth"
	Data interface{} `json:"data"`
}

// Server hosts the orderbook engine and orchestrates REST/WS APIs.
type Server struct {
	ob           *orderbook.OrderBook
	inputChan    chan OrderRequest
	hub          *Hub
	depthCache   map[string]interface{}
	depthCacheMu sync.RWMutex
}

// NewServer creates a new Server instance.
func NewServer() *Server {
	s := &Server{
		ob:        orderbook.NewOrderBook(),
		inputChan: make(chan OrderRequest, 100000), // Large buffer to avoid blocking ingestion
		hub:       NewHub(),
	}
	s.depthCache = map[string]interface{}{
		"bids": []orderbook.PriceLevel{},
		"asks": []orderbook.PriceLevel{},
	}
	return s
}

// Start runs the core matching event loop and starts the WebSocket hub.
func (s *Server) Start() {
	go s.hub.Run()
	go s.runMatchingLoop()
}

// runMatchingLoop executes as a single thread to mutate the orderbook.
func (s *Server) runMatchingLoop() {
	// Send periodic depth updates (every 50ms) to clients to avoid choking on high update rates
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	var bookChanged bool

	for {
		select {
		case req := <-s.inputChan:
			switch req.Type {
			case PlaceRequest:
				start := time.Now()
				trades := s.ob.Process(req.Order)
				latency := time.Since(start).Seconds()
				
				orderType := "limit"
				if req.Order.Type == 1 {
					orderType = "market"
				}

				telemetry.MatchingDuration.Observe(latency)
				telemetry.OrdersProcessed.WithLabelValues(req.Order.Side.String(), orderType).Inc()

				req.Result <- Response{Success: true, Trades: trades}
				bookChanged = true

				// Broadcast trades immediately if any occurred
				if len(trades) > 0 {
					s.broadcastEvent("trade", trades)
				}

			case CancelRequest:
				success := s.ob.Cancel(req.CancelID)
				if success {
					req.Result <- Response{Success: true}
					bookChanged = true
				} else {
					req.Result <- Response{Success: false, Error: "order not found"}
				}
			}

		case <-ticker.C:
			if bookChanged {
				bids, asks := s.ob.GetDepth(50) // L2 depth top 50
				data := map[string]interface{}{
					"bids": bids,
					"asks": asks,
				}

				// Update thread-safe cache
				s.depthCacheMu.Lock()
				s.depthCache = data
				s.depthCacheMu.Unlock()

				s.broadcastEvent("depth", data)
				bookChanged = false
			}
		}
	}
}

func (s *Server) broadcastEvent(evtType string, data interface{}) {
	msg := WSMessage{
		Type: evtType,
		Data: data,
	}
	bytes, err := json.Marshal(msg)
	if err == nil {
		s.hub.broadcast <- bytes
	}
}

// RegisterRoutes binds HTTP routes to server handlers.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/orders", s.handlePlaceOrder)
	mux.HandleFunc("DELETE /api/v1/orders/", s.handleCancelOrder) // /api/v1/orders/{id}
	mux.HandleFunc("GET /api/v1/orderbook", s.handleGetOrderBook)
	mux.HandleFunc("GET /ws/market-data", func(w http.ResponseWriter, r *http.Request) {
		ServeWs(s.hub, w, r)
	})
}

type placeOrderReq struct {
	ID       string `json:"id"`
	Side     int    `json:"side"` // 0 = Buy, 1 = Sell
	Type     int    `json:"type"` // 0 = Limit, 1 = Market
	Price    uint64 `json:"price"`
	Quantity uint64 `json:"quantity"`
}

func (s *Server) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	var body placeOrderReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.ID == "" || body.Quantity == 0 {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	side := orderbook.Side(body.Side)
	orderType := orderbook.OrderType(body.Type)

	order := orderbook.NewOrder(body.ID, side, orderType, body.Price, body.Quantity)
	resChan := make(chan Response, 1)

	s.inputChan <- OrderRequest{
		Type:   PlaceRequest,
		Order:  order,
		Result: resChan,
	}

	resp := <-resChan
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	// Path layout: /api/v1/orders/{id}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "missing order ID", http.StatusBadRequest)
		return
	}
	orderID := parts[4]

	resChan := make(chan Response, 1)
	s.inputChan <- OrderRequest{
		Type:     CancelRequest,
		CancelID: orderID,
		Result:   resChan,
	}

	resp := <-resChan
	w.Header().Set("Content-Type", "application/json")
	if !resp.Success {
		w.WriteHeader(http.StatusNotFound)
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleGetOrderBook(w http.ResponseWriter, r *http.Request) {
	s.depthCacheMu.RLock()
	cache := s.depthCache
	s.depthCacheMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cache)
}
