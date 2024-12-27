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
