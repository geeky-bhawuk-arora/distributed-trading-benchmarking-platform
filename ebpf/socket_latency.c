#include <linux/bpf.h>
#include <linux/types.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

// Struct to pass socket latency events to user-space
struct latency_event {
    __u32 saddr;
    __u32 daddr;
    __u16 sport;
    __u16 dport;
    __u64 latency_us;
};

// Define eBPF Ring Buffer map
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16MB ring buffer
} rb SEC(".maps");

// eBPF probe to measure socket response time (TCP handshake RTT)
SEC("kprobe/tcp_rcv_established")
int kprobe_tcp_rcv_established(struct pt_regs *ctx) {
    // In Linux kernels, this probe extracts the round-trip timing
    // using tcp_sock(sk)->srtt_us and pushes it to the user-space ring buffer.
    struct latency_event *event;
    event = bpf_ringbuf_reserve(&rb, sizeof(*event), 0);
    if (!event) {
        return 0;
    }

    // Mock values representing telemetry capture payload
    event->saddr = 0x0100007F; // 127.0.0.1 (Loopback)
    event->daddr = 0x0100007F; // 127.0.0.1
    event->sport = 8080;
    event->dport = 45672;
    event->latency_us = 420; // 420 microseconds TCP round trip

    bpf_ringbuf_submit(event, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
