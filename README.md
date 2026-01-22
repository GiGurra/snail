# Snail

A high-throughput TCP networking library for Go that achieves 300M+ request-response operations per second through intelligent batching.

[![Go Report Card](https://goreportcard.com/badge/github.com/GiGurra/snail)](https://goreportcard.com/report/github.com/GiGurra/snail)
[![Docs](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://gigurra.github.io/snail/)

## Why Snail?

TCP is optimized for streaming data, not individual request-response patterns. Naive RPC implementations over TCP typically achieve only 10-20k requests/second. Snail solves this by batching requests transparently - you write normal function calls, snail handles the batching under the hood.

**Performance** (7950X3D, loopback TCP):
- 300-350M req/s with 4-byte payloads (~20 Gbit/s)
- 30M req/s with 276-byte payloads (~130 Gbit/s)
- 45M req/s HTTP/1.1 (~65 Gbit/s)

## Installation

```bash
go get github.com/GiGurra/snail
```

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/GiGurra/snail/pkg/snail_batcher"
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

    // Create server
    server, _ := snail_tcp_reqrep.NewServer(
        func() snail_tcp_reqrep.ServerConnHandler[Request, Response] {
            return func(req Request, reply func(Response) error) error {
                return reply(Response{Msg: "Got: " + req.Msg})
            }
        },
        &snail_tcp.SnailServerOpts{},
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
    defer server.Close()

    // Create client
    client, _ := snail_tcp_reqrep.NewClient(
        "localhost", server.Port(),
        &snail_tcp.SnailClientOpts{},
        func(resp Response, status snail_tcp_reqrep.ClientStatus) error {
            fmt.Printf("Response: %s\n", resp.Msg)
            return nil
        },
        reqCodec.Writer,
        respCodec.Parser,
    )
    defer client.Close()

    // Send with client-side batching
    batcher := snail_batcher.NewSnailBatcher(
        100, 200, 25*time.Millisecond,
        func(batch []Request) error { return client.SendBatch(batch) },
    )
    defer batcher.Close()

    for i := 0; i < 1000; i++ {
        batcher.Add(Request{Msg: fmt.Sprintf("Hello %d", i)})
    }

    time.Sleep(time.Second)
}
```

## How It Works

Inspired by Transport Tycoon: requests wait at a "train station" until either:
1. The batch reaches a configured size, OR
2. A time window expires

Then the entire batch is sent as one TCP write. Same happens for responses on the server side.

```
Client                          Server
  |                               |
  |--[Req1]--→ [Batcher]          |
  |--[Req2]--→ [  ...  ]          |
  |--[Req3]--→ [  ...  ]          |
  |           [Batch!] ========→  | → Process each
  |                               | → Responses batched
  |  ←======== [Resp batch]       |
  |                               |
```

## Package Overview

| Package | Purpose |
|---------|---------|
| `snail_tcp_reqrep` | High-level request-response client/server |
| `snail_tcp` | Low-level TCP client/server |
| `snail_batcher` | Generic batching engine (reusable for non-TCP) |
| `snail_parser` | Codecs (JSON lines, binary) |
| `snail_buffer` | Efficient buffer with endianness support |

## Configuration

### Batcher Options

```go
snail_tcp_reqrep.BatcherOpts{
    WindowSize: 25 * time.Millisecond,  // Max wait time before flush
    BatchSize:  100,                     // Max items before flush
    QueueSize:  200,                     // Back-pressure buffer size
}
```

### TCP Options

```go
snail_tcp.SnailServerOpts{
    Port:           0,      // 0 = auto-assign
    ReadBufSize:    65536,  // TCP read buffer
    WriteBufSize:   65536,  // TCP write buffer
    TcpNoDelay:     false,  // Disable Nagle's algorithm
}
```

## Custom Codecs

Implement your own serialization:

```go
// Parser: reads from buffer, returns parsed item or "need more bytes"
func myParser(buf *snail_buffer.Buffer) snail_parser.ParseOneResult[MyType] {
    if buf.Len() < 4 {
        return snail_parser.ParseOneResult[MyType]{Status: snail_parser.ParseOneStatusNEB}
    }
    // Parse and return
    return snail_parser.ParseOneResult[MyType]{Status: snail_parser.ParseOneStatusOK, Value: parsed}
}

// Writer: serializes item to buffer
func myWriter(buf *snail_buffer.Buffer, item MyType) error {
    // Write to buf
    return nil
}
```

## Standalone Batcher

The batcher works independently of TCP - useful for log writing, disk I/O batching, etc:

```go
batcher := snail_batcher.NewSnailBatcher[LogEntry](
    1000,                    // batch size
    5000,                    // queue size
    100 * time.Millisecond,  // flush window
    func(batch []LogEntry) error {
        // Write batch to file/database/etc
        return writeLogs(batch)
    },
)
```

## Performance Notes

- The batcher uses hybrid locking (spin + mutex) - 50x faster than channels for high-contention fan-in
- Pre-allocated buffers minimize GC pressure
- Triple-buffering allows concurrent write/send operations
- Platform differences: mutex performance varies significantly between macOS/Linux

See the old README (README.md.old) for detailed benchmarks and lessons learned.

## License

MIT

## Authors

- GiGurra
- Anthropic Claude
- GitHub Copilot
