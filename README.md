Here's a GitHub README.md for your project:

# Snail 🐌

A high-performance networking library for Go focusing on TCP communication with request-response patterns.

## Features

- High-performance TCP client/server implementation
- Request-response pattern support
- Efficient batching mechanism
- Support for different encoding formats (JSON, custom binary protocols)
- Buffer management with memory pooling options
- Configurable optimization for latency or throughput

## Installation

```bash
go get github.com/GiGurra/snail
```

## Quick Start

### Simple JSON Server

```go
reqCodec := snail_parser.NewJsonLinesCodec[requestStruct]()
respCodec := snail_parser.NewJsonLinesCodec[responseStruct]()

server, err := NewServer[requestStruct, responseStruct](
    func() ServerConnHandler[requestStruct, responseStruct] {
        return func(req requestStruct, repFunc func(resp responseStruct) error) error {
            return repFunc(responseStruct{Msg: "Hello from server"})
        }
    },
    nil,
    reqCodec.Parser,
    respCodec.Writer,
    nil,
)
```

### Simple JSON Client

```go
client, err := NewClient[requestStruct, responseStruct](
    "localhost",
    server.Port(),
    nil,
    func(resp responseStruct, status ClientStatus) error {
        fmt.Printf("Received response: %v\n", resp)
        return nil
    },
    reqCodec.Writer,
    respCodec.Parser,
)

// Send a request
client.Send(requestStruct{Msg: "Hello"})
```

## Performance Features

- Efficient batching system for high-throughput scenarios
- Configurable TCP options (NoDelay, buffer sizes)
- Memory pooling for reduced GC pressure
- Support for custom allocators
- Optimized for both latency and throughput use cases

## Main Components

- `snail_tcp`: Low-level TCP networking
- `snail_tcp_reqrep`: Request-response pattern implementation
- `snail_batcher`: Efficient batching system
- `snail_buffer`: Buffer management
- `snail_parser`: Encoding/decoding framework

## Configuration

### TCP Options

```go
opts := &snail_tcp.SnailServerOpts{
    Optimization: OptimizeForThroughput,  // or OptimizeForLatency
    ReadBufSize: 64 * 1024,
    TcpReadWindowSize: 1024 * 1024,
}
```

### Batching Options

```go
batchOpts := BatcherOpts{
    WindowSize: 25 * time.Millisecond,
    BatchSize: 1000,
    QueueSize: 2000,
}
```

## Performance Tips

1. Use batching for high-throughput scenarios
2. Configure appropriate buffer sizes
3. Consider using custom memory allocators for reduced GC pressure
4. Choose the right optimization mode (latency vs throughput)

## License

[Add your license here]

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Readme  Authors
- Anthropic Claude 3.5

## Code Authors

- GiGurra
- Github Copilot
- Anthropic Claude 3.5