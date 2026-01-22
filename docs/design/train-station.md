# The Train Station Concept

The core innovation in snail is the "train station" batching mechanism, inspired by simulation games like Transport Tycoon.

## The Analogy

In Transport Tycoon, you build train stations and configure trains to wait based on different criteria:

- Wait for a **full load** of goods
- Wait for a **certain amount of time**
- Leave **immediately**

Snail applies this same concept to network requests.

## How It Works

### Without Batching (Traditional)

```
Client                    Network                   Server
  |                          |                        |
  |--[Req1]----------------->|----------------------->| Process
  |<-------------------------|<-----------------------| [Resp1]
  |--[Req2]----------------->|----------------------->| Process
  |<-------------------------|<-----------------------| [Resp2]
  |--[Req3]----------------->|----------------------->| Process
  |<-------------------------|<-----------------------| [Resp3]
```

Each request incurs:
- TCP packet overhead
- Kernel syscall overhead
- Context switches
- Network round-trip latency

**Result**: ~10-20k req/s per goroutine

### With Batching (Snail)

```
Client                    Network                   Server
  |                          |                        |
  |--[Req1]--→ [Station]     |                        |
  |--[Req2]--→ [  ...  ]     |                        |
  |--[Req3]--→ [  ...  ]     |                        |
  |            [DEPART!] ===>|=======================>| Process all
  |                          |                        | [Batch responses]
  |<=======================  |<=======================| [Station]
  |                          |                        | [DEPART!]
```

Requests accumulate at the "station" until:
1. **Batch is full** (e.g., 100 items), OR
2. **Timer expires** (e.g., 25ms)

Then the entire batch is sent as one TCP write.

**Result**: 300M+ req/s

## The Trade-off

| Setting | Throughput | Latency |
|---------|------------|---------|
| Large batches, long timeout | Maximum | Higher |
| Small batches, short timeout | Lower | Minimum |
| Immediate flush (batch=1) | Lowest | Lowest |

You choose based on your application's needs:

- **High-frequency trading**: Small batches, short timeouts
- **Batch processing**: Large batches, longer timeouts
- **General microservices**: Balance (100 items, 25ms)

## Configuration

```go
snail_tcp_reqrep.BatcherOpts{
    BatchSize:  100,                     // "Full load" threshold
    WindowSize: 25 * time.Millisecond,  // "Time limit" threshold
    QueueSize:  200,                     // Back-pressure buffer
}
```

### BatchSize

How many items to collect before sending. Higher = more throughput, but:
- More memory per batch
- Higher latency for early items in batch

### WindowSize

Maximum time an item waits before the batch is sent, even if not full. This guarantees a latency upper bound.

### QueueSize

How many additional items can queue up while a batch is being sent. This provides back-pressure:
- If the queue fills up, producers block
- Prevents unbounded memory growth
- Must be a multiple of `BatchSize`

## Manual Flushing

Sometimes you need to flush immediately:

```go
batcher.Add(importantRequest)
batcher.Flush()  // Don't wait for timer or full batch
```

## Why Not Just Use Nagle's Algorithm?

TCP already has Nagle's algorithm for batching small writes. Why reinvent it?

1. **Application-level semantics**: Nagle doesn't know about request boundaries
2. **Configurable timing**: Nagle's 200ms delay is often too long
3. **Batch-aware processing**: Server can process entire batches efficiently
4. **Back-pressure**: Nagle doesn't provide application-level back-pressure

## Server-Side Batching

The same concept applies to responses:

```go
func handler(req Request, reply func(Response) error) error {
    // Process request...
    result := process(req)

    // This doesn't send immediately!
    // Response goes to server's batcher
    return reply(result)
}
```

Responses from multiple concurrent requests are batched together before being sent back to the client.

## Next

- [Architecture](architecture.md) - How the components fit together
- [Lessons Learned](../journey/lessons-learned.md) - Why channels and mutexes weren't enough
