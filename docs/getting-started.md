# Getting Started

## Installation

```bash
go get github.com/GiGurra/snail
```

## Core Concepts

Snail has three layers:

1. **`snail_tcp`** - Low-level TCP client/server
2. **`snail_tcp_reqrep`** - Request-response pattern on top of TCP
3. **`snail_batcher`** - Generic batching engine (can be used independently)

## Basic Server

```go
package main

import (
    "fmt"
    "time"
    "github.com/GiGurra/snail/pkg/snail_parser"
    "github.com/GiGurra/snail/pkg/snail_tcp"
    "github.com/GiGurra/snail/pkg/snail_tcp_reqrep"
)

type Request struct {
    Msg string `json:"msg"`
}

type Response struct {
    Msg string `json:"msg"`
}

func main() {
    reqCodec := snail_parser.NewJsonLinesCodec[Request]()
    respCodec := snail_parser.NewJsonLinesCodec[Response]()

    server, err := snail_tcp_reqrep.NewServer(
        // Handler factory - called for each new connection
        func() snail_tcp_reqrep.ServerConnHandler[Request, Response] {
            // Return the actual handler
            return func(req Request, reply func(Response) error) error {
                if reply == nil {
                    // Client disconnected
                    return nil
                }
                // Process and reply
                return reply(Response{Msg: "Echo: " + req.Msg})
            }
        },
        &snail_tcp.SnailServerOpts{
            Port: 8080,  // 0 = auto-assign
        },
        reqCodec.Parser,
        respCodec.Writer,
        &snail_tcp_reqrep.SnailServerOpts[Request, Response]{
            Batcher: snail_tcp_reqrep.BatcherOpts{
                WindowSize: 25 * time.Millisecond,
                BatchSize:  100,
                QueueSize:  200,
            },
        },
    )
    if err != nil {
        panic(err)
    }
    defer server.Close()

    fmt.Printf("Server listening on port %d\n", server.Port())
    select {} // Block forever
}
```

## Basic Client

```go
package main

import (
    "fmt"
    "github.com/GiGurra/snail/pkg/snail_parser"
    "github.com/GiGurra/snail/pkg/snail_tcp"
    "github.com/GiGurra/snail/pkg/snail_tcp_reqrep"
)

type Request struct {
    Msg string `json:"msg"`
}

type Response struct {
    Msg string `json:"msg"`
}

func main() {
    reqCodec := snail_parser.NewJsonLinesCodec[Request]()
    respCodec := snail_parser.NewJsonLinesCodec[Response]()

    client, err := snail_tcp_reqrep.NewClient(
        "localhost", 8080,
        &snail_tcp.SnailClientOpts{},
        // Response handler
        func(resp Response, status snail_tcp_reqrep.ClientStatus) error {
            if status == snail_tcp_reqrep.ClientStatusDisconnected {
                fmt.Println("Disconnected")
                return nil
            }
            fmt.Printf("Got response: %s\n", resp.Msg)
            return nil
        },
        reqCodec.Writer,
        respCodec.Parser,
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Send a single request
    client.Send(Request{Msg: "Hello!"})
}
```

## Adding Client-Side Batching

For maximum throughput, use the batcher:

```go
import "github.com/GiGurra/snail/pkg/snail_batcher"

// Create batcher that sends to client
batcher := snail_batcher.NewSnailBatcher(
    100,                     // Batch size
    200,                     // Queue size (back-pressure buffer)
    25 * time.Millisecond,   // Flush timeout
    func(batch []Request) error {
        return client.SendBatch(batch)
    },
)
defer batcher.Close()

// Add requests - they'll be batched automatically
for i := 0; i < 10000; i++ {
    batcher.Add(Request{Msg: fmt.Sprintf("Message %d", i)})
}
```

## Custom Codecs

JSON is convenient but slow. For performance, implement custom codecs:

```go
import "github.com/GiGurra/snail/pkg/snail_buffer"
import "github.com/GiGurra/snail/pkg/snail_parser"

// Parser: read from buffer, return parsed item or "need more bytes"
func myParser(buf *snail_buffer.Buffer) snail_parser.ParseOneResult[MyType] {
    if buf.Len() < 4 {
        return snail_parser.ParseOneResult[MyType]{
            Status: snail_parser.ParseOneStatusNEB, // Not Enough Bytes
        }
    }

    // Read 4-byte length prefix
    length := buf.ReadInt32BE()

    if buf.Len() < int(length) {
        return snail_parser.ParseOneResult[MyType]{
            Status: snail_parser.ParseOneStatusNEB,
        }
    }

    // Parse the actual data
    data := buf.ReadBytes(int(length))
    parsed := deserialize(data)

    return snail_parser.ParseOneResult[MyType]{
        Status: snail_parser.ParseOneStatusOK,
        Value:  parsed,
    }
}

// Writer: serialize item to buffer
func myWriter(buf *snail_buffer.Buffer, item MyType) error {
    data := serialize(item)
    buf.WriteInt32BE(int32(len(data)))
    buf.WriteBytes(data)
    return nil
}
```

## Configuration Options

### Batcher Options

```go
snail_tcp_reqrep.BatcherOpts{
    WindowSize: 25 * time.Millisecond,  // Max time before flush
    BatchSize:  100,                     // Max items before flush
    QueueSize:  200,                     // Back-pressure buffer
}
```

!!! tip "Choosing batch size"
    - Larger batches = higher throughput, higher latency
    - Smaller batches = lower latency, lower throughput
    - `QueueSize` must be a multiple of `BatchSize`

### TCP Options

```go
snail_tcp.SnailServerOpts{
    Port:           0,      // 0 = auto-assign
    ReadBufSize:    65536,  // TCP read buffer
    WriteBufSize:   65536,  // TCP write buffer
    TcpNoDelay:     false,  // true = disable Nagle
    TcpRcvBufSize:  0,      // OS TCP receive buffer
    TcpSndBufSize:  0,      // OS TCP send buffer
}
```

## Next Steps

- [Train Station Concept](design/train-station.md) - Understand the batching design
- [Lessons Learned](journey/lessons-learned.md) - Deep dive into Go concurrency
- [Performance Tuning](performance/tuning.md) - Optimize for your use case
