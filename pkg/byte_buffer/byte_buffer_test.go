package byte_buffer

import "testing"

func TestNewByteBuffer(t *testing.T) {
	bb := NewByteBuffer(BigEndian, 10)
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

func TestByteBuffer_WriteInt32BigEndian(t *testing.T) {
	bb := NewByteBuffer(BigEndian, 10)
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
	bb := NewByteBuffer(LittleEndian, 10)
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
	bb := NewByteBuffer(BigEndian, 10)
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
	bb := NewByteBuffer(LittleEndian, 10)
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
	bb := NewByteBuffer(BigEndian, 10)
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
	bb := NewByteBuffer(BigEndian, 10)
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
	bb := NewByteBuffer(BigEndian, 10)
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
