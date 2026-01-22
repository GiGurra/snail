# Benchmarks

All benchmarks were performed on:

- **CPU**: AMD Ryzen 9 7950X3D
- **RAM**: 64GB DDR5 6000MT/s (dual channel)
- **OS**: Ubuntu 24.04
- **Network**: Loopback TCP connections

## Reference: Raw Network Capacity

First, let's establish what the hardware can do:

| Tool | Throughput |
|------|------------|
| `iperf3` | 800+ Gbit/s |
| Go `net` package (raw) | 400+ Gbit/s |

These numbers represent the theoretical maximum we're working toward.

## Snail Performance

### Small Payloads (4 bytes)

**Configuration**: Request and response are single `int32` values, big-endian encoded.

| Metric | Value |
|--------|-------|
| Request rate | **300-350 million req/s** |
| Bandwidth | ~20 Gbit/s |
| Latency (p99) | < 1ms (batched) |

This is the maximum throughput scenario - minimal serialization, minimal parsing.

### Medium Payloads (276 bytes)

**Configuration**: Custom struct with:
- 1x `int32` field
- 2x `int64` fields
- 1x 256-byte array
- Custom encoder/parser (no reflection)

| Metric | Value |
|--------|-------|
| Request rate | **30 million req/s** |
| Bandwidth | ~130 Gbit/s |
| CPU bottleneck | Serialization |

Each request and response is fully parsed and encoded on both ends.

### HTTP/1.1 (h2load)

**Configuration**: Using `h2load` as the load generator with HTTP/1.1 pipelining.

| Metric | Value |
|--------|-------|
| Request rate | **20-25 million req/s** |
| Bandwidth | ~30 Gbit/s |
| Parser | Minimal HTTP parsing |

Note: We parse only essential parts of the HTTP request for testing purposes.

### HTTP/1.1 (Custom Tool)

**Configuration**: Using snail's [built-in HTTP load tester](https://github.com/GiGurra/snail/blob/main/cmd/cmd_load/cmd_load_h1/cmd_load_h1.go).

| Metric | Value |
|--------|-------|
| Request rate | **45 million req/s** |
| Bandwidth | ~60-65 Gbit/s |
| Note | Our tool "cheats" with optimizations |

The custom tool has more aggressive batching and minimal parsing.

### JSON Payloads

**Configuration**: Using Go's standard `encoding/json` package.

```go
type stupidJsonStruct struct {
    Msg            string `json:"msg"`
    Bla            int    `json:"bla"`
    Foo            string `json:"foo"`
    Bar            int    `json:"bar"`
    GoRoutineIndex int    `json:"go_routine_index"`
    IsFinalMessage bool   `json:"is_final_message"`
}
```

| Metric | Value |
|--------|-------|
| Request rate | **5 million req/s** |
| CPU in JSON | >80% |
| CPU in malloc | ~10% |
| Bottleneck | `encoding/json` |

**Analysis**: JSON marshalling/unmarshalling is the clear bottleneck. A faster JSON library (like `sonic` or `jsoniter`) would likely improve this significantly.

## Batcher Microbenchmarks

Testing the `SnailBatcher` component in isolation:

### Single Producer

| Implementation | Ops/Second |
|----------------|------------|
| Channel-based | 30-40M |
| Mutex-based | 100-150M |
| Spin lock (snail) | **100-150M** |
| Non-blocking atomics | 80-100M |

### 10,000 Concurrent Producers

| Implementation | Ops/Second |
|----------------|------------|
| Channel-based | 2-5M |
| Mutex-based | 20-25M |
| Spin lock (snail) | **80-100M** |
| Non-blocking atomics | 30-40M |

### Platform Comparison (10k producers)

| Platform | Mutex | Spin Lock | Atomics |
|----------|-------|-----------|---------|
| Linux x86_64 | 20-25M | 80-100M | 30-40M |
| macOS ARM | 10-15M | 60-80M | 10-20M |

## Memory Usage

With default settings (batch size 100, queue size 200):

| Component | Memory per Connection |
|-----------|----------------------|
| Server batcher | ~300 * sizeof(Response) |
| Client batcher | ~300 * sizeof(Request) |
| TCP buffers | 128KB (configurable) |

Memory usage scales linearly with:
- Number of connections
- Batch size * (1 + QueueSize/BatchSize)
- Size of request/response types

## Latency Characteristics

Snail trades latency for throughput. With default settings:

| Scenario | Latency |
|----------|---------|
| Batch fills quickly | < WindowSize |
| Low traffic | = WindowSize (25ms default) |
| Manual flush | Immediate |

For latency-sensitive applications, reduce `WindowSize` and `BatchSize`:

```go
BatcherOpts{
    WindowSize: 1 * time.Millisecond,
    BatchSize:  10,
    QueueSize:  20,
}
```

## Scaling

### Connections

Tested with up to 10,000 concurrent connections:
- Linear scaling up to CPU saturation
- Memory usage scales with connection count
- Each connection has its own batcher

### CPU Cores

On 7950X3D (16 cores, 32 threads):
- Throughput scales with cores up to ~16 threads
- Diminishing returns beyond physical core count
- Best results with GOMAXPROCS = physical cores

## Comparison with Other Tools

For context, here are typical numbers for common frameworks:

| Framework/Tool | Typical Throughput |
|---------------|-------------------|
| HTTP (no pipelining) | 10-50K req/s |
| gRPC (unary) | 50-200K req/s |
| gRPC (streaming) | 500K-2M req/s |
| Redis (pipelining) | 500K-1M req/s |
| **Snail** | **30-350M req/s** |

Note: These are rough comparisons. Actual performance depends heavily on payload size, network conditions, and configuration.

## Running Your Own Benchmarks

```bash
# Clone the repo
git clone https://github.com/GiGurra/snail
cd snail

# Run batcher benchmarks
go test -bench=. ./pkg/snail_batcher/

# Run TCP benchmarks
go test -bench=. ./pkg/snail_tcp_reqrep/

# Manual performance tests
go run ./pkg/snail_tcp/manual_perf_tests/server/
go run ./pkg/snail_tcp/manual_perf_tests/client/
```

## Next

- [Tuning Guide](tuning.md) - Optimize for your specific use case
- [Lessons Learned](../journey/lessons-learned.md) - Technical insights
