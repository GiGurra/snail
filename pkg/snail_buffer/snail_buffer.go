package snail_buffer

import (
	"encoding/binary"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_slice"
	"io"
)

type Endian int

const (
	BigEndian    Endian = iota
	LittleEndian Endian = iota
)

type Buffer struct {
	endian      Endian
	buf         []byte
	readPos     int
	readPosMark int
}

func (b *Buffer) Read(p []byte) (n int, err error) {

	numToRead := b.NumBytesReadable()

	// if there's no data to read, return EOF
	if numToRead == 0 {
		return 0, io.EOF
	}

	if len(p) < numToRead {
		if len(p) == 0 {
			return 0, fmt.Errorf("can't read into a zero-length buffer")
		}
		numToRead = len(p)
	}

	// if p doesn't fit in the buffer, copy what we can and return
	copy(p, b.buf[b.readPos:b.readPos+numToRead])
	b.readPos += numToRead
	return numToRead, nil
}

// Write implements io.Writer
func (b *Buffer) Write(p []byte) (n int, err error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// prove Buffer implements io.Writer
var _ io.Writer = &Buffer{}

// prove Buffer implements io.Reader
var _ io.Reader = &Buffer{}

func New(endian Endian, size int) *Buffer {
	return &Buffer{
		endian: endian,
		buf:    make([]byte, 0, size),
	}
}

func (b *Buffer) WriteBytes(val []byte) {
	b.buf = append(b.buf, val...)
}

func (b *Buffer) WriteString(val string) {
	b.buf = append(b.buf, []byte(val)...)
}

func (b *Buffer) WriteInt8(val int8) {
	b.buf = append(b.buf, byte(val))
}

func (b *Buffer) WriteInt16(val int16) {
	if b.endian == BigEndian {
		b.buf = binary.BigEndian.AppendUint16(b.buf, uint16(val))
	} else {
		b.buf = binary.LittleEndian.AppendUint16(b.buf, uint16(val))
	}
}

func (b *Buffer) ReadInt16() (int16, error) {
	if !b.CanRead(2) {
		return 0, fmt.Errorf("not enough data to read int16")
	}

	var val int16
	if b.endian == BigEndian {
		val = int16(binary.BigEndian.Uint16(b.buf[b.readPos:]))
	} else {
		val = int16(binary.LittleEndian.Uint16(b.buf[b.readPos:]))
	}
	b.readPos += 2
	return val, nil
}

func (b *Buffer) ReadString(n int) (string, error) {
	if !b.CanRead(n) {
		return "", fmt.Errorf("not enough data to read string")
	}
	val := string(b.buf[b.readPos : b.readPos+n])
	b.readPos += n
	return val, nil
}

func (b *Buffer) WriteInt32(value int32) {
	if b.endian == BigEndian {
		b.buf = binary.BigEndian.AppendUint32(b.buf, uint32(value))
	} else {
		b.buf = binary.LittleEndian.AppendUint32(b.buf, uint32(value))
	}
}

func (b *Buffer) CanRead(n int) bool {
	return b.Readable() >= n
}

func (b *Buffer) Readable() int {
	return len(b.buf) - b.readPos
}

func (b *Buffer) ReadInt32() (int32, error) {
	if !b.CanRead(4) {
		return 0, fmt.Errorf("not enough data to read int32")
	}

	var val int32
	if b.endian == BigEndian {
		val = int32(binary.BigEndian.Uint32(b.buf[b.readPos:]))
	} else {
		val = int32(binary.LittleEndian.Uint32(b.buf[b.readPos:]))
	}
	b.readPos += 4
	return val, nil
}

func (b *Buffer) ReadBytes(n int) ([]byte, error) {
	if !b.CanRead(n) {
		return nil, fmt.Errorf("not enough data to read bytes")
	}

	cpy := make([]byte, n)
	copy(cpy, b.buf[b.readPos:b.readPos+n])
	b.readPos += n

	return cpy, nil
}

func (b *Buffer) ReadBytesInto(trg []byte, n int) error {
	if !b.CanRead(n) {
		return fmt.Errorf("not enough data to read bytes")
	}

	if len(trg) < n {
		return fmt.Errorf("target buffer too small")
	}

	copy(trg, b.buf[b.readPos:b.readPos+n])
	b.readPos += n

	return nil
}

func (b *Buffer) NumBytesReadable() int {
	return len(b.buf) - b.readPos
}

func (b *Buffer) DiscardReadBytes() {
	readPosBefore := b.readPos
	b.buf = snail_slice.DiscardFirstN(b.buf, readPosBefore)
	b.readPos = 0
	b.readPosMark -= readPosBefore
}

func (b *Buffer) WriteByte(u byte) error {
	b.buf = append(b.buf, u)
	return nil
}

func (b *Buffer) WriteByteNoE(u byte) {
	b.buf = append(b.buf, u)
}

func (b *Buffer) String() string {
	return fmt.Sprintf("Buffer{readPos: %d, size: %d}", b.readPos, len(b.buf))
}

func (b *Buffer) ReadAll() []byte {
	out := make([]byte, len(b.buf)-b.readPos)
	copy(out, b.buf[b.readPos:])
	b.readPos = len(b.buf)
	return out
}

func (b *Buffer) Underlying() []byte {
	return b.buf
}

func (b *Buffer) MarkReadPos() {
	b.readPosMark = b.readPos
}

func (b *Buffer) ResetReadPosToMark() {
	b.readPos = b.readPosMark
}

func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
	b.readPos = 0
	b.readPosMark = 0
}

func (b *Buffer) ReadPos() int {
	return b.readPos
}

func (b *Buffer) SetReadPos(pos int) {
	if pos < 0 || pos > len(b.buf) {
		panic(fmt.Sprintf("invalid read pos: %d", pos))
	}
	b.readPos = pos
}

func (b *Buffer) AdvanceReadPos(delta int) {
	b.readPos += delta
}

func (b *Buffer) EnsureSpareCapacity(n int) {
	if cap(b.buf)-len(b.buf) < n {
		newBuf := make([]byte, len(b.buf), len(b.buf)+n)
		copy(newBuf, b.buf)
		b.buf = newBuf
	}
}

func (b *Buffer) AddWritten(n int) {
	b.buf = b.buf[:len(b.buf)+n]
}

func (b *Buffer) UnderlyingWriteable() []byte {
	return b.buf[len(b.buf):cap(b.buf)]
}

func (b *Buffer) UnderlyingReadable() []byte {
	return b.buf[b.readPos:]
}

func (b *Buffer) UnderlyingReadableView() *Buffer {
	return &Buffer{
		endian:      b.endian,
		buf:         b.UnderlyingReadable(),
		readPos:     0,
		readPosMark: 0,
	}
}

func (b *Buffer) WriteInt64(value int64) {
	if b.endian == BigEndian {
		b.buf = binary.BigEndian.AppendUint64(b.buf, uint64(value))
	} else {
		b.buf = binary.LittleEndian.AppendUint64(b.buf, uint64(value))
	}
}

func (b *Buffer) ReadInt64() (int64, error) {
	if !b.CanRead(8) {
		return 0, fmt.Errorf("not enough data to read int64")
	}

	var val int64
	if b.endian == BigEndian {
		val = int64(binary.BigEndian.Uint64(b.buf[b.readPos:]))
	} else {
		val = int64(binary.LittleEndian.Uint64(b.buf[b.readPos:]))
	}

	b.readPos += 8

	return val, nil
}
