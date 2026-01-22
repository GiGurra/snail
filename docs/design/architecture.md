# Architecture

## Package Overview

```
snail/
├── pkg/
│   ├── snail_tcp_reqrep/   # High-level request-response API
│   ├── snail_tcp/          # Low-level TCP client/server
│   ├── snail_batcher/      # Generic batching engine
│   ├── snail_parser/       # Codecs (JSON, binary)
│   ├── snail_buffer/       # Efficient buffer implementation
│   ├── snail_channel/      # Circular buffer channel
│   ├── snail_slice/        # Memory-efficient slice ops
│   └── snail_logging/      # Structured logging
```

## Layer Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Your Application                          │
├─────────────────────────────────────────────────────────────┤
│              snail_tcp_reqrep (Request-Response)            │
│   ┌─────────────────┐              ┌─────────────────┐      │
│   │  SnailServer    │              │  SnailClient    │      │
│   │  [Req,Resp]     │              │  [Req,Resp]     │      │
│   └────────┬────────┘              └────────┬────────┘      │
├────────────┼────────────────────────────────┼───────────────┤
│            │        snail_batcher           │               │
│   ┌────────▼────────┐              ┌────────▼────────┐      │
│   │  SnailBatcher   │              │  SnailBatcher   │      │
│   │  (responses)    │              │  (requests)     │      │
│   └────────┬────────┘              └────────┬────────┘      │
├────────────┼────────────────────────────────┼───────────────┤
│            │          snail_tcp             │               │
│   ┌────────▼────────┐              ┌────────▼────────┐      │
│   │  SnailServer    │◄────TCP────► │  SnailClient    │      │
│   │  (low-level)    │              │  (low-level)    │      │
│   └─────────────────┘              └─────────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## The Batcher: Core Engine

The `SnailBatcher[T]` is the heart of snail. It's a generic component that:

1. Accepts individual items from multiple goroutines (fan-in)
2. Collects them into batches
3. Sends batches to a worker goroutine
4. Provides back-pressure when overwhelmed

### N-Buffer Design

```
┌──────────────────────────────────────────────────────────────┐
│                         Producers                             │
│    goroutine1    goroutine2    goroutine3    goroutine4      │
│        │             │             │             │            │
│        └─────────────┼─────────────┼─────────────┘            │
│                      ▼                                        │
│              ┌───────────────┐                                │
│              │  Spin Lock    │  ← Fast path (TryLock)         │
│              └───────┬───────┘                                │
│                      ▼                                        │
│              ┌───────────────┐                                │
│              │ Back Buffer   │  ← Currently being written     │
│              │ [item,item,..]│                                │
│              └───────┬───────┘                                │
│                      │ (when full)                            │
│                      ▼                                        │
│              ┌───────────────┐                                │
│              │  Push Channel │  ← Buffered channel            │
│              └───────┬───────┘                                │
│                      ▼                                        │
│              ┌───────────────┐                                │
│              │    Worker     │  ← Single consumer             │
│              │   goroutine   │                                │
│              └───────┬───────┘                                │
│                      │                                        │
│                      ▼                                        │
│              ┌───────────────┐                                │
│              │  Pull Channel │  ← Returns empty buffers       │
│              └───────────────┘                                │
└──────────────────────────────────────────────────────────────┘
```

### Triple Buffering

With `BatchSize=100` and `QueueSize=200`:

- **Buffer 1**: Currently being written by producers
- **Buffer 2**: In the push channel, waiting for worker
- **Buffer 3**: Being processed by worker or in pull channel

Formula: `TotalBuffers = 1 + (QueueSize / BatchSize)`

## Request-Response Flow

### Client Side

```
1. Application calls batcher.Add(request)
2. Request added to current batch
3. When batch full OR timeout:
   - Batch sent to client's internal worker
   - Worker calls client.SendBatch(batch)
   - Batch written to TCP connection
4. TCP connection delivers to server
```

### Server Side

```
1. TCP data arrives
2. Parser extracts individual requests from stream
3. For each request:
   - Handler called with request + reply function
   - Handler processes, calls reply(response)
   - Response added to per-connection batcher
4. When response batch full OR timeout:
   - Batch written to TCP connection
5. TCP connection delivers to client
```

### Client Response Handling

```
1. TCP data arrives from server
2. Parser extracts individual responses
3. For each response:
   - Client's response handler called
   - Application processes response
```

## Codec Interface

Codecs are simple function pairs:

```go
// Parser: Extract items from byte stream
type ParseFunc[T any] func(buf *snail_buffer.Buffer) ParseOneResult[T]

type ParseOneResult[T any] struct {
    Status ParseOneStatus  // OK or NEB (Not Enough Bytes)
    Value  T
}

// Writer: Serialize items to byte stream
type WriteFunc[T any] func(buf *snail_buffer.Buffer, item T) error
```

### Built-in Codecs

- **JSON Lines**: `NewJsonLinesCodec[T]()` - One JSON object per line
- **Int32**: `NewInt32Codec()` - Simple 4-byte big-endian integers

## Connection Lifecycle

### Server

```go
server, _ := snail_tcp_reqrep.NewServer(
    func() ServerConnHandler[Req, Resp] {
        // Called once per new connection
        // Return the handler for this connection
        return func(req Req, reply func(Resp) error) error {
            if reply == nil {
                // Connection closed
                return nil
            }
            return reply(processRequest(req))
        }
    },
    // ... options
)
```

### Client

```go
client, _ := snail_tcp_reqrep.NewClient(
    host, port, opts,
    func(resp Resp, status ClientStatus) error {
        if status == ClientStatusDisconnected {
            // Handle disconnect
            return nil
        }
        // Process response
        return nil
    },
    // ... codecs
)
```

## Error Handling

- **Parse errors**: Logged, connection continues
- **Handler errors**: Logged, processing continues
- **Connection errors**: Disconnect, cleanup
- **Batcher errors**: Logged, batch dropped

## Thread Safety

- `SnailBatcher.Add()` is safe to call from multiple goroutines
- `SnailBatcher.Flush()` is safe to call from multiple goroutines
- `SnailClient.Send()` is safe to call from multiple goroutines
- Handlers may be called concurrently for different requests

## Next

- [Evolution](../journey/evolution.md) - How we arrived at this design
- [Lessons Learned](../journey/lessons-learned.md) - Why this specific architecture
