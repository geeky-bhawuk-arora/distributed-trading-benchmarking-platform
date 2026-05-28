#!/bin/bash
# sysctl-tune.sh: Tune Linux kernel parameters for ultra-low-latency HFT matching engines.
# Run this script with root privileges (sudo).

if [ "$EUID" -ne 0 ]; then
  echo "Error: Please run as root (sudo)."
  exit 1
fi

echo "=== Tuning Kernel Parameters for HFT Matching Engine ==="

# 1. Disable swap memory to prevent garbage collection and application freezes
echo "[tuning] Disabling swap..."
swapoff -a
sysctl -w vm.swappiness=0

# 2. Increase maximum number of open files
echo "[tuning] Increasing open file descriptor limits..."
sysctl -w fs.file-max=2097152

# 3. Increase network socket max connections backlog
echo "[tuning] Increasing network backlogs..."
sysctl -w net.core.somaxconn=65535
sysctl -w net.core.netdev_max_backlog=100000

# 4. Tune socket read/write TCP buffer sizes
# Pre-size buffers for low latency rather than memory efficiency
echo "[tuning] Optimizing TCP memory buffers..."
sysctl -w net.core.rmem_max=134217728
sysctl -w net.core.wmem_max=134217728
sysctl -w net.ipv4.tcp_rmem="4096 87380 134217728"
sysctl -w net.ipv4.tcp_wmem="4096 65536 134217728"

# 5. Disable TCP slow start after idle (keeps TCP window warm)
echo "[tuning] Disabling TCP slow start after idle..."
sysctl -w net.ipv4.tcp_slow_start_after_idle=0

# 6. Enable TCP low latency mode
echo "[tuning] Enabling TCP low latency mode..."
sysctl -w net.ipv4.tcp_low_latency=1

# 7. Disable Nagle's algorithm (TCP_NODELAY) system-wide if possible, or ensure socket uses it.
# (Handled application-side in Go dialers and listener sockets).

# 8. Adjust TCP keepalive times for fast dead connection detection
echo "[tuning] Tuning TCP keepalive settings..."
sysctl -w net.ipv4.tcp_keepalive_time=300
sysctl -w net.ipv4.tcp_keepalive_intvl=15
sysctl -w net.ipv4.tcp_keepalive_probes=5

echo "=== System Tuning Successfully Completed! ==="
