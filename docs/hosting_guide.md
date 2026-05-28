# Cloud Hosting Strategy Guide: AWS vs. Azure

This document outlines the hosting architecture, instance recommendations, and resource mapping for deploying the Distributed Trading Benchmarking Platform to production environments in either **Amazon Web Services (AWS)** or **Microsoft Azure**.

---

## 1. System Architecture Mapping

To host this platform securely and scale bot fleets dynamically, components are mapped to managed cloud services:

```
                      ┌────────────────────────┐
                      │    NLB / Azure AppGW   │
                      └───────────┬────────────┘
                                  │
                                  v
┌──────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
│               (Amazon EKS / Azure AKS)                       │
│                                                              │
│  ┌───────────────────────┐        ┌───────────────────────┐  │
│  │  Leaderboard Service  │        │  Submission Service   │  │
│  └───────────┬───────────┘        └───────────┬───────────┘  │
│              │ (WebSockets)                   │ (Dynamic Pod)│
│              v                                v              │
│  ┌───────────────────────┐        ┌───────────────────────┐  │
│  │   Load Gen Fleet      │        │  Contestant Sandbox   │  │
│  │ (Autoscaled via KEDA) │        │   (Namespace Egress)  │  │
│  └───────────┬───────────┘        └───────────────────────┘  │
│              │ (Metric Pub)                                  │
│              v                                               │
│  ┌───────────────────────┐                                   │
│  │   Redpanda Cluster    │                                   │
│  └───────────────────────┘                                   │
└──────────────────────────────────────────────────────────────┘
               │                                 │
               v (Event Log Ingestion)           v (Telemetry Persistence)
┌───────────────────────────┐       ┌──────────────────────────┐
│   Amazon ElastiCache /    │       │     Amazon RDS /         │
│   Azure Cache for Redis   │       │  Azure DB for PostgreSQL │
└───────────────────────────┘       └──────────────────────────┘
```

---

## 2. Option A: Amazon Web Services (AWS) Deployment

AWS is highly recommended for high-frequency trading (HFT) platforms due to bare-metal Nitro instances that bypass hypervisor overhead.

### A. Compute & Container Orchestration
* **Cluster**: **Amazon EKS** (Elastic Kubernetes Service).
* **Contestant Node Groups**: 
  - Instance Type: **EC2 Nitro Instances** (e.g., `c6i.metal` or `c6in.2xlarge`).
  - Configuration: Enable the **Kubernetes CPU Manager** with a `static` policy to pin contestant containers to dedicated physical cores, preventing virtual CPU scheduling jitter.
* **Autoscaling**: Configure EKS node groups to scale using **Karpenter** or Cluster Autoscaler targeting CPU/Memory requirements of the bot fleet.

### B. Network & Security
* **Network Policies**: Implement **Calico** or the AWS VPC CNI with network policy support to enforce egress blocking rules on contestant namespaces (denying outbound internet traffic).
* **Ingress**: Deploy the **AWS Load Balancer Controller** to provision a Network Load Balancer (NLB) for handling low-latency WebSocket connections.

### C. Storage & Database Tier
* **Telemetry Persistent Database**: **Amazon RDS for PostgreSQL** with the **TimescaleDB extension** enabled.
* **Memory Leaderboard**: **Amazon ElastiCache for Redis** (Premium / Cluster Mode enabled).
* **Redpanda Broker Storage**: Mount Amazon EBS **GP3 volumes** or use local NVMe SSD instance storage with a StatefulSet.

---

## 3. Option B: Microsoft Azure Deployment

For enterprise integrations in Azure, deploy components onto AKS utilizing NVMe-backed virtual machines.

### A. Compute & Container Orchestration
* **Cluster**: **Azure AKS** (Azure Kubernetes Service).
* **Contestant Node Pools**:
  - Instance Type: **Lsv3-series VMs** (featuring high-speed local NVMe storage and high network bandwidth) or **Fsv2-series** (compute-optimized).
  - Configuration: Enable isolated node pools to host contestant pods separately from system pods.
* **Autoscaling**: Install **KEDA** (Kubernetes Event-driven Autoscaling) on AKS to monitor Redis queues and scale the load generator deployments.

### B. Network & Security
* **Network Policies**: Enable **Azure CNI with Cilium** to implement network filters with minimal latency penalty.
* **Ingress**: Use the **Application Gateway Ingress Controller (AGIC)** to leverage Azure Web Application Firewall (WAF) while preserving WebSockets.

### C. Storage & Database Tier
* **Telemetry Persistent Database**: **Azure Database for PostgreSQL (Flexible Server)** with TimescaleDB extensions enabled.
* **Memory Leaderboard**: **Azure Cache for Redis** (Premium tier).
* **Redpanda Broker Storage**: Use AKS StatefulSets backed by **Azure Premium SSD v2** or **Ultra Disk** storage classes.
