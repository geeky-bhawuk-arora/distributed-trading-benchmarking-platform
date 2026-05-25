package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestServer_RESTAndWS(t *testing.T) {
	// Create API server
	s := NewServer()
	s.Start()

	// Setup mock test server
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Establish WebSocket Connection
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/market-data"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect to websocket: %v", err)
	}
	defer ws.Close()

	// 2. Submit Limit Order via POST
	orderPayload := placeOrderReq{
		ID:       "ord-101",
		Side:     0, // Buy
		Type:     0, // Limit
		Price:    10000,
		Quantity: 5,
	}
	body, _ := json.Marshal(orderPayload)
	resp, err := http.Post(ts.URL+"/api/v1/orders", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to post order: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var orderResp Response
	json.NewDecoder(resp.Body).Decode(&orderResp)
	if !orderResp.Success {
		t.Errorf("expected success to be true, got response: %+v", orderResp)
	}

	// 3. Verify WebSocket receives depth update
	ws.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msgBytes, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}

	var wsMsg WSMessage
	if err := json.Unmarshal(msgBytes, &wsMsg); err != nil {
		t.Fatalf("failed to parse websocket message: %v", err)
	}

	if wsMsg.Type != "depth" {
		t.Errorf("expected websocket message type 'depth', got %q", wsMsg.Type)
	}

	// 4. Cancel the order
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/orders/ord-101", nil)
	cancelResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send cancel request: %v", err)
	}
	defer cancelResp.Body.Close()

	if cancelResp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 on cancel, got %d", cancelResp.StatusCode)
	}

	var cancelResult Response
	json.NewDecoder(cancelResp.Body).Decode(&cancelResult)
	if !cancelResult.Success {
		t.Errorf("cancel failed: %+v", cancelResult)
	}
}
