# snail_parser

Codec system for serialization and deserialization.

## Core Types

### ParseFunc

Function type for parsing items from a byte buffer:

```go
type ParseFunc[T any] func(buf *snail_buffer.Buffer) ParseOneResult[T]
```

### WriteFunc

Function type for writing items to a byte buffer:

```go
type WriteFunc[T any] func(buf *snail_buffer.Buffer, item T) error
```

### ParseOneResult

Result of a parse operation:

```go
type ParseOneResult[T any] struct {
    Status ParseOneStatus
    Value  T
}

type ParseOneStatus int

const (
    ParseOneStatusOK  ParseOneStatus = iota  // Successfully parsed
    ParseOneStatusNEB                         // Not Enough Bytes
)
```

### Codec

Convenience struct combining parser and writer:

```go
type Codec[T any] struct {
    Parser ParseFunc[T]
    Writer WriteFunc[T]
}
```

## Built-in Codecs

### JSON Lines

One JSON object per line, newline-delimited:

```go
func NewJsonLinesCodec[T any]() Codec[T]
```

**Format**: `{"field":"value"}\n{"field":"value2"}\n`

**Example**:

```go
type Message struct {
    ID   int    `json:"id"`
    Text string `json:"text"`
}

codec := snail_parser.NewJsonLinesCodec[Message]()

// Use with server/client
server, _ := snail_tcp_reqrep.NewServer(
    handler,
    tcpOpts,
    codec.Parser,   // Parse incoming JSON
    codec.Writer,   // Write outgoing JSON
    serverOpts,
)
```

**Performance**: ~5M ops/sec (bottlenecked by `encoding/json`)

### Int32

Simple 4-byte big-endian integer:

```go
func NewInt32Codec() Codec[int32]
```

**Format**: 4 bytes, big-endian

**Example**:

```go
codec := snail_parser.NewInt32Codec()

// Useful for simple counters, IDs, etc.
```

**Performance**: 300M+ ops/sec

## Custom Codecs

For maximum performance, implement custom codecs.

### Basic Pattern

```go
// Parser
func myParser(buf *snail_buffer.Buffer) snail_parser.ParseOneResult[MyType] {
    // Check if we have enough bytes
    if buf.Len() < minimumSize {
        return snail_parser.ParseOneResult[MyType]{
            Status: snail_parser.ParseOneStatusNEB,
        }
    }

    // Parse the data
    value := parseFromBuffer(buf)

    return snail_parser.ParseOneResult[MyType]{
        Status: snail_parser.ParseOneStatusOK,
        Value:  value,
    }
}

// Writer
func myWriter(buf *snail_buffer.Buffer, item MyType) error {
    // Write to buffer
    writeToBuffer(buf, item)
    return nil
}
```

### Length-Prefixed Protocol

Common pattern: 4-byte length prefix followed by data.

```go
type Message struct {
    ID      int32
    Payload []byte
}

func parseMessage(buf *snail_buffer.Buffer) snail_parser.ParseOneResult[Message] {
    // Need at least 4 bytes for length
    if buf.Len() < 4 {
        return snail_parser.ParseOneResult[Message]{
            Status: snail_parser.ParseOneStatusNEB,
        }
    }

    // Peek at length (don't consume yet)
    length := int(buf.PeekInt32BE())

    // Check if full message available
    if buf.Len() < 4 + length {
        return snail_parser.ParseOneResult[Message]{
            Status: snail_parser.ParseOneStatusNEB,
        }
    }

    // Now consume
    buf.ReadInt32BE()  // Consume length
    id := buf.ReadInt32BE()
    payload := buf.ReadBytes(length - 4)

    return snail_parser.ParseOneResult[Message]{
        Status: snail_parser.ParseOneStatusOK,
        Value:  Message{ID: id, Payload: payload},
    }
}

func writeMessage(buf *snail_buffer.Buffer, msg Message) error {
    totalLen := 4 + len(msg.Payload)  // ID + payload
    buf.WriteInt32BE(int32(totalLen))
    buf.WriteInt32BE(msg.ID)
    buf.WriteBytes(msg.Payload)
    return nil
}
```

### Fixed-Size Protocol

When all messages are the same size:

```go
const MessageSize = 276  // 4 + 8 + 8 + 256

type FixedMessage struct {
    Field1 int32
    Field2 int64
    Field3 int64
    Data   [256]byte
}

func parseFixed(buf *snail_buffer.Buffer) snail_parser.ParseOneResult[FixedMessage] {
    if buf.Len() < MessageSize {
        return snail_parser.ParseOneResult[FixedMessage]{
            Status: snail_parser.ParseOneStatusNEB,
        }
    }

    msg := FixedMessage{
        Field1: buf.ReadInt32BE(),
        Field2: buf.ReadInt64BE(),
        Field3: buf.ReadInt64BE(),
    }
    copy(msg.Data[:], buf.ReadBytes(256))

    return snail_parser.ParseOneResult[FixedMessage]{
        Status: snail_parser.ParseOneStatusOK,
        Value:  msg,
    }
}

func writeFixed(buf *snail_buffer.Buffer, msg FixedMessage) error {
    buf.WriteInt32BE(msg.Field1)
    buf.WriteInt64BE(msg.Field2)
    buf.WriteInt64BE(msg.Field3)
    buf.WriteBytes(msg.Data[:])
    return nil
}
```

## snail_buffer.Buffer

The buffer type used by parsers and writers.

### Reading Methods

```go
// Integers (big-endian)
func (b *Buffer) ReadInt32BE() int32
func (b *Buffer) ReadInt64BE() int64
func (b *Buffer) ReadUint32BE() uint32
func (b *Buffer) ReadUint64BE() uint64

// Integers (little-endian)
func (b *Buffer) ReadInt32LE() int32
func (b *Buffer) ReadInt64LE() int64

// Bytes
func (b *Buffer) ReadByte() byte
func (b *Buffer) ReadBytes(n int) []byte

// Peeking (don't advance position)
func (b *Buffer) PeekInt32BE() int32
func (b *Buffer) PeekBytes(n int) []byte

// Info
func (b *Buffer) Len() int  // Remaining bytes
```

### Writing Methods

```go
// Integers (big-endian)
func (b *Buffer) WriteInt32BE(v int32)
func (b *Buffer) WriteInt64BE(v int64)
func (b *Buffer) WriteUint32BE(v uint32)
func (b *Buffer) WriteUint64BE(v uint64)

// Integers (little-endian)
func (b *Buffer) WriteInt32LE(v int32)
func (b *Buffer) WriteInt64LE(v int64)

// Bytes
func (b *Buffer) WriteByte(v byte)
func (b *Buffer) WriteBytes(v []byte)
```

## Performance Comparison

| Codec Type | Throughput | Notes |
|------------|------------|-------|
| Custom binary | 30M+ ops/sec | No reflection, no allocation |
| Protocol Buffers | 10-20M ops/sec | Some reflection |
| JSON (sonic) | 10-15M ops/sec | Fast JSON library |
| JSON (std) | 5M ops/sec | Heavy reflection |

## Best Practices

### 1. Minimize Allocations

```go
// Bad: Allocates on every parse
func parse(buf *Buffer) Result {
    data := make([]byte, length)  // Allocation!
    copy(data, buf.ReadBytes(length))
    return Result{Data: data}
}

// Good: Use existing buffer when possible
func parse(buf *Buffer) Result {
    // ReadBytes returns a slice into the buffer
    // Only copy if you need to keep it
    data := buf.ReadBytes(length)
    return Result{Data: data}
}
```

### 2. Always Check Length First

```go
func parse(buf *Buffer) ParseOneResult[T] {
    // ALWAYS check before reading
    if buf.Len() < requiredSize {
        return ParseOneResult[T]{Status: ParseOneStatusNEB}
    }
    // Now safe to read
}
```

### 3. Use Peek for Variable-Length Protocols

```go
func parse(buf *Buffer) ParseOneResult[T] {
    if buf.Len() < 4 {
        return ParseOneResult[T]{Status: ParseOneStatusNEB}
    }

    // Peek at length without consuming
    length := buf.PeekInt32BE()

    if buf.Len() < 4 + int(length) {
        return ParseOneResult[T]{Status: ParseOneStatusNEB}
    }

    // Now consume everything
    buf.ReadInt32BE()  // Consume the length we peeked
    // ... read rest
}
```

### 4. Consider Endianness

Network protocols typically use big-endian (network byte order):

```go
// Network protocol - use big-endian
buf.WriteInt32BE(value)
value := buf.ReadInt32BE()

// Native format - use little-endian on most modern CPUs
buf.WriteInt32LE(value)
value := buf.ReadInt32LE()
```
