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

> **Note**: Phase 2 implementation is complete. To test this specific phase, you can check out commit `fbbba9d34f22afd0cafd3dbbc75f4256ac87d6ff`.

This phase introduces a highly concurrent HTTP load generator used to simulate high-frequency trading bot traffic against the Phase 1 matching engine.

### How to Run & Verify (Phase 2)

#### 1. Start the Matching Engine (Target)
First, start the Phase 1 matching engine in a separate terminal window:
```powershell
go run ./cmd/matching-engine --port 8080
```

#### 2. Run the Load Generator
In a new terminal window, execute the load generator to simulate traffic. You can specify the target TPS, run duration, and number of concurrent bots.

```powershell
go run ./cmd/load-generator -endpoint http://localhost:8080 -tps 1000 -duration 10s -bots 5
```

**Parameters:**
* `-endpoint`: The target API URL (default: `http://localhost:8080`)
* `-tps`: Total Target Transactions Per Second across all bots (default: `1000`)
* `-duration`: The time duration to run the load test for (default: `10s`)
* `-bots`: Number of parallel simulated trader bots (default: `10`)

Once the run completes, it will output the total successful requests and the precise average client-side latency (Round-Trip Time).

#### How to test this specific Phase
If you are viewing the codebase in the future and want to test exactly how it looked at the end of Phase 2:
1. `git checkout fbbba9d34f22afd0cafd3dbbc75f4256ac87d6ff` (This detaches your HEAD and goes back in time to this phase).
2. Run the code using the instructions above.
3. `git checkout main` (Returns you to the present/latest code).

---
## Phase 3: Metrics & Telemetry Pipeline

> **Note**: Phase 3 implementation is complete. To test this specific phase, you can check out commit `9533889fb15b96f725bf8273958607f29c733f59`.

This phase wires the full observability stack — **Prometheus**, **Grafana**, **TimescaleDB**, and **Redis** — into our existing matching engine and load generator. Every order match and HTTP round trip now emits real-time metrics viewable in a live Grafana dashboard.

### Architecture

```
Matching Engine  ---> [/metrics on :2112] <--- Prometheus ----> Grafana Dashboard
Load Generator   ---> [/metrics on :2113] <--- Prometheus
TimescaleDB      <--- Telemetry Aggregator (stores historical run data)
Redis            <--- Telemetry Aggregator (fast leaderboard caching)
```

### How to Run & Verify (Phase 3)

#### 1. Start the Observability Stack
From the repository root, spin up Prometheus, Grafana, TimescaleDB and Redis with Docker Compose:
```powershell
docker-compose -f deployments/docker-compose.yml up -d
```

#### 2. Start the Matching Engine with Telemetry
Open a terminal and run (note the telemetry port is exposed on `:2112` by default):
```powershell
go run ./cmd/matching-engine --port 8080
```

#### 3. Run the Load Generator with Telemetry
Open a second terminal (telemetry will emit on `:2113` by default):
```powershell
go run ./cmd/load-generator -endpoint http://localhost:8080 -tps 1000 -duration 60s -bots 5
```

#### 4. Open Grafana
Navigate to **http://localhost:3000** in your browser. Login with `admin / admin`.

You will find a pre-provisioned **"Trading Platform KPIs"** dashboard showing:
- **Orders Processed/sec** (by side and order type)
- **Matching Engine Latency** (p50, p90, p99 percentiles)
- **Load Generator TPS** (success vs error)
- **Load Generator RTT Latency** (p50, p90, p99 client-side round trip)

#### 5. Check Raw Prometheus Metrics (optional)
- Matching engine: http://localhost:2112/metrics
- Load generator: http://localhost:2113/metrics
- Prometheus UI: http://localhost:9090

#### 6. Shut Down the Stack
```powershell
docker-compose -f deployments/docker-compose.yml down
```

#### How to test this specific Phase
1. `git checkout <phase-3-commit-hash>` (will be added once merged to main)
2. Run the stack using the instructions above.
3. `git checkout main` (Returns you to the latest code).

---
## Phase 4: Contestant Submission System & Docker Sandboxing

> **Note**: Phase 4 implementation is complete. To test this specific phase, check out commit `425d956471d989ff665015a9d8e3d0bdd81a6d77`.

This phase introduces a secure submission pipeline. Contestants can upload a Go source archive or a Git repo URL. The platform clones/saves it, builds it inside an isolated Docker builder container, and runs it inside a fully locked-down sandbox container with no network access.

### Security Constraints on Runner Containers
| Constraint | Value |
|---|---|
| Network | `--network none` (no internet) |
| Filesystem | `--read-only` |
| Memory | `--memory=512m` |
| CPU | `--cpus=1.0` |
| Privileges | `--cap-drop=ALL`, `--security-opt=no-new-privileges` |
| User | Runs as non-root `sandboxuser` |

### API Reference (Phase 4)
- `POST /api/v1/submissions` — Upload source `.zip` / `.tar.gz` (multipart, `X-Contestant-ID` header required)
- `POST /api/v1/submissions/git` — Submit a public Git repository URL
- `GET /api/v1/submissions/{id}` — Poll the status of a submission (`PENDING`, `COMPILING`, `SUCCESS`, `FAILED`)
- `GET /health` — Health check

### How to Run & Verify (Phase 4)

#### 1. Start the Submission Service
```powershell
go run ./cmd/submission-service --port 9090
```

#### 2. Submit a Git Repo
```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:9090/api/v1/submissions/git `
  -ContentType "application/json" `
  -Body '{"contestant_id":"team-alpha","git_url":"https://github.com/your/engine-repo"}'
```

#### 3. Poll Build Status
```powershell
Invoke-RestMethod -Uri http://localhost:9090/api/v1/submissions/{id}
```

#### How to test this specific Phase
1. `git checkout <phase-4-commit-hash>` (will be updated once merged)
2. Run the service using the instructions above.
3. `git checkout main` to return.

## Phase 5: Kubernetes Orchestration & Fleet Autoscaling

> **Note**: Phase 5 implementation is complete. To inspect files, check out commit `8152c7151eebca9282280923cecd9a6470d22876`.

This phase adds cloud-native Kubernetes orchestration and fleet autoscaling to the benchmarking platform:
- **Custom CRD `BenchmarkRun`** defines the custom resource schema for runs.
- **Go Kubernetes Controller** using `sigs.k8s.io/controller-runtime` reconciles CRDs, dynamically spinning up runner namespaces, NetworkPolicies, and ResourceQuotas.
- **Helm Charts** package the submission-service, controller, databases, and load-generator.
- **KEDA ScaledObject** configuration automatically scales out load-generator bot fleets based on Redis TPS metrics.

---
## Phase 6: Real-time Leaderboard Dashboard

This phase implements a real-time, low-latency scoring leaderboard and glassmorphic dashboard:
- **Redis ZSET & Hashes** calculate and persist real-time standings using the scoring formula: $\text{Score} = \text{TPS} \times \frac{1000}{\text{p99\_latency\_ms}}$.
- **WebSocket Broadcast Hub** streams new run completions to connected browsers.
- **Vibrant Neon Web Interface** renders real-time standings and dynamic latency vs. TPS charts using Chart.js.

### How to Run & Verify (Phase 6)
1. **Start the Database stack**:
   ```powershell
   docker-compose -f deployments/docker-compose.yml up -d
   ```
2. **Start the Leaderboard Service**:
   ```powershell
   go run ./cmd/leaderboard-service --port 8282
   ```
3. **Open the Dashboard**:
   Navigate to **http://localhost:8282** in your browser.
4. **Trigger a mock run** to watch it stream:
   ```powershell
   Invoke-RestMethod -Method Post -Uri http://localhost:8282/api/v1/debug/run `
     -ContentType "application/json" `
     -Body '{"contestant_id":"team-lambda","submission_id":"run-001","tps":22500,"p99_latency_ms":0.65,"success_rate":100.0}'
   ```

## Phase 7: Event-Driven Ingestion Bus (Kafka/Redpanda)

> **Note**: Phase 7 implementation is complete. To inspect files, check out commit `325fd18b20766a674fef541e86cf1ef4eab7ea38`.

This phase decouples telemetry ingestion by integrating a high-throughput, fault-tolerant messaging bus:
- **Redpanda Integration**: Configured a single-broker Redpanda instance (fully Kafka API compatible) in `deployments/docker-compose.yml`.
- **Asynchronous Producer**: [`pkg/queue/producer.go`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/pkg/queue/producer.go) implements low-overhead, asynchronous, batched writes to the `run.telemetry` topic using `segmentio/kafka-go`.
- **Consumer Group Listener**: [`pkg/queue/consumer.go`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/pkg/queue/consumer.go) handles parallel partition message streaming and triggers handlers to record stats.
- **Protobuf Telemetry Schema**: Schema defined in [`proto/events.proto`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/proto/events.proto) with pre-generated Go struct models in [`pkg/queue/messages.go`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/pkg/queue/messages.go).

---
## Phase 8: Low-latency Optimization & Kernel Tuning

> **Note**: Phase 8 implementation is complete. To inspect files, check out commit `325fd18b20766a674fef541e86cf1ef4eab7ea38`.

This phase maximizes order matching speed and establishes high-performance low-level systems:
- **eBPF Socket Latency Probe**: [`ebpf/socket_latency.c`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/ebpf/socket_latency.c) compiles C-based kernel hooks to measure TCP handshake RTTs. [`ebpf/main.go`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/ebpf/main.go) consumes the ring buffer and provides a user-space socket timing fallback for Windows hosts.
- **Kernel Parameter Tuning Script**: [`scripts/sysctl-tune.sh`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/scripts/sysctl-tune.sh) optimizes the Linux socket backlogs, disables swap, configures TCP buffers, and enables low-latency TCP modes.
- **Zero-Allocation FIX Parser**: [`pkg/fix/parser.go`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/pkg/fix/parser.go) parses tag-value FIX strings without heap allocations in the hot path.
- **FIX TCP Session Server**: [`pkg/fix/session.go`](file:///c:/Users/bhawu/Documents/GitHub/distributed-trading-benchmarking-platform/pkg/fix/session.go) listens on port `10443` supporting standard FIX 4.2 logon handshakes, heartbeats, and order execution reports.

### How to Run & Verify FIX (Phase 8)
1. Run the Go unit tests to verify the FIX session and parser handshake:
   ```powershell
   go test -v ./pkg/fix/...
   ```
