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
- Reference max bandwidth achieved on loopback: 240-250 GBit/s (using `snail`)
  - achieved sending larger (10kB+) byte chunks as messages without
  - `hperf3` achieved only about 135 GBit/s, but I'm probably using it wrong
- Request rate with 4 byte requests/responses: 300-350 million request-responses/s
  - Each request and response is just a single int32
  - This equates to about 20 GBit/s throughput
    - half of which is the data sent to the server
    - half of which is the data sent back to the client
- Request rate with 250 byte requests/responses: 20-30 million request-responses/s 
  - Each request and response is a custom struct with
    - 3 encoded/decoded integer fields
    - 1 long byte array at the end
    - custom encoder/parser
  - This equates to about 100-150 Gbit/s throughput
    - half of which is the data sent to the server
    - half of which is the data sent back to the client
- Request rate with http1.1 using `h2load` as load generator: 20-25 million request-responses/s 
  - We parse very minimal parts of the incoming http request for testing purposes
  - http 1.1 pipelining is enabled
    - a comparable number should be achievable with http2 multiplexing
  - We should investigate `h2load` to se if it is the bottleneck
  - This equates to about 40 Gbit/s throughput (according to `h2load`)
    - half of which is the data sent to the server
    - half of which is the data sent back to the client
- Request rate with json payload: 5 million request-responses/s
  - Almost all time spent in go std lib json marshalling/unmarshalling

What does `request-responses` mean?
- A request is exactly one go function call with the request type as argument
- A response is exactly one go function callback with the response type as argument

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

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Readme  Authors
- Anthropic Claude 3.5

## Code Authors

- GiGurra
- Github Copilot
- Anthropic Claude 3.5 sonnet