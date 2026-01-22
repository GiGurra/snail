# snail_tcp_reqrep

High-level request-response API for TCP communication.

## Server

### NewServer

Creates a new request-response server.

```go
func NewServer[Req, Resp any](
    handlerFactory func() ServerConnHandler[Req, Resp],
    tcpOpts *snail_tcp.SnailServerOpts,
    reqParser snail_parser.ParseFunc[Req],
    respWriter snail_parser.WriteFunc[Resp],
    opts *SnailServerOpts[Req, Resp],
) (*SnailServer[Req, Resp], error)
```

**Parameters**:

| Name | Type | Description |
|------|------|-------------|
| `handlerFactory` | `func() ServerConnHandler[Req, Resp]` | Called for each new connection, returns the handler |
| `tcpOpts` | `*snail_tcp.SnailServerOpts` | TCP configuration |
| `reqParser` | `ParseFunc[Req]` | Parses requests from bytes |
| `respWriter` | `WriteFunc[Resp]` | Writes responses to bytes |
| `opts` | `*SnailServerOpts[Req, Resp]` | Server options including batcher config |

**Example**:

```go
server, err := snail_tcp_reqrep.NewServer(
    func() snail_tcp_reqrep.ServerConnHandler[Request, Response] {
        // Per-connection state can be initialized here
        return func(req Request, reply func(Response) error) error {
            if reply == nil {
                // Connection closed
                return nil
            }
            return reply(Response{Data: process(req)})
        }
    },
    &snail_tcp.SnailServerOpts{Port: 8080},
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
```

### ServerConnHandler

The function type for handling requests:

```go
type ServerConnHandler[Req, Resp any] func(req Req, reply func(Resp) error) error
```

- `req`: The parsed request
- `reply`: Function to send a response (nil when connection closes)
- Returns: Error if processing failed

### SnailServer Methods

```go
// Port returns the port the server is listening on
func (s *SnailServer[Req, Resp]) Port() int

// Close shuts down the server
func (s *SnailServer[Req, Resp]) Close()
```

### SnailServerOpts

```go
type SnailServerOpts[Req, Resp any] struct {
    Batcher      BatcherOpts
    PerConnCodec *PerConnCodec[Req, Resp]  // Optional per-connection codecs
}

type BatcherOpts struct {
    WindowSize time.Duration  // Auto-flush timeout
    BatchSize  int            // Max items per batch
    QueueSize  int            // Back-pressure buffer (must be multiple of BatchSize)
}
```

## Client

### NewClient

Creates a new request-response client.

```go
func NewClient[Req, Resp any](
    host string,
    port int,
    tcpOpts *snail_tcp.SnailClientOpts,
    respHandler ClientRespHandler[Resp],
    reqWriter snail_parser.WriteFunc[Req],
    respParser snail_parser.ParseFunc[Resp],
) (*SnailClient[Req, Resp], error)
```

**Parameters**:

| Name | Type | Description |
|------|------|-------------|
| `host` | `string` | Server hostname |
| `port` | `int` | Server port |
| `tcpOpts` | `*snail_tcp.SnailClientOpts` | TCP configuration |
| `respHandler` | `ClientRespHandler[Resp]` | Called for each response |
| `reqWriter` | `WriteFunc[Req]` | Writes requests to bytes |
| `respParser` | `ParseFunc[Resp]` | Parses responses from bytes |

**Example**:

```go
client, err := snail_tcp_reqrep.NewClient(
    "localhost", 8080,
    &snail_tcp.SnailClientOpts{},
    func(resp Response, status snail_tcp_reqrep.ClientStatus) error {
        if status == snail_tcp_reqrep.ClientStatusDisconnected {
            log.Println("Disconnected")
            return nil
        }
        fmt.Printf("Got: %v\n", resp)
        return nil
    },
    reqCodec.Writer,
    respCodec.Parser,
)
```

### ClientRespHandler

```go
type ClientRespHandler[Resp any] func(resp Resp, status ClientStatus) error

type ClientStatus int

const (
    ClientStatusOK           ClientStatus = iota
    ClientStatusDisconnected
)
```

### SnailClient Methods

```go
// Send sends a single request
func (c *SnailClient[Req, Resp]) Send(req Req) error

// SendBatch sends multiple requests at once
func (c *SnailClient[Req, Resp]) SendBatch(batch []Req) error

// Close closes the connection
func (c *SnailClient[Req, Resp]) Close()
```

## TCP Options

### SnailServerOpts

```go
type SnailServerOpts struct {
    Port           int   // 0 = auto-assign
    ReadBufSize    int   // Application read buffer (default: 65536)
    WriteBufSize   int   // Application write buffer (default: 65536)
    TcpNoDelay     bool  // Disable Nagle's algorithm
    TcpRcvBufSize  int   // OS TCP receive buffer
    TcpSndBufSize  int   // OS TCP send buffer
}
```

### SnailClientOpts

```go
type SnailClientOpts struct {
    ReadBufSize    int   // Application read buffer (default: 65536)
    WriteBufSize   int   // Application write buffer (default: 65536)
    TcpNoDelay     bool  // Disable Nagle's algorithm
    TcpRcvBufSize  int   // OS TCP receive buffer
    TcpSndBufSize  int   // OS TCP send buffer
}
```

## Complete Example

```go
package main

import (
    "fmt"
    "time"
    "github.com/GiGurra/snail/pkg/snail_parser"
    "github.com/GiGurra/snail/pkg/snail_tcp"
    "github.com/GiGurra/snail/pkg/snail_tcp_reqrep"
)

type Ping struct{ ID int `json:"id"` }
type Pong struct{ ID int `json:"id"` }

func main() {
    pingCodec := snail_parser.NewJsonLinesCodec[Ping]()
    pongCodec := snail_parser.NewJsonLinesCodec[Pong]()

    // Server
    server, _ := snail_tcp_reqrep.NewServer(
        func() snail_tcp_reqrep.ServerConnHandler[Ping, Pong] {
            return func(req Ping, reply func(Pong) error) error {
                if reply == nil { return nil }
                return reply(Pong{ID: req.ID})
            }
        },
        &snail_tcp.SnailServerOpts{},
        pingCodec.Parser, pongCodec.Writer,
        &snail_tcp_reqrep.SnailServerOpts[Ping, Pong]{
            Batcher: snail_tcp_reqrep.BatcherOpts{
                WindowSize: 10 * time.Millisecond,
                BatchSize:  50,
                QueueSize:  100,
            },
        },
    )
    defer server.Close()

    // Client
    received := 0
    client, _ := snail_tcp_reqrep.NewClient(
        "localhost", server.Port(),
        &snail_tcp.SnailClientOpts{},
        func(resp Pong, status snail_tcp_reqrep.ClientStatus) error {
            received++
            return nil
        },
        pingCodec.Writer, pongCodec.Parser,
    )
    defer client.Close()

    // Send requests
    for i := 0; i < 100; i++ {
        client.Send(Ping{ID: i})
    }

    time.Sleep(100 * time.Millisecond)
    fmt.Printf("Received %d responses\n", received)
}
```
