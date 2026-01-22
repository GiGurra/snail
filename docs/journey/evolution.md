# Evolution

This page chronicles how snail evolved from a simple idea to its current form. The batcher alone went through approximately **10 different implementations** before arriving at the current design.

## The Beginning: A Simple Observation

It started with a question: *Why are microservices so slow?*

Web servers, microservices, and distributed systems are built on the request-response pattern. But they're also built on TCP, and naive request-response over TCP is incredibly inefficient.

A simple experiment showed that a basic RPC over TCP achieves maybe **10-20k requests/second** per goroutine. Meanwhile, `iperf3` shows the same network can handle **800+ Gbit/s** on loopback. Something is very wrong.

## Phase 1: The Naive Approach

**First attempt**: Just batch things in a slice and send periodically.

```go
// Don't do this
var batch []Request
var mu sync.Mutex

func Add(r Request) {
    mu.Lock()
    batch = append(batch, r)
    if len(batch) >= 100 {
        send(batch)
        batch = nil
    }
    mu.Unlock()
}
```

**Problems**:
- Mutex contention killed performance with many goroutines
- No timeout mechanism (items could wait forever)
- Memory allocation on every batch
- No back-pressure

## Phase 2: Channel-Based Batching

**Second attempt**: Use Go channels - they're supposed to be good for this!

```go
// Also don't do this for high throughput
ch := make(chan Request, 10000)

go func() {
    batch := make([]Request, 0, 100)
    ticker := time.NewTicker(25 * time.Millisecond)
    for {
        select {
        case r := <-ch:
            batch = append(batch, r)
            if len(batch) >= 100 {
                send(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                send(batch)
                batch = batch[:0]
            }
        }
    }
}()
```

**Problems**:
- Channels are **surprisingly slow** under high contention
- Only **2-5 million ops/second** with many producers
- We need **100+ million ops/second**

## Phase 3: Pure Atomics / Lock-Free

**Third attempt**: Surely lock-free algorithms are the answer?

Built a non-blocking batcher using atomic operations:
- `atomic.Uint64` for write position
- `atomic.Pointer` for current buffer
- Compare-and-swap for buffer switching

```go
// Excerpt from snail_batcher_nonblocking_prototype.go
type preallocedBatch[T any] struct {
    _        [CacheLinePadding]byte  // Prevent false sharing
    writePos atomic.Uint64
    _        [CacheLinePadding]byte
    nWritten atomic.Uint64
    _        [CacheLinePadding]byte
    data     []T
}
```

**Results**:
- On Linux x86_64: Slightly slower than spin locks under high contention
- On macOS/Apple Silicon: **Catastrophically slow** - 5x slower than alternatives
- The cache line padding (256 bytes!) was needed to prevent false sharing

**Lesson**: Atomics are not magic. They have real costs, especially on ARM.

## Phase 4: Spin Locks

**Fourth attempt**: What about spin locks?

```go
type spinLock struct {
    mutex sync.Mutex
}

func (m *spinLock) Lock() {
    for !m.mutex.TryLock() {
        runtime.Gosched()
    }
}
```

**Results**:
- Much faster than regular mutexes under contention
- Much faster than channels
- Much faster than atomics on Apple Silicon
- But: No way to block when we need back-pressure

## Phase 5: Hybrid Approach (Current)

**Final design**: Combine the best of everything.

```go
// The actual implementation
type SnailBatcher[T any] struct {
    lock              sync.Mutex  // Spin on this one
    newBackBufferLock sync.Mutex  // Block on this one for back-pressure

    pushChan chan []T  // Transfer batches to worker
    pullChan chan []T  // Get empty buffers back

    currentBackBuffer []T
}

func (sb *SnailBatcher[T]) Add(item T) {
    // Spin lock for the fast path
    for !sb.lock.TryLock() {
        runtime.Gosched()
    }
    defer sb.lock.Unlock()

    // ... add item ...

    // If buffer full, swap buffers
    // This may block on newBackBufferLock if no buffers available
}
```

**Why this works**:

1. **Spin lock** for normal operation (fast, low overhead)
2. **Regular mutex** for back-pressure (blocks efficiently)
3. **Channels** for buffer transfer (perfect for single-producer-single-consumer)
4. **Pre-allocated buffers** (no allocation in hot path)
5. **N-buffering** (allows overlap of writing and sending)

## Benchmark Comparison

All tests: 10,000 goroutines, 10 million operations

| Approach | Ops/Second |
|----------|------------|
| Channels | ~2-5M |
| Regular Mutex | ~20-25M |
| Atomics (Linux) | ~30-40M |
| Atomics (macOS) | ~5-10M |
| Spin Lock | ~100-150M |
| Hybrid (snail) | ~100-150M |

## Key Insights

### 1. Go Channels Have Overhead

Channels are great for correctness and simplicity, but they have significant overhead:
- Internal mutex operations
- Memory allocation for queue management
- Goroutine scheduling on send/receive

For low-contention scenarios, they're fine. For high-contention fan-in with hundreds of millions of ops/second, they're a bottleneck.

### 2. Platform Differences Are Real

The Go mutex implementation behaves very differently on different platforms:

**Linux x86_64**:
- Smooth performance degradation as contention increases
- 150M ops/sec (1 goroutine) → 20-25M ops/sec (10k goroutines)

**macOS Apple Silicon**:
- Very fast with no contention
- Falls off a cliff at 2-8 concurrent goroutines
- Recovers somewhat at higher counts
- Profiler shows excessive time in `usleep`

### 3. Atomics Aren't Always Faster

The assumption "lock-free = faster" is wrong. On Apple Silicon:
- Atomic operations have high latency
- Cache coherency traffic is expensive
- A spin lock with `TryLock()` beats raw atomics

### 4. False Sharing Is Real

Adding 256-byte padding between atomic fields made the non-blocking prototype **2-3x faster**:

```go
type preallocedBatch[T any] struct {
    _         [256]byte  // Cache line padding
    writePos  atomic.Uint64
    _         [256]byte  // More padding
    nWritten  atomic.Uint64
}
```

### 5. Back-Pressure Is Essential

Without back-pressure, a slow consumer causes:
- Unbounded memory growth
- Eventual OOM
- Silent data loss if you drop items

The hybrid design provides back-pressure naturally: when all buffers are in use, producers block on `newBackBufferLock`.

## The Result

After all these iterations:

- **300-350 million** request-responses/second (4-byte payloads)
- **30 million** request-responses/second (276-byte payloads)
- **~130 Gbit/s** throughput on loopback

The journey taught us that high-performance concurrent programming requires:
1. Empirical testing (not assumptions)
2. Platform-specific optimization
3. Understanding the actual costs of primitives
4. Pragmatic hybrid approaches

## Next

- [Lessons Learned](lessons-learned.md) - Detailed technical insights
- [Benchmarks](../performance/benchmarks.md) - Full performance numbers
