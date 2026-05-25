# Distributed Benchmarking & Hosting Platform

A production-grade, cloud-native benchmarking and hosting platform designed to test, sandbox, and evaluate high-frequency trading engines under distributed market traffic.

---

## Project Vision & Phase-Wise Approach

We approach this project phase-by-phase like an institutional systems engineering team, building outward from a core matching engine to a fully-distributed, auto-scaling, fault-tolerant orchestration platform.

### Phase Progression
* **Phase 1 (Completed)**: Core Matching Engine & Local APIs (In-memory LOB, REST/WebSocket APIs, thread-safe memory buffers, execution benchmarks).
* **Phase 2 (Next)**: Concurrent Distributed Load Generator (Bot simulator fleets generating REST/WS traffic at target TPS).
* **Phase 3**: Metrics & Telemetry Pipeline (OTel, Prometheus scraper, TimescaleDB persistence, Grafana visualization).
* **Phase 4**: Contestant Submission System & Docker Sandboxing (Secure compilation, gVisor container runtime boundaries).
* **Phase 5**: Kubernetes Orchestration & Fleet Autoscaling (BenchmarkRun CRDs, network namespace isolation, KEDA autoscaling).
* **Phase 6**: Real-time Leaderboard Dashboard (Redis Sorted Sets, live Next.js charts, streaming WebSocket feeds).
* **Phase 7**: Event-Driven Ingestion Bus (Redpanda/Kafka decoupling, HA architecture, consumer-group offsets).
* **Phase 8**: Low-latency Optimization & Kernel Tuning (eBPF TCP handshake timing, sysctl kernel tuning, zero-allocation FIX parser).

---

## Phase 1 Architecture: Matching Engine & APIs

The Phase 1 matching engine is written in Go 1.22+ and is optimized for low-latency operations.

```
                  ┌──────────────────────────────┐
                  │          Client / UI         │
                  └──────┬────────────────▲──────┘
             HTTP POST   │                │ WebSocket Stream
             (Buy/Sell)  v                │ (Execution Reports / Depth)
                  ┌──────────────┐        │
                  │  API Server  │────────┘
                  └──────┬───────┘
             Buffered    │
             Go Channel  v
                  ┌──────────────┐
                  │ Matching Loop│ (Single-Threaded Event Loop)
                  │ ┌──────────┐ │
                  │ │Order Book│ │ (Price-Time Priority)
                  │ └──────────┘ │
                  └──────────────┘
```

### Design Decisions
1. **Single-Threaded Matching Loop**: The core order book (`OrderBook`) is kept strictly single-threaded inside a dedicated Go event loop. All order submissions and cancellations flow through a buffered channel (`inputChan`). This guarantees deterministic price-time matching without the overhead of mutex lock contention.
2. **Double-Linked List Price Levels**: Each price level (`Limit` struct) uses a doubly-linked list of `Order` structs to achieve $O(1)$ insertions at the tail, and $O(1)$ removals from anywhere in the queue during cancellations.
3. **Thread-Safe Depth Cache**: To prevent HTTP readers from blocking the core matching thread (or causing race conditions), the server computes an L2 book snapshot at a throttled interval (50ms) and caches it inside a read-write-locked (`sync.RWMutex`) structure.

---

## Directory Structure

```
├── cmd/
│   └── matching-engine/
│       └── main.go             # Entrypoint, route registration, graceful shutdown
├── pkg/
│   ├── orderbook/
│   │   ├── order.go            # Order definitions & models
│   │   ├── limit.go            # Doubly-linked list price levels
│   │   ├── orderbook.go        # Matching and crossing algorithms
│   │   └── orderbook_test.go   # Correctness unit tests & microbenchmarks
│   └── api/
│       ├── hub.go              # WebSocket connection manager & broadcast hub
│       ├── server.go           # REST server and event-loop coordinator
│       └── server_test.go      # HTTP and WebSocket integration tests
├── go.mod
├── go.sum
└── README.md
```

---

## API Reference (Phase 1)

### REST Endpoints
* **Place Order**: `POST /api/v1/orders`
  * Body:
    ```json
    {
      "id": "order-101",
      "side": 0,      // 0 = Buy, 1 = Sell
      "type": 0,      // 0 = Limit, 1 = Market
      "price": 15000, // E.g., 150.00 (represented as integer cents)
      "quantity": 10
    }
    ```
  * Response:
    ```json
    {
      "success": true,
      "trades": [
        {
          "maker_order_id": "sell-order-abc",
          "taker_order_id": "order-101",
          "price": 15000,
          "quantity": 10
        }
      ]
    }
    ```

* **Cancel Order**: `DELETE /api/v1/orders/{id}`
  * Response:
    ```json
    {
      "success": true
    }
    ```

* **Fetch Depth Snapshot**: `GET /api/v1/orderbook`
  * Response:
    ```json
    {
      "bids": [{"price": 14990, "volume": 50}],
      "asks": [{"price": 15010, "volume": 120}]
    }
    ```

### WebSocket Streams
* **Market Data Connection**: `GET /ws/market-data`
  * Event Types:
    * `trade`: Emitted immediately when matching executions occur.
    * `depth`: Throttled L2 snapshot (top 50 price levels) broadcast every 50ms (if modified).

---

## How to Run & Verify

Ensure you have **Go 1.22+** installed on your system.

### 1. Compile the Binary
To compile the matching engine server:
```bash
go build -o matching-engine.exe ./cmd/matching-engine
```

### 2. Run the Server
Run the executable (defaults to port `8080`):
```bash
./matching-engine.exe --port 8080
```

### 3. Run Correctness Tests
To execute all unit and integration tests across packages:
```bash
go test -v ./...
```

### 4. Run Microbenchmarks
To measure execution throughput and memory allocations under simulated matches:
```bash
go test -bench="." -benchmem ./pkg/orderbook
```

#### Benchmark Results (Windows, AMD Ryzen 7 7730U)
```
BenchmarkOrderBook_Matching-16    626794    2458 ns/op    160 B/op    2 allocs/op
```
* **Performance**: ~2.46 microseconds per order match operation (~406,000 matches/second).
* **Efficiency**: Highly memory-efficient path (only 2 allocations per cycle).
