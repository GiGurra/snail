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
