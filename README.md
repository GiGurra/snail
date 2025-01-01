Here's a what Anthropic claude 3.5 sonnet thinks about this project:

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


### Shared
    
```go

type requestStruct struct {
    Msg string
}

type responseStruct struct {
    Msg string
}

reqCodec := snail_parser.NewJsonLinesCodec[requestStruct]()
respCodec := snail_parser.NewJsonLinesCodec[responseStruct]()
```

### Simple JSON Server

```go

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
- Efficient Fan-In/Fan-Out pattern support
- Configurable TCP options (buffer sizes)
- Configurable for both latency and throughput use cases

## Preliminary Benchmarks

Measured on 
- 7950x3d CPU 
- 64GB of 6000MT/s dual channel RAM
- loopback tcp connections
- WSL2 Ubuntu 22.04 on Windows 11

Numbers:
- Reference max bandwidth achieved on loopback: 240-250 GBit/s
  - achieved sending larger (10kB+) byte chunks as messages without
  - `hperf3` achieved only about 135 GBit/s, but I'm probably using it wrong
- Request rate with small, individually processed, messages (4 Bytes/message): 300-350 million requests/s
- Request rate with "regular", individually processed, messages (250 Bytes/message): 20-30 million requests/s 
  - (100-150 Gbit/s)
- Request rate with http1.1 request/responses using `h2load` as load generator: 20-25 million requests/s 
- Request rate with json payload: 5 million requests/s
  - Almost all time spent in go std lib json marshalling/unmarshalling

### Experimental features used in testing and benchmarking

- Memory pooling for reduced GC pressure
- Support for custom allocators

## Configuration

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
3. Choose the right optimization mode (latency vs throughput)
4. Consider using custom memory allocators for reduced GC pressure if needed

## License

[Add your license here]

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Readme  Authors
- Anthropic Claude 3.5

## Code Authors

- GiGurra
- Github Copilot
- Anthropic Claude 3.5 sonnet