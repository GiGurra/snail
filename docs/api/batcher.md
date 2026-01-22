# snail_batcher

Generic batching engine. Can be used independently of TCP for any batching use case.

## SnailBatcher

### NewSnailBatcher

Creates a new batcher.

```go
func NewSnailBatcher[T any](
    batchSize int,
    queueSize int,
    timeout time.Duration,
    outputFunc func([]T) error,
) *SnailBatcher[T]
```

**Parameters**:

| Name | Type | Description |
|------|------|-------------|
| `batchSize` | `int` | Maximum items per batch |
| `queueSize` | `int` | Back-pressure buffer size (must be multiple of batchSize) |
| `timeout` | `time.Duration` | Auto-flush interval |
| `outputFunc` | `func([]T) error` | Called with each completed batch |

**Panics if**:
- `batchSize <= 0`
- `queueSize <= 0`
- `queueSize % batchSize != 0`

**Example**:

```go
batcher := snail_batcher.NewSnailBatcher[LogEntry](
    100,                     // Batch up to 100 entries
    200,                     // Allow 200 more in queue
    50 * time.Millisecond,  // Flush at least every 50ms
    func(batch []LogEntry) error {
        return writeToFile(batch)
    },
)
defer batcher.Close()
```

### Methods

#### Add

Adds a single item to the batcher. Thread-safe.

```go
func (sb *SnailBatcher[T]) Add(item T)
```

May block if the queue is full (back-pressure).

```go
batcher.Add(LogEntry{Level: "INFO", Message: "Hello"})
```

#### AddMany

Adds multiple items efficiently. Thread-safe.

```go
func (sb *SnailBatcher[T]) AddMany(items []T)
```

More efficient than calling `Add` in a loop.

```go
entries := []LogEntry{
    {Level: "INFO", Message: "One"},
    {Level: "INFO", Message: "Two"},
    {Level: "INFO", Message: "Three"},
}
batcher.AddMany(entries)
```

#### Flush

Forces an immediate flush of the current batch. Thread-safe.

```go
func (sb *SnailBatcher[T]) Flush()
```

Use when you need to ensure items are processed immediately:

```go
batcher.Add(criticalEntry)
batcher.Flush()  // Don't wait for timeout
```

#### Close

Flushes remaining items and stops the batcher.

```go
func (sb *SnailBatcher[T]) Close()
```

Always defer `Close()` to ensure cleanup:

```go
batcher := snail_batcher.NewSnailBatcher(...)
defer batcher.Close()
```

## How It Works

### N-Buffer Design

The batcher uses n-buffering (typically triple buffering):

```
Total Buffers = 1 + (QueueSize / BatchSize)
```

Example with `BatchSize=100, QueueSize=200`:
- 3 buffers total
- 1 being written to
- 1 in queue to worker
- 1 being processed or waiting

### Flow

```
Producers                    Batcher                     Worker
─────────────────────────────────────────────────────────────────
goroutine1 ──┐
goroutine2 ──┼──► [Spin Lock] ──► [Back Buffer] ──┐
goroutine3 ──┘                                    │
                                                  │ (when full)
                                                  ▼
                                           [Push Channel]
                                                  │
                                                  ▼
                                            [Worker Loop]
                                                  │
                                                  ▼
                                            outputFunc()
                                                  │
                                                  ▼
                                           [Pull Channel] ──► Empty buffer returned
```

### Back-Pressure

When all buffers are in use:
1. Producers block on acquiring a new buffer
2. They resume when the worker finishes processing
3. This prevents unbounded memory growth

## Use Cases

### Log Batching

```go
batcher := snail_batcher.NewSnailBatcher[string](
    1000,
    2000,
    100 * time.Millisecond,
    func(batch []string) error {
        return appendToLogFile(batch)
    },
)

// From anywhere in your app
batcher.Add(fmt.Sprintf("[%s] %s", time.Now(), message))
```

### Database Inserts

```go
batcher := snail_batcher.NewSnailBatcher[Record](
    500,
    1000,
    200 * time.Millisecond,
    func(batch []Record) error {
        return db.BulkInsert(batch)
    },
)
```

### Metrics Collection

```go
batcher := snail_batcher.NewSnailBatcher[Metric](
    100,
    200,
    1 * time.Second,
    func(batch []Metric) error {
        return metricsServer.SendBatch(batch)
    },
)
```

### Network Requests (with snail)

```go
batcher := snail_batcher.NewSnailBatcher[Request](
    100,
    200,
    25 * time.Millisecond,
    func(batch []Request) error {
        return client.SendBatch(batch)
    },
)
```

## Configuration Guide

### Choosing BatchSize

| Use Case | Recommended |
|----------|-------------|
| Low latency | 10-50 |
| Balanced | 100-500 |
| High throughput | 1000+ |

### Choosing QueueSize

Usually 1-2x BatchSize:
- `QueueSize = BatchSize`: Double buffering (minimal memory)
- `QueueSize = 2 * BatchSize`: Triple buffering (recommended)
- `QueueSize = N * BatchSize`: (N+1) buffering

### Choosing Timeout

| Use Case | Recommended |
|----------|-------------|
| Real-time | 1-10ms |
| Interactive | 10-50ms |
| Batch processing | 100ms-1s |

## Thread Safety

All methods are thread-safe:
- `Add()` - Can be called from any goroutine
- `AddMany()` - Can be called from any goroutine
- `Flush()` - Can be called from any goroutine
- `Close()` - Should only be called once

## Error Handling

Errors from `outputFunc` are currently logged but not propagated:

```go
func(batch []T) error {
    err := processItems(batch)
    if err != nil {
        // This error is logged internally
        // Items in the batch are lost
        return err
    }
    return nil
}
```

For critical data, implement retry logic in your `outputFunc`:

```go
func(batch []T) error {
    for retries := 0; retries < 3; retries++ {
        err := processItems(batch)
        if err == nil {
            return nil
        }
        time.Sleep(time.Duration(retries+1) * 100 * time.Millisecond)
    }
    // Log failure, maybe write to dead letter queue
    return errors.New("failed after retries")
}
```
