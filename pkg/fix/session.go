package fix

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// SessionServer listens for low-latency FIX protocol connections.
type SessionServer struct {
	port     string
	listener net.Listener
	mu       sync.Mutex
	active   bool
}

// NewSessionServer creates a new SessionServer.
func NewSessionServer(port string) *SessionServer {
	return &SessionServer{port: port}
}

// Start boots the TCP server and handles incoming FIX sessions.
func (s *SessionServer) Start(ctx context.Context) error {
	var err error
	s.listener, err = net.Listen("tcp", ":"+s.port)
	if err != nil {
		return fmt.Errorf("failed to bind FIX server to port %s: %w", s.port, err)
	}

	s.active = true
	log.Printf("[fix-server] Bound high-speed FIX session server to TCP port %s", s.port)

	// Close listener if context is cancelled
	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			active := s.active
			s.mu.Unlock()
			if !active {
				return nil // Shutting down
			}
			log.Printf("[fix-server] Accept error: %v", err)
			continue
		}

		go s.handleConnection(ctx, conn)
	}
}

// handleConnection manages the lifecycle of an individual FIX session.
func (s *SessionServer) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	log.Printf("[fix-server] Accepted connection from client %s", conn.RemoteAddr())

	reader := make([]byte, 4096)
	msg := NewMessage()

	seqNum := 1

	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			n, err := conn.Read(reader)
			if err != nil {
				if err != io.EOF {
					log.Printf("[fix-server] Session read error: %v", err)
				}
				log.Printf("[fix-server] Client disconnected: %s", conn.RemoteAddr())
				return
			}

			// Parse FIX message
			err = msg.Parse(reader[:n])
			if err != nil {
				log.Printf("[fix-server] Failed to parse raw FIX packet: %v", err)
				continue
			}

			msgType, found := msg.GetString(35) // MsgType (Tag 35)
			if !found {
				log.Println("[fix-server] Missing MsgType tag 35, dropping packet.")
				continue
			}

			senderCompID, _ := msg.GetString(49) // SenderCompID (Tag 49)
			targetCompID, _ := msg.GetString(56) // TargetCompID (Tag 56)

			switch msgType {
			case "A": // Logon
				log.Printf("[fix-session] Received Logon request (Sender: %s, Target: %s)", senderCompID, targetCompID)
				// Respond with logon acknowledgement
				logonAck := formatFIXMessage("A", seqNum, "ENGINE", senderCompID, "98=0\x01108=30")
				conn.Write([]byte(logonAck))
				seqNum++

			case "0": // Heartbeat
				// Respond with heartbeat acknowledgement
				hbResponse := formatFIXMessage("0", seqNum, "ENGINE", senderCompID, "")
				conn.Write([]byte(hbResponse))
				seqNum++

			case "D": // New Order Single
				orderID, _ := msg.GetString(11) // ClOrdID (Tag 11)
				symbol, _ := msg.GetString(55)  // Symbol (Tag 55)
				side, _ := msg.GetString(54)    // Side (Tag 54, 1=Buy, 2=Sell)
				price, _ := msg.GetString(44)   // Price (Tag 44)
				qty, _ := msg.GetString(38)     // OrderQty (Tag 38)

				sideStr := "BUY"
				if side == "2" {
					sideStr = "SELL"
				}

				log.Printf("[fix-session] Order Received -> ID: %s | Symbol: %s | Side: %s | Price: %s | Qty: %s", orderID, symbol, sideStr, price, qty)

				// Respond with execution report showing order is accepted and matched
				execReport := formatFIXMessage("8", seqNum, "ENGINE", senderCompID,
					fmt.Sprintf("37=exec-%s\x0111=%s\x0117=trade-%d\x01150=2\x0139=2\x0155=%s\x0154=%s\x0138=%s\x0144=%s",
						orderID, orderID, time.Now().UnixNano(), symbol, side, qty, price))
				conn.Write([]byte(execReport))
				seqNum++

			default:
				log.Printf("[fix-session] Received unsupported FIX MsgType: %s", msgType)
			}
		}
	}
}

// Stop closes the listener TCP socket.
func (s *SessionServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	if s.listener != nil {
		log.Println("[fix-server] Stopping FIX session server...")
		s.listener.Close()
	}
}

// Helper to construct a standard FIX message string
func formatFIXMessage(msgType string, seqNum int, sender string, target string, body string) string {
	ts := time.Now().UTC().Format("20060102-15:04:05.000")
	var raw string
	if body != "" {
		raw = fmt.Sprintf("35=%s\x0149=%s\x0156=%s\x0134=%d\x0152=%s\x01%s\x01", msgType, sender, target, seqNum, ts, body)
	} else {
		raw = fmt.Sprintf("35=%s\x0149=%s\x0156=%s\x0134=%d\x0152=%s\x01", msgType, sender, target, seqNum, ts)
	}

	length := len(raw)
	header := fmt.Sprintf("8=FIX.4.2\x019=%d\x01", length)
	fullMsg := header + raw

	// Calculate checksum tag 10
	sum := 0
	for i := 0; i < len(fullMsg); i++ {
		sum += int(fullMsg[i])
	}
	checksum := fmt.Sprintf("10=%03d\x01", sum%256)

	return fullMsg + checksum
}
