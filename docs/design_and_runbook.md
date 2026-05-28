# Distributed Trading Benchmarking Platform: Design Solution & Runbook

This document serves as the complete technical design manual and operational runbook for the Distributed Trading Benchmarking Platform (Phases 1-8).

---

## 1. System Architecture & Component Design

The platform operates as a high-throughput, low-latency benchmarking monorepo. The following architecture diagram shows the decoupled telemetry and execution pipelines:

```
┌────────────────────────────────────────────────────────┐
│                      Client / UI                       │
└───────────────────┬────────────────┬───────────────────┘
                    │ (REST/WS API)  │ (Logon/Orders)
                    v                v
┌───────────────────────────┐    ┌───────────────────────┐
│    Leaderboard Service    │    │      FIX Server       │
│        (Port 8282)        │    │     (Port 10443)      │
└─────────────▲─────────────┘    └───────────┬───────────┘
              │ (Pub/Sub)                    │ (API)
┌─────────────┴─────────────┐                v
│       Redis Cache         │    ┌───────────────────────┐
│        (Port 6379)        │    │    Matching Engine    │
└─────────────▲─────────────┘    │      (Port 8080)      │
              │                  └───────────┬───────────┘
┌─────────────┴─────────────┐                │ (Prometheus)
│   Telemetry Aggregator    │                v
│     (Event Consumer)      │    ┌───────────────────────┐
└─────────────▲─────────────┘    │   Prometheus Scraper  │
              │ (Ingest)         │      (Port 9090)      │
┌─────────────┴─────────────┐    └───────────┬───────────┘
│   Redpanda Message Bus    │                v
│        (Port 9092)        │    ┌───────────────────────┐
└─────────────▲─────────────┘    │   Grafana Dashboard   │
              │ (Produce)        │      (Port 3000)      │
┌─────────────┴─────────────┐    └───────────────────────┘
│      Load Generator       │
│       (Bot Fleet)         │
└───────────────────────────┘
```

### Component Roles
1. **Matching Engine (`cmd/matching-engine/`)**: Core exchange logic implementing FIFO price-time matching in a single-threaded execution loop. Exposes REST and WebSocket endpoints.
2. **Load Generator (`cmd/load-generator/`)**: Multi-bot load generator sending concurrent trading traffic to evaluate engine performance.
3. **Telemetry Exporter (`pkg/telemetry/`)**: Captures metrics (TPS, latencies) and exposes them on a Prometheus `/metrics` endpoint.
4. **Submission Service (`cmd/submission-service/`)**: Receives contestant Git repositories, compiles code in Docker builders, and deploys secure runners in sandboxed environments.
5. **K8s Controller (`pkg/controller/`)**: Automates benchmark execution lifecycle by provisioning CRDs, namespaces, NetworkPolicies, and ResourceQuotas.
6. **Redpanda Event Bus (`pkg/queue/`)**: Decouples telemetry ingestion to protect the database against high-frequency ingestion spikes.
7. **FIX Ingestion Engine (`pkg/fix/`)**: Low-overhead TCP gateway supporting standard tag-value FIX messages (Logon, Heartbeats, New Order Single) with a zero-allocation parser.
8. **Leaderboard Service (`cmd/leaderboard-service/`)**: Integrates Redis Sorted Sets to compute real-time scores and feeds a glassmorphic web dashboard over WebSockets.

---

## 2. Key Engineering Decisions & Rationale

* **Go Runtime Optimizations**: To avoid garbage collection (GC) latency spikes on the hot paths, the matching engine uses `sync.Pool` to recycle structures and pre-allocates slice capacities to avoid heap allocations.
* **Redis Sorted Sets for Standings**: Rankings are computed inside Redis (`ZSET`) rather than querying Postgres/TimescaleDB. This converts $O(N \log N)$ sorting database aggregations into $O(\log N)$ memory cache writes, delivering sub-millisecond leaderboard fetches.
* **Redpanda decoupling**: Telemetry is pushed asynchronously to Redpanda. This decouples database writes from metric creation, preventing database congestion during load generator spikes.
* **Zero-Allocation FIX Parser**: The FIX engine parses inputs by mapping byte boundaries within the read buffer instead of allocating new Go strings, avoiding memory allocation overhead on TCP connection threads.
* **eBPF Sensor with Fallback**: Since eBPF requires a Linux kernel ($>5.4$) and cannot run natively on Windows, the Go client implements a fallback to user-space `net.Dialer` handshake timings to ensure successful multi-OS compilation.

---

## 3. Operational Runbook

Ensure your Docker environment is active before running these commands.

### Phase A: Start the Infrastructure Stack
Spin up Redis, TimescaleDB, Grafana, Prometheus, and Redpanda:
```powershell
docker-compose -f deployments/docker-compose.yml up -d
```

### Phase B: Launch the Leaderboard Dashboard
1. Start the service (runs on port `8282`):
   ```powershell
   go run ./cmd/leaderboard-service --port 8282
   ```
2. Open your browser and navigate to: **http://localhost:8282**

### Phase C: Running Benchmarks (Engine & Load Gen)
1. Start the Matching Engine (telemetry exposes on `:2112` by default):
   ```powershell
   go run ./cmd/matching-engine --port 8080
   ```
2. Launch the Load Generator (simulating 5 bots targeting 1000 TPS for 30s):
   ```powershell
   go run ./cmd/load-generator -endpoint http://localhost:8080 -tps 1000 -duration 30s -bots 5
   ```

### Phase D: Verify the FIX Session Gateway
1. Verify compilation and test suite (validates logon handshakes and execution report mappings):
   ```powershell
   go test -v ./pkg/fix/...
   ```
2. Spin up the eBPF socket timing sensor (activates user-space fallback timing on Windows):
   ```powershell
   go run ./ebpf/main.go --port 8080
   ```

### Phase E: Automated Local Demo (All-in-One Script)
Instead of starting each service manually in multiple terminals, you can run the entire platform end-to-end using a single command:
```powershell
.\scripts\run_local_demo.ps1
```
This script:
1. Starts the Docker Compose stack (databases & queues).
2. Spawns the leaderboard and matching engine services in the background.
3. Automatically opens **http://localhost:8282** in your browser.
4. Performs a 15-second load test with 5 bots targeting 1500 TPS.
5. Shuts down and cleans up all background processes automatically.
