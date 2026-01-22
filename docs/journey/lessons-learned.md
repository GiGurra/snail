# Lessons Learned

Building snail involved extensive experimentation with Go's concurrency primitives. Here are the key technical insights, backed by benchmarks and profiler data.

## 1. Go Channels Are Slow Under Contention

**Expectation**: Channels are Go's built-in concurrency primitive; they should be fast.

**Reality**: Under high-contention fan-in scenarios, channels max out at **2-5 million ops/second**.

```go
// Benchmark: 10,000 goroutines sending to one channel
ch := make(chan struct{}, 100_000)

// Result: ~2-5M ops/sec
// We need: 100M+ ops/sec
```

**Why?**

Channels have internal overhead:
- Lock acquisition for each send/receive
- Queue management
- Goroutine scheduling when blocking

**When channels ARE good**:
- Low-contention scenarios
- Transferring prepared batches (single producer, single consumer)
- Correctness over raw performance

## 2. Mutex Behavior Is Platform-Dependent

The Go `sync.Mutex` implementation performs **very differently** on different platforms.

### Linux x86_64 (AMD 7950X3D)

```
Contention Level    Ops/Second
─────────────────────────────
1 goroutine         150M
10 goroutines       80M
100 goroutines      40M
1000 goroutines     25M
10000 goroutines    20M
```

Smooth degradation. Predictable.

### macOS Apple Silicon

```
Contention Level    Ops/Second
─────────────────────────────
1 goroutine         200M
2 goroutines        15M  ← Cliff!
4 goroutines        10M
8 goroutines        8M
100 goroutines      12M  ← Recovery
10000 goroutines    15M
```

There's a **performance cliff** at low-to-medium contention (2-8 goroutines).

**Why?**

The profiler shows excessive time in `usleep`. The macOS implementation appears to back off aggressively, which helps at very high contention but hurts at medium contention.

**Lesson**: Always benchmark on your target platform.

## 3. Spin Locks Beat Regular Mutexes

For high-contention scenarios where you expect to acquire the lock quickly:

```go
type spinLock struct {
    mutex sync.Mutex
}

func (m *spinLock) Lock() {
    for !m.mutex.TryLock() {
        runtime.Gosched()  // Yield to scheduler
    }
}
```

**Results**:
- 5x faster than `sync.Mutex` on macOS at medium contention
- Comparable to `sync.Mutex` on Linux
- Uses CPU while spinning (trade-off)

**When to use**:
- Lock held very briefly
- High contention expected
- Throughput matters more than CPU efficiency

**When NOT to use**:
- Locks held for extended periods
- Need to block efficiently (use regular mutex)
- CPU usage is constrained

## 4. Atomics Aren't Always Faster

**Expectation**: Lock-free algorithms using atomics should be faster than locks.

**Reality**: On Apple Silicon, atomics can be **5x slower** than spin locks.

```go
// Atomic approach
counter := atomic.Uint64{}
counter.Add(1)  // Slow on Apple Silicon under contention

// Spin lock approach (faster on Apple Silicon)
spinLock.Lock()
counter++
spinLock.Unlock()
```

**Why?**

- Atomic operations require cache coherency protocols
- ARM's memory model has different costs than x86
- High contention = lots of cache line bouncing

**Lesson**: "Lock-free" doesn't mean "fast". Benchmark everything.

## 5. False Sharing Is a Real Performance Killer

When multiple atomic variables share a cache line, updates to one invalidate the other:

```go
// Bad: Variables share cache line
type bad struct {
    writePos atomic.Uint64
    nWritten atomic.Uint64
}

// Good: Padding prevents false sharing
type good struct {
    _        [256]byte
    writePos atomic.Uint64
    _        [256]byte
    nWritten atomic.Uint64
}
```

Adding 256-byte padding made our non-blocking prototype **2-3x faster**.

**Cache line sizes**:
- ARM: 32 bytes (often)
- AMD: 64 bytes
- Intel: 16-256 bytes (varies!)

We use 256 bytes to be safe across all platforms.

## 6. Memory Allocation Is Expensive

When moving 10-20 GB/s of data:

```go
// Bad: Allocates on every batch
batch = make([]T, 0, batchSize)

// Good: Reuse buffers
batch = batch[:0]  // Reset length, keep capacity
```

**Profiler data**: With naive allocation, 15-20% of CPU time was in `malloc`.

**Solution**: Pre-allocate buffers and reuse them via channels:

```go
// Pull empty buffer from pool
buffer := <-pullChan

// Fill it
for _, item := range items {
    buffer = append(buffer, item)
}

// Send to worker
pushChan <- buffer

// Worker sends empty buffer back
buffer = buffer[:0]
pullChan <- buffer
```

## 7. Back-Pressure Is Essential

Without back-pressure, a slow consumer leads to:
- Unbounded queue growth
- Memory exhaustion
- Crash or data loss

**Solution**: Blocking when buffers are exhausted:

```go
func (sb *SnailBatcher[T]) ensureBackBufferInternal() {
    for sb.currentBackBuffer == nil {
        sb.unlockSpinLock()          // Let others stop spinning
        sb.newBackBufferLock.Lock()  // Block here
        if sb.currentBackBuffer == nil {
            sb.currentBackBuffer = <-sb.pullChan  // Wait for buffer
        }
        sb.newBackBufferLock.Unlock()
        sb.lockSpinLock()
    }
}
```

This naturally throttles producers when the worker can't keep up.

## 8. The Hybrid Approach Wins

No single primitive solves everything. The winning combination:

| Primitive | Used For |
|-----------|----------|
| Spin lock (`TryLock` + `Gosched`) | Fast-path synchronization |
| Regular mutex | Back-pressure blocking |
| Buffered channel | Buffer transfer to worker |
| Pre-allocated slices | Avoid allocation overhead |

```go
type SnailBatcher[T any] struct {
    lock              sync.Mutex  // Spin on this
    newBackBufferLock sync.Mutex  // Block on this
    pushChan          chan []T    // Send batches
    pullChan          chan []T    // Receive empty buffers
    currentBackBuffer []T         // Pre-allocated
}
```

## 9. Custom Serialization Matters

JSON marshalling bottlenecks at **5 million ops/second** (with >80% CPU in `encoding/json`).

Custom binary serialization:
- No reflection
- No allocation
- Direct buffer writes

```go
// JSON: 5M ops/sec
json.Marshal(obj)

// Custom: 30M+ ops/sec
buf.WriteInt32BE(obj.Field1)
buf.WriteInt64BE(obj.Field2)
buf.WriteBytes(obj.Data)
```

## 10. Profiler Is Your Friend

Every optimization was guided by profiling:

```bash
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
```

Key findings from profiling:
- Mutex contention in `runtime.lock2`
- Memory allocation in `runtime.mallocgc`
- `usleep` on macOS mutex waits
- False sharing in atomic operations

**Rule**: Don't optimize without profiling first.

## Summary

Building a high-performance concurrent system in Go requires:

1. **Benchmark on target platforms** - x86 and ARM behave differently
2. **Profile before optimizing** - Find real bottlenecks
3. **Channels aren't always the answer** - They have overhead
4. **Atomics aren't magic** - Sometimes locks are faster
5. **Mind the cache lines** - False sharing kills performance
6. **Pre-allocate everything** - Allocation is expensive at scale
7. **Implement back-pressure** - Essential for stability
8. **Use hybrid approaches** - Combine primitives strategically

## Next

- [Benchmarks](../performance/benchmarks.md) - Full performance data
- [Tuning Guide](../performance/tuning.md) - Optimize for your use case
