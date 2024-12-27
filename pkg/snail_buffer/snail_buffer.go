package snail_buffer

import (
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
		b.buf = append(b.buf, byte((val>>8)&0xFF))
		b.buf = append(b.buf, byte(val&0xFF))
	} else {
		b.buf = append(b.buf, byte(val&0xFF))
		b.buf = append(b.buf, byte((val>>8)&0xFF))
	}
}

func (b *Buffer) ReadInt16() (int16, error) {
	if !b.CanRead(2) {
		return 0, fmt.Errorf("not enough data to read int16")
	}

	var val int16
	if b.endian == BigEndian {
		val = int16(b.buf[b.readPos])<<8 | int16(b.buf[b.readPos+1])
	} else {
		val = int16(b.buf[b.readPos]) | int16(b.buf[b.readPos+1])<<8
	}
	b.readPos += 2
	return val, nil
}

func (b *Buffer) ReadSInt8() (int8, error) {
	if !b.CanRead(1) {
		return 0, fmt.Errorf("not enough data to read int8")
	}
	val := int8(b.buf[b.readPos])
	b.readPos++
	return val, nil
}

func (b *Buffer) ReadUInt8() (uint8, error) {
	if !b.CanRead(1) {
		return 0, fmt.Errorf("not enough data to read int8")
	}
	val := b.buf[b.readPos]
	b.readPos++
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

func (b *Buffer) WriteInt32(val int32) {
	if b.endian == BigEndian {
		b.buf = append(b.buf, byte((val>>24)&0xFF))
		b.buf = append(b.buf, byte((val>>16)&0xFF))
		b.buf = append(b.buf, byte((val>>8)&0xFF))
		b.buf = append(b.buf, byte(val&0xFF))
	} else {
		b.buf = append(b.buf, byte(val&0xFF))
		b.buf = append(b.buf, byte((val>>8)&0xFF))
		b.buf = append(b.buf, byte((val>>16)&0xFF))
		b.buf = append(b.buf, byte((val>>24)&0xFF))
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
		val = int32(b.buf[b.readPos])<<24 | int32(b.buf[b.readPos+1])<<16 | int32(b.buf[b.readPos+2])<<8 | int32(b.buf[b.readPos+3])
	} else {
		val = int32(b.buf[b.readPos]) | int32(b.buf[b.readPos+1])<<8 | int32(b.buf[b.readPos+2])<<16 | int32(b.buf[b.readPos+3])<<24
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

func (b *Buffer) NumBytesReadable() int {
	return len(b.buf) - b.readPos
}

func (b *Buffer) DiscardReadBytes() {
	b.buf = snail_slice.DiscardFirstN(b.buf, b.readPos)
	b.readPos = 0
}

func (b *Buffer) WriteUInt8(u uint8) {
	b.buf = append(b.buf, u)
}

func (b *Buffer) String() string {
	return fmt.Sprintf("Buffer{readPos: %d, size: %d}", b.readPos, len(b.buf))
}

func (b *Buffer) Copy() []byte {
	return append([]byte{}, b.buf...)
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
	b.readPos = 0
	b.buf = b.buf[:0]
}
