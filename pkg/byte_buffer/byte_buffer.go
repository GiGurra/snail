package byte_buffer

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/util_slice"
)

type Endian int

const (
	BigEndian    Endian = iota
	LittleEndian Endian = iota
)

type ByteBuffer struct {
	endian      Endian
	buf         []byte
	readPos     int
	readPosMark int
}

func NewByteBuffer(endian Endian, size int) *ByteBuffer {
	return &ByteBuffer{
		endian: endian,
		buf:    make([]byte, 0, size),
	}
}

func (b *ByteBuffer) WriteBytes(val []byte) {
	b.buf = append(b.buf, val...)
}

func (b *ByteBuffer) WriteString(val string) {
	b.buf = append(b.buf, []byte(val)...)
}

func (b *ByteBuffer) WriteInt8(val int8) {
	b.buf = append(b.buf, byte(val))
}

func (b *ByteBuffer) WriteInt16(val int16) {
	if b.endian == BigEndian {
		b.buf = append(b.buf, byte((val>>8)&0xFF))
		b.buf = append(b.buf, byte(val&0xFF))
	} else {
		b.buf = append(b.buf, byte(val&0xFF))
		b.buf = append(b.buf, byte((val>>8)&0xFF))
	}
}

func (b *ByteBuffer) ReadInt16() (int16, error) {
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

func (b *ByteBuffer) ReadSInt8() (int8, error) {
	if !b.CanRead(1) {
		return 0, fmt.Errorf("not enough data to read int8")
	}
	val := int8(b.buf[b.readPos])
	b.readPos++
	return val, nil
}

func (b *ByteBuffer) ReadUInt8() (uint8, error) {
	if !b.CanRead(1) {
		return 0, fmt.Errorf("not enough data to read int8")
	}
	val := b.buf[b.readPos]
	b.readPos++
	return val, nil
}

func (b *ByteBuffer) ReadString(n int) (string, error) {
	if !b.CanRead(n) {
		return "", fmt.Errorf("not enough data to read string")
	}
	val := string(b.buf[b.readPos : b.readPos+n])
	b.readPos += n
	return val, nil
}

func (b *ByteBuffer) WriteInt32(val int32) {
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

func (b *ByteBuffer) CanRead(n int) bool {
	return b.readPos+n <= len(b.buf)
}

func (b *ByteBuffer) Readable() int {
	return len(b.buf) - b.readPos
}

func (b *ByteBuffer) ReadInt32() (int32, error) {
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

func (b *ByteBuffer) NumBytesReadable() int {
	return len(b.buf) - b.readPos
}

func (b *ByteBuffer) DiscardReadBytes() {
	b.buf = util_slice.DiscardFirstN(b.buf, b.readPos)
	b.readPos = 0
}

func (b *ByteBuffer) WriteUInt8(u uint8) {
	b.buf = append(b.buf, u)
}

func (c *ByteBuffer) String() string {
	return fmt.Sprintf("ByteBuffer{readPos: %d, size: %d}", c.readPos, len(c.buf))
}

func (b *ByteBuffer) Copy() []byte {
	return append([]byte{}, b.buf...)
}

func (b *ByteBuffer) Underlying() []byte {
	return b.buf
}

func (b *ByteBuffer) MarkReadPos() {
	b.readPosMark = b.readPos
}

func (b *ByteBuffer) ResetReadPosToMark() {
	b.readPos = b.readPosMark
}

func (b *ByteBuffer) Reset() {
	b.readPos = 0
	b.buf = b.buf[:0]
}
