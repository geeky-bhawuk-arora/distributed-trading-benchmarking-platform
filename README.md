# Distributed Benchmarking & Hosting Platform

A production-grade, cloud-native benchmarking and hosting platform designed to test, sandbox, and evaluate high-frequency trading engines under distributed market traffic.

---

## Git Workflow & Branching Strategy

To manage the complexity of this project, we follow a strict phase-wise branching strategy:

*   **`main` Branch**: This is the source of truth. It contains only verified, completed, and stable phases. Code is merged here only after a phase is fully tested.
*   **Phase Branches (`phase-1`, `phase-2`, etc.)**: Active development for a specific phase happens in its dedicated branch. Once the phase is complete and verified, a Pull Request is made to merge it into `main`.

> **Example Workflow:**
> 1. `git checkout -b phase-2` (Create and switch to branch for Phase 2)
> 2. Develop, test, and commit features.
> 3. `git checkout main`
> 4. `git merge phase-2` (Merge completed work into the main actual branch)

---

## Table of Contents (Phase-Wise Execution)

Click on any phase below to jump to its specific implementation details and documentation.

*   [Phase 1: Core Matching Engine & Local APIs](#phase-1-core-matching-engine--local-apis) *(Completed)*
*   [Phase 2: Concurrent Distributed Load Generator](#phase-2-concurrent-distributed-load-generator) *(Upcoming)*
*   [Phase 3: Metrics & Telemetry Pipeline](#phase-3-metrics--telemetry-pipeline) *(Upcoming)*
*   [Phase 4: Contestant Submission System & Docker Sandboxing](#phase-4-contestant-submission-system--docker-sandboxing) *(Upcoming)*
*   [Phase 5: Kubernetes Orchestration & Fleet Autoscaling](#phase-5-kubernetes-orchestration--fleet-autoscaling) *(Upcoming)*
*   [Phase 6: Real-time Leaderboard Dashboard](#phase-6-real-time-leaderboard-dashboard) *(Upcoming)*
*   [Phase 7: Event-Driven Ingestion Bus (Kafka/Redpanda)](#phase-7-event-driven-ingestion-bus-kafkaredpanda) *(Upcoming)*
*   [Phase 8: Low-latency Optimization & Kernel Tuning](#phase-8-low-latency-optimization--kernel-tuning) *(Upcoming)*

---

## Phase 1: Core Matching Engine & Local APIs

This phase establishes the foundational in-memory limit order book (LOB) and the API layer required for clients to place orders and receive market data.

### Architecture
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

**Key Design Decisions:**
1.  **Single-Threaded Matching Loop**: The core order book is single-threaded inside a Go event loop, fed by a buffered channel. This guarantees deterministic price-time matching without mutex lock contention.
2.  **Double-Linked List Price Levels**: Each price level uses a doubly-linked list of orders to achieve $O(1)$ insertions at the tail, and $O(1)$ removals from anywhere during cancellations.
3.  **Thread-Safe Depth Cache**: To prevent HTTP readers from blocking the core matching thread, an L2 book snapshot is computed at a throttled interval (50ms) and cached inside a `sync.RWMutex` structure.

### API Reference

#### REST Endpoints
*   **Place Order**: `POST /api/v1/orders`
    ```json
    { "id": "ord-101", "side": 0, "type": 0, "price": 15000, "quantity": 10 }
    // Side: 0=Buy, 1=Sell | Type: 0=Limit, 1=Market
    ```
*   **Cancel Order**: `DELETE /api/v1/orders/{id}`
*   **Fetch Depth Snapshot**: `GET /api/v1/orderbook`

#### WebSocket Streams
*   **Market Data Connection**: `GET /ws/market-data`
    *   `trade`: Emitted immediately on matching executions.
    *   `depth`: Throttled L2 snapshot broadcast every 50ms (if modified).

### How to Run & Verify (Phase 1)

Ensure you have **Go 1.22+** installed.

#### 1. Compile the Binary
```powershell
go build -o matching-engine.exe ./cmd/matching-engine
```
*(Note: The `.exe` file is intentionally ignored by Git via `.gitignore` to keep the repository clean).*

#### 2. Run the Server
Run the executable (defaults to port `8080`):
```powershell
.\matching-engine.exe --port 8080
```

#### 3. Run Correctness Tests
Execute all unit and integration tests:
```powershell
go test -v ./...
```

#### 4. Run Microbenchmarks
Measure execution throughput and memory allocations:
```powershell
go test -bench="." -benchmem ./pkg/orderbook
```
*Expected Performance (AMD Ryzen 7): ~2.46 microseconds per order match (~406,000 matches/second).*

---

## Phase 2: Concurrent Distributed Load Generator
*(Documentation to be added in `phase-2` branch)*

---
## Phase 3: Metrics & Telemetry Pipeline
*(Documentation to be added in `phase-3` branch)*

---
## Phase 4: Contestant Submission System & Docker Sandboxing
*(Documentation to be added in `phase-4` branch)*

---
## Phase 5: Kubernetes Orchestration & Fleet Autoscaling
*(Documentation to be added in `phase-5` branch)*

---
## Phase 6: Real-time Leaderboard Dashboard
*(Documentation to be added in `phase-6` branch)*

---
## Phase 7: Event-Driven Ingestion Bus (Kafka/Redpanda)
*(Documentation to be added in `phase-7` branch)*

---
## Phase 8: Low-latency Optimization & Kernel Tuning
*(Documentation to be added in `phase-8` branch)*
