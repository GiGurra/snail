# Performance Tuning Guide

This guide helps you optimize snail for your specific use case.

## Identify Your Priority

First, determine what matters most:

| Priority | Optimize For |
|----------|-------------|
| Maximum throughput | Large batches, longer timeouts |
| Low latency | Small batches, short timeouts |
| Balanced | Default settings |
| Memory efficiency | Smaller buffers |

## Batcher Configuration

### For Maximum Throughput

```go
BatcherOpts{
    BatchSize:  1000,                    // Large batches
    QueueSize:  5000,                    // Deep queue
    WindowSize: 100 * time.Millisecond,  // Longer timeout
}
```

**Trade-offs**:
- Higher latency for individual requests
- More memory usage
- Better network efficiency

### For Low Latency

```go
BatcherOpts{
    BatchSize:  10,                     // Small batches
    QueueSize:  20,                     // Shallow queue
    WindowSize: 1 * time.Millisecond,  // Short timeout
}
```

**Trade-offs**:
- Lower throughput
- More CPU overhead
- Better responsiveness

### For Balanced Performance

```go
BatcherOpts{
    BatchSize:  100,                     // Default
    QueueSize:  200,                     // Default
    WindowSize: 25 * time.Millisecond,  // Default
}
```

## TCP Configuration

### High Throughput

```go
SnailServerOpts{
    ReadBufSize:   256 * 1024,  // 256KB
    WriteBufSize:  256 * 1024,  // 256KB
    TcpRcvBufSize: 4 * 1024 * 1024,  // 4MB OS buffer
    TcpSndBufSize: 4 * 1024 * 1024,  // 4MB OS buffer
    TcpNoDelay:    false,  // Let TCP batch (Nagle on)
}
```

### Low Latency

```go
SnailServerOpts{
    ReadBufSize:   16 * 1024,  // 16KB
    WriteBufSize:  16 * 1024,  // 16KB
    TcpNoDelay:    true,  // Disable Nagle
}
```

## Serialization

Serialization is often the bottleneck. In order of performance:

### 1. Custom Binary (Fastest)

```go
func writeRequest(buf *snail_buffer.Buffer, req Request) error {
    buf.WriteInt32BE(req.ID)
    buf.WriteInt64BE(req.Timestamp)
    buf.WriteBytes(req.Data)
    return nil
}

func parseRequest(buf *snail_buffer.Buffer) snail_parser.ParseOneResult[Request] {
    if buf.Len() < 12 {
        return snail_parser.ParseOneResult[Request]{Status: snail_parser.ParseOneStatusNEB}
    }
    return snail_parser.ParseOneResult[Request]{
        Status: snail_parser.ParseOneStatusOK,
        Value: Request{
            ID:        buf.ReadInt32BE(),
            Timestamp: buf.ReadInt64BE(),
            Data:      buf.ReadBytes(256),
        },
    }
}
```

**Performance**: 30M+ req/s

### 2. Protocol Buffers

Use `protoc` generated code:

```go
func writeRequest(buf *snail_buffer.Buffer, req *pb.Request) error {
    data, err := proto.Marshal(req)
    if err != nil {
        return err
    }
    buf.WriteInt32BE(int32(len(data)))
    buf.WriteBytes(data)
    return nil
}
```

**Performance**: 10-20M req/s

### 3. JSON with Fast Library

Use `sonic` or `jsoniter` instead of `encoding/json`:

```go
import "github.com/bytedance/sonic"

func writeRequest(buf *snail_buffer.Buffer, req Request) error {
    data, err := sonic.Marshal(req)
    if err != nil {
        return err
    }
    buf.WriteBytes(data)
    buf.WriteByte('\n')
    return nil
}
```

**Performance**: 10-15M req/s

### 4. Standard JSON (Slowest)

```go
codec := snail_parser.NewJsonLinesCodec[Request]()
```

**Performance**: 5M req/s

## Memory Optimization

### Reduce Per-Connection Memory

```go
// Smaller batches = less memory per connection
BatcherOpts{
    BatchSize: 50,
    QueueSize: 50,  // Just 2 buffers
}
```

### Avoid Allocations in Hot Path

```go
// Bad: Allocates on every request
func handle(req Request, reply func(Response) error) error {
    data := make([]byte, 1024)  // Allocation!
    // ...
}

// Good: Reuse buffers
var bufPool = sync.Pool{
    New: func() any { return make([]byte, 1024) },
}

func handle(req Request, reply func(Response) error) error {
    data := bufPool.Get().([]byte)
    defer bufPool.Put(data)
    // ...
}
```

## OS-Level Tuning

### Linux

```bash
# Increase socket buffer limits
sudo sysctl -w net.core.rmem_max=16777216
sudo sysctl -w net.core.wmem_max=16777216
sudo sysctl -w net.ipv4.tcp_rmem="4096 87380 16777216"
sudo sysctl -w net.ipv4.tcp_wmem="4096 87380 16777216"

# Increase max connections
sudo sysctl -w net.core.somaxconn=65535
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=65535

# Reduce TIME_WAIT (for benchmarking)
sudo sysctl -w net.ipv4.tcp_tw_reuse=1
```

### Go Runtime

```bash
# Match physical cores
export GOMAXPROCS=16

# Reduce GC overhead (if memory allows)
export GOGC=200
```

## Profiling

Always profile before optimizing:

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=BenchmarkName ./pkg/...
go tool pprof -http=:8080 cpu.prof

# Memory profile
go test -memprofile=mem.prof -bench=BenchmarkName ./pkg/...
go tool pprof -http=:8080 mem.prof

# Trace
go test -trace=trace.out -bench=BenchmarkName ./pkg/...
go tool trace trace.out
```

### What to Look For

| Symptom | Likely Cause | Solution |
|---------|--------------|----------|
| High CPU in `runtime.mallocgc` | Too many allocations | Pool/reuse buffers |
| High CPU in `encoding/json` | JSON bottleneck | Custom codec |
| High CPU in `runtime.lock2` | Mutex contention | Larger batches |
| High CPU in `syscall` | Too many small writes | Larger batches |

## Common Mistakes

### 1. Not Using Client-Side Batching

```go
// Bad: No batching
for _, req := range requests {
    client.Send(req)  // One TCP write per request
}

// Good: With batching
batcher := snail_batcher.NewSnailBatcher(...)
for _, req := range requests {
    batcher.Add(req)  // Batched automatically
}
```

### 2. Too Small Batches with High Latency Network

```go
// Bad for WAN (100ms RTT)
BatcherOpts{
    BatchSize:  10,
    WindowSize: 5 * time.Millisecond,
}

// Better for WAN
BatcherOpts{
    BatchSize:  1000,
    WindowSize: 50 * time.Millisecond,
}
```

### 3. Not Matching Queue Size to Batch Size

```go
// Bad: Queue not multiple of batch
BatcherOpts{
    BatchSize: 100,
    QueueSize: 150,  // Panic!
}

// Good
BatcherOpts{
    BatchSize: 100,
    QueueSize: 200,  // 2x batch size = triple buffering
}
```

## Benchmark Your Changes

Always measure before and after:

```go
func BenchmarkMyConfig(b *testing.B) {
    // Setup server and client with your config

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        batcher.Add(request)
    }
    batcher.Flush()
    // Wait for responses
}
```

Run with:

```bash
go test -bench=BenchmarkMyConfig -benchtime=10s ./...
```

## Summary

| Goal | Key Settings |
|------|--------------|
| Max throughput | Large BatchSize, long WindowSize, custom codec |
| Low latency | Small BatchSize, short WindowSize, TcpNoDelay=true |
| Memory efficiency | Small BatchSize and QueueSize |
| Network efficiency | Large batches, Nagle enabled |

Start with defaults, profile, then tune based on actual bottlenecks.
