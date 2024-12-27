package snail_parser

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"testing"
)

func IntParser(buffer *snail_buffer.Buffer) ParseOneResult[int] {
	var res ParseOneResult[int]
	if buffer.NumBytesReadable() < 4 {
		res.Status = ParseOneStatusNEB
		return res
	}
	i32, err := buffer.ReadInt32()
	if err != nil {
		res.Status = ParseOneStatusInvalid
		res.Err = fmt.Errorf("failed to parse int: %w", err)
		return res
	}
	return ParseOneResult[int]{Value: int(i32), Status: ParseOneStatusOK}
}

func TestParseOne(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	buffer.WriteInt32(42)
	buffer.WriteInt32(43)

	result := ParseOne(buffer, IntParser)
	if result.Status != ParseOneStatusOK {
		t.Errorf("expected status OK, got %v", result.Status)
	}

	if result.Value != 42 {
		t.Errorf("expected value 42, got %v", result.Value)
	}

	result = ParseOne(buffer, IntParser)
	if result.Status != ParseOneStatusOK {
		t.Errorf("expected status OK, got %v", result.Status)
	}

	if result.Value != 43 {
		t.Errorf("expected value 43, got %v", result.Value)
	}

	result = ParseOne(buffer, IntParser)
	if result.Status != ParseOneStatusNEB {
		t.Errorf("expected status NEB, got %v", result.Status)
	}
}

func TestParseAll(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	buffer.WriteInt32(42)
	buffer.WriteInt32(43)

	results, err := ParseAll(buffer, IntParser)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %v", len(results))
	}

	if results[0] != 42 {
		t.Errorf("expected value 42, got %v", results[0])
	}

	if results[1] != 43 {
		t.Errorf("expected value 43, got %v", results[1])
	}

	if buffer.ReadPos() != 8 {
		t.Errorf("expected read pos 8, got %v", buffer.ReadPos())
	}
}

func TestParseAll_Int32StreamedByteForByte(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.LittleEndian, 1024)
	buffer.WriteBytes([]byte{42})

	results, err := ParseAll(buffer, IntParser)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %v", len(results))
	}

	if buffer.NumBytesReadable() != 1 {
		t.Errorf("expected 1 byte readable, got %v", buffer.NumBytesReadable())
	}

	buffer.WriteBytes([]byte{0})
	buffer.WriteBytes([]byte{0})
	buffer.WriteBytes([]byte{0})

	results, err = ParseAll(buffer, IntParser)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %v", len(results))
	}

	if results[0] != 42 {
		t.Errorf("expected value 42, got %v", results[0])
	}
}
