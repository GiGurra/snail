# Snail

**High-throughput TCP networking library for Go with intelligent batching**

[![Go Report Card](https://goreportcard.com/badge/github.com/GiGurra/snail)](https://goreportcard.com/report/github.com/GiGurra/snail)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## What is Snail?

Snail is a Go library that achieves **300+ million request-response operations per second** over TCP by transparently batching requests. You write normal function calls; snail handles the batching under the hood.

## The Problem

TCP is optimized for streaming data, not individual request-response patterns. A naive RPC implementation over TCP typically achieves only **10-20k requests/second** per goroutine. This is because each small write incurs significant overhead.

## The Solution

Snail uses the "train station" concept: requests wait at a station until either:

1. The batch reaches a configured size, OR
2. A time window expires

Then the entire batch is sent as one TCP write. The same happens for responses on the server side.

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

## Performance

Measured on 7950X3D, 64GB RAM, Ubuntu 24.04, loopback TCP:

| Payload Size | Throughput | Bandwidth |
|-------------|------------|-----------|
| 4 bytes | 300-350M req/s | ~20 Gbit/s |
| 276 bytes | 30M req/s | ~130 Gbit/s |
| HTTP/1.1 | 45M req/s | ~65 Gbit/s |
| JSON | 5M req/s | (JSON marshalling bottleneck) |

## Quick Example

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

type Request struct{ Msg string `json:"msg"` }
type Response struct{ Msg string `json:"msg"` }

func main() {
    codec := snail_parser.NewJsonLinesCodec[Request]()
    respCodec := snail_parser.NewJsonLinesCodec[Response]()

    // Server
    server, _ := snail_tcp_reqrep.NewServer(
        func() snail_tcp_reqrep.ServerConnHandler[Request, Response] {
            return func(req Request, reply func(Response) error) error {
                return reply(Response{Msg: "Got: " + req.Msg})
            }
        },
        &snail_tcp.SnailServerOpts{},
        codec.Parser, respCodec.Writer,
        &snail_tcp_reqrep.SnailServerOpts[Request, Response]{
            Batcher: snail_tcp_reqrep.BatcherOpts{
                WindowSize: 25 * time.Millisecond,
                BatchSize:  100,
                QueueSize:  200,
            },
        },
    )
    defer server.Close()

    // Client with batching
    client, _ := snail_tcp_reqrep.NewClient(
        "localhost", server.Port(),
        &snail_tcp.SnailClientOpts{},
        func(resp Response, _ snail_tcp_reqrep.ClientStatus) error {
            fmt.Println(resp.Msg)
            return nil
        },
        codec.Writer, respCodec.Parser,
    )
    defer client.Close()

    batcher := snail_batcher.NewSnailBatcher(100, 200, 25*time.Millisecond,
        func(batch []Request) error { return client.SendBatch(batch) },
    )
    defer batcher.Close()

    for i := 0; i < 1000; i++ {
        batcher.Add(Request{Msg: fmt.Sprintf("Hello %d", i)})
    }
    time.Sleep(time.Second)
}
```

## Installation

```bash
go get github.com/GiGurra/snail
```

## Learn More

- [Getting Started](getting-started.md) - Full setup guide
- [Train Station Concept](design/train-station.md) - The batching design philosophy
- [Evolution](journey/evolution.md) - How this project came to be
- [Lessons Learned](journey/lessons-learned.md) - Deep technical insights from building snail
- [Benchmarks](performance/benchmarks.md) - Detailed performance numbers
