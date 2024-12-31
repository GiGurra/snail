package snail_buffer

import (
	"errors"
	"io"
	"testing"
)

func TestNewByteBuffer(t *testing.T) {
	bb := New(BigEndian, 10)
	if bb.endian != BigEndian {
		t.Errorf("Expected %v, got %v", BigEndian, bb.endian)
	}
	if cap(bb.buf) != 10 {
		t.Errorf("Expected %v, got %v", 10, cap(bb.buf))
	}
	if bb.readPos != 0 {
		t.Errorf("Expected %v, got %v", 0, bb.readPos)
	}
	if bb.readPosMark != 0 {
		t.Errorf("Expected %v, got %v", 0, bb.readPosMark)
	}
}

func TestUintInt64Conversion(t *testing.T) {
	// 0xFFFFFFFFFFFFFFFF converted to int64 and back to uint64 should be the same
	val := uint64(0xFFFFFFFFFFFFFFFF)
	if int64(val) != -1 {
		t.Errorf("Expected %v, got %v", -1, int64(val))
	}
	if uint64(int64(val)) != val {
		t.Errorf("Expected %v, got %v", val, uint64(int64(val)))
	}
}

func TestByteBuffer_WriteInt64BigEndian(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt64(0x1234567878563412)
	expected := []byte{0x12, 0x34, 0x56, 0x78, 0x78, 0x56, 0x34, 0x12}
	if len(bb.buf) != 8 {
		t.Errorf("Expected %v, got %v", 4, len(bb.buf))
	}
	for i, b := range bb.buf {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
	val, err := bb.ReadInt64()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != 0x1234567878563412 {
		t.Errorf("Expected %v, got %v", 0x1234567878563412, val)
	}
}

func TestByteBuffer_WriteInt64LittleEndian(t *testing.T) {
	bb := New(LittleEndian, 10)
	bb.WriteInt64(0x1234567878563412)
	expected := []byte{0x12, 0x34, 0x56, 0x78, 0x78, 0x56, 0x34, 0x12}
	if len(bb.buf) != 8 {
		t.Errorf("Expected %v, got %v", 4, len(bb.buf))
	}
	for i, b := range bb.buf {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
	val, err := bb.ReadInt64()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != 0x1234567878563412 {
		t.Errorf("Expected %v, got %v", 0x1234567878563412, val)
	}
}

func TestByteBuffer_WriteInt32BigEndian(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt32(0x12345678)
	expected := []byte{0x12, 0x34, 0x56, 0x78}
	if len(bb.buf) != 4 {
		t.Errorf("Expected %v, got %v", 4, len(bb.buf))
	}
	for i, b := range bb.buf {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
	val, err := bb.ReadInt32()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != 0x12345678 {
		t.Errorf("Expected %v, got %v", 0x12345678, val)
	}
}

func TestByteBuffer_WriteInt32LittleEndian(t *testing.T) {
	bb := New(LittleEndian, 10)
	bb.WriteInt32(0x12345678)
	expected := []byte{0x78, 0x56, 0x34, 0x12}
	if len(bb.buf) != 4 {
		t.Errorf("Expected %v, got %v", 4, len(bb.buf))
	}
	for i, b := range bb.buf {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
	val, err := bb.ReadInt32()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != 0x12345678 {
		t.Errorf("Expected %v, got %v", 0x12345678, val)
	}
}

func TestByteBuffer_ReadInt16BigEndian(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	expected := []byte{0x12, 0x34}
	if len(bb.buf) != 2 {
		t.Errorf("Expected %v, got %v", 2, len(bb.buf))
	}
	for i, b := range bb.buf {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
	val, err := bb.ReadInt16()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != 0x1234 {
		t.Errorf("Expected %v, got %v", 0x1234, val)
	}
}

func TestByteBuffer_ReadInt16LittleEndian(t *testing.T) {
	bb := New(LittleEndian, 10)
	bb.WriteInt16(0x1234)
	expected := []byte{0x34, 0x12}
	if len(bb.buf) != 2 {
		t.Errorf("Expected %v, got %v", 2, len(bb.buf))
	}
	for i, b := range bb.buf {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
	val, err := bb.ReadInt16()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != 0x1234 {
		t.Errorf("Expected %v, got %v", 0x1234, val)
	}
}

func TestByteBuffer_CanRead(t *testing.T) {
	// write Two consecutive int16 and read them
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	bb.WriteInt16(0x5678)
	if !bb.CanRead(4) {
		t.Errorf("Expected true, got false")
	}

	if bb.Readable() != 4 {
		t.Errorf("Expected %v, got %v", 4, bb.Readable())
	}

	val1, err := bb.ReadInt16()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val1 != 0x1234 {
		t.Errorf("Expected %v, got %v", 0x1234, val1)
	}
	if !bb.CanRead(2) {
		t.Errorf("Expected true, got false")
	}
	if bb.Readable() != 2 {
		t.Errorf("Expected %v, got %v", 2, bb.Readable())
	}

	val2, err := bb.ReadInt16()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val2 != 0x5678 {
		t.Errorf("Expected %v, got %v", 0x5678, val2)
	}
	if bb.CanRead(1) {
		t.Errorf("Expected false, got true")
	}
	if bb.Readable() != 0 {
		t.Errorf("Expected %v, got %v", 0, bb.Readable())
	}
}

func TestByteBuffer_CanReadCorrectNum(t *testing.T) {
	// write 32 int32s
	bb := New(BigEndian, 10)
	for i := 0; i < 32; i++ {
		bb.WriteInt32(int32(i))
	}
	if !bb.CanRead(32 * 4) {
		t.Errorf("Expected true, got false")
	}
	if bb.Readable() != 32*4 {
		t.Errorf("Expected %v, got %v", 32*4, bb.Readable())
	}
	if !bb.CanRead(32*4 - 1) {
		t.Errorf("Expected false, got true")
	}
	if bb.CanRead(32*4 + 1) {
		t.Errorf("Expected false, got true")
	}
}

func TestByteBuffer_MarkResetReadPos(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	bb.WriteInt16(0x5678)
	_, _ = bb.ReadInt16()
	bb.MarkReadPos()
	val1, err := bb.ReadInt16()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val1 != 0x5678 {
		t.Errorf("Expected %v, got %v", 0x5678, val1)
	}
	bb.ResetReadPosToMark()
	val2, err := bb.ReadInt16()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val2 != 0x5678 {
		t.Errorf("Expected %v, got %v", 0x5678, val2)
	}
}

func TestByteBuffer_ReadBytes(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	bb.WriteInt16(0x5678)
	expected := []byte{0x12, 0x34, 0x56, 0x78}
	val, err := bb.ReadBytes(4)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	for i, b := range val {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}

	// reset and rewrite the buffer
	bb.Reset()
	bb.WriteInt16(0x5678)
	bb.WriteInt16(0x1234)

	// the already read back bytes should not be changes
	for i, b := range val {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
}

func TestByteBuffer_ReadString(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteString("hello")
	expected := "hello"
	val, err := bb.ReadString(5)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != expected {
		t.Errorf("Expected %v, got %v", expected, val)
	}

	// reset and rewrite the buffer
	bb.Reset()
	bb.WriteString("world")
	if val != expected {
		t.Errorf("Expected %v, got %v", expected, val)
	}

	// read back the string
	val, err = bb.ReadString(5)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if val != "world" {
		t.Errorf("Expected %v, got %v", "world", val)
	}
}

func TestByteBuffer_WriteReadAsIoWriterReader(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	bb.WriteInt16(0x5678)
	expected := []byte{0x12, 0x34, 0x56, 0x78}
	val := make([]byte, 4)
	n, err := bb.Read(val)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if n != 4 {
		t.Errorf("Expected %v, got %v", 4, n)
	}
	for i, b := range val {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}
	if bb.NumBytesReadable() != 0 {
		t.Errorf("Expected %v, got %v", 0, bb.NumBytesReadable())
	}

	// reading again should return EOF
	n, err = bb.Read(val)
	if err == nil {
		t.Errorf("Expected EOF, got nil")
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("Expected EOF, got %v", err)
	}
}

func TestByteBuffer_WriteReadAsIoWriterReaderBufferTooSmall(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	bb.WriteInt16(0x5678)
	expected := []byte{0x12, 0x34, 0x56, 0x78}
	val := make([]byte, 3)
	n, err := bb.Read(val)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if n != 3 {
		t.Errorf("Expected %v, got %v", 3, n)
	}

	for i, b := range val {
		if b != expected[i] {
			t.Errorf("Expected %v, got %v", expected[i], b)
		}
	}

	if bb.NumBytesReadable() != 1 {
		t.Errorf("Expected %v, got %v", 1, bb.NumBytesReadable())
	}

	// read the rest
	n, err = bb.Read(val)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if n != 1 {
		t.Errorf("Expected %v, got %v", 1, n)
	}

	if val[0] != expected[3] {
		t.Errorf("Expected %v, got %v", expected[3], val[0])
	}

	if bb.NumBytesReadable() != 0 {
		t.Errorf("Expected %v, got %v", 0, bb.NumBytesReadable())
	}

	// Now should get EOF
	n, err = bb.Read(val)
	if err == nil {
		t.Errorf("Expected EOF, got nil")
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("Expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected %v, got %v", 0, n)
	}
}

func TestByteBuffer_DiscardReadBytes(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	bb.WriteInt16(0x5678)
	bb.DiscardReadBytes() // a no-op since we haven't read anything
	if bb.NumBytesReadable() != 4 {
		t.Errorf("Expected %v, got %v", 4, bb.NumBytesReadable())
	}

	_, _ = bb.ReadInt16()
	bb.DiscardReadBytes()
	if bb.NumBytesReadable() != 2 {
		t.Errorf("Expected %v, got %v", 2, bb.NumBytesReadable())
	}

	// read the rest
	val, err := bb.ReadInt16()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if val != 0x5678 {
		t.Errorf("Expected %v, got %v", 0x5678, val)
	}

	bb.DiscardReadBytes()
	if bb.NumBytesReadable() != 0 {
		t.Errorf("Expected %v, got %v", 0, bb.NumBytesReadable())
	}
}

func TestBuffer_EnsureSpareBytes(t *testing.T) {
	bb := New(BigEndian, 10)
	bb.WriteInt16(0x1234)
	bb.WriteInt16(0x5678)
	bb.EnsureSpareCapacity(10)
	if cap(bb.buf) != 14 {
		t.Errorf("Expected %v, got %v", 14, cap(bb.buf))
	}

	if bb.NumBytesReadable() != 4 {
		t.Errorf("Expected %v, got %v", 4, bb.NumBytesReadable())
	}

	if bb.ReadPos() != 0 {
		t.Errorf("Expected %v, got %v", 0, bb.ReadPos())
	}

	val, err := bb.ReadInt32()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if val != 0x12345678 {
		t.Errorf("Expected %v, got %v", 0x12345678, val)
	}

	writeable := bb.UnderlyingWriteable()
	if len(writeable) != 10 {
		t.Errorf("Expected %v, got %v", 10, len(writeable))
	}

	// write 10 bytes
	for i := 0; i < 10; i++ {
		writeable[i] = byte(i)
	}
}
