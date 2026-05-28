package fix

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestFIXSessionHandshakeAndOrder(t *testing.T) {
	// Start FIX Server on port 10444 for testing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := NewSessionServer("10444")
	go func() {
		if err := server.Start(ctx); err != nil {
			t.Errorf("Server start failed: %v", err)
		}
	}()

	// Give server time to bind
	time.Sleep(100 * time.Millisecond)
	defer server.Stop()

	// 1. Connect to TCP Port
	conn, err := net.Dial("tcp", "127.0.0.1:10444")
	if err != nil {
		t.Fatalf("Failed to connect to FIX server: %v", err)
	}
	defer conn.Close()

	// 2. Send Logon message (MsgType = A)
	logonMsg := "8=FIX.4.2\x019=65\x0135=A\x0149=CLIENT\x0156=ENGINE\x0134=1\x0152=20260528-18:00:00\x0198=0\x01108=30\x0110=100\x01"
	_, err = conn.Write([]byte(logonMsg))
	if err != nil {
		t.Fatalf("Failed to write logon: %v", err)
	}

	// Read Logon response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read logon response: %v", err)
	}

	response := string(buf[:n])
	t.Logf("Server Response: %s", response)

	// Verify message is a Logon Ack (35=A)
	var respMsg Message
	err = respMsg.Parse(buf[:n])
	if err != nil {
		t.Fatalf("Failed to parse response message: %v", err)
	}

	msgType, _ := respMsg.GetString(35)
	if msgType != "A" {
		t.Errorf("Expected MsgType A (Logon Ack), got %s", msgType)
	}

	// 3. Send New Order Single (MsgType = D)
	orderMsg := "8=FIX.4.2\x019=84\x0135=D\x0149=CLIENT\x0156=ENGINE\x0134=2\x0152=20260528-18:00:00\x0111=ord-100\x0155=AAPL\x0154=1\x0138=100\x0144=150\x0110=200\x01"
	_, err = conn.Write([]byte(orderMsg))
	if err != nil {
		t.Fatalf("Failed to write order: %v", err)
	}

	// Read Execution Report
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read order response: %v", err)
	}

	response = string(buf[:n])
	t.Logf("Server Response: %s", response)

	err = respMsg.Parse(buf[:n])
	if err != nil {
		t.Fatalf("Failed to parse order response: %v", err)
	}

	msgType, _ = respMsg.GetString(35)
	if msgType != "8" {
		t.Errorf("Expected MsgType 8 (Execution Report), got %s", msgType)
	}

	clOrdID, _ := respMsg.GetString(11)
	if clOrdID != "ord-100" {
		t.Errorf("Expected ClOrdID ord-100, got %s", clOrdID)
	}
}
