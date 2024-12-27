package snail_parser

import (
	"encoding/json"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/google/go-cmp/cmp"
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
		res.Err = fmt.Errorf("failed to parse int: %w", err)
		return res
	}
	return ParseOneResult[int]{Value: int(i32), Status: ParseOneStatusOK}
}

func IntWriter(buffer *snail_buffer.Buffer, i int) error {
	buffer.WriteInt32(int32(i))
	return nil
}

func TestParseOne(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	buffer.WriteInt32(42)
	buffer.WriteInt32(43)

	result := parseOne(buffer, IntParser)
	if result.Status != ParseOneStatusOK {
		t.Fatalf("expected status OK, got %v", result.Status)
	}

	if result.Value != 42 {
		t.Fatalf("expected value 42, got %v", result.Value)
	}

	result = parseOne(buffer, IntParser)
	if result.Status != ParseOneStatusOK {
		t.Fatalf("expected status OK, got %v", result.Status)
	}

	if result.Value != 43 {
		t.Fatalf("expected value 43, got %v", result.Value)
	}

	result = parseOne(buffer, IntParser)
	if result.Status != ParseOneStatusNEB {
		t.Fatalf("expected status NEB, got %v", result.Status)
	}
}

func TestParseAll(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	buffer.WriteInt32(42)
	buffer.WriteInt32(43)

	results, err := ParseAll(buffer, IntParser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %v", len(results))
	}

	if results[0] != 42 {
		t.Fatalf("expected value 42, got %v", results[0])
	}

	if results[1] != 43 {
		t.Fatalf("expected value 43, got %v", results[1])
	}

	if buffer.NumBytesReadable() != 0 {
		t.Fatalf("expected 0 bytes readable, got %v", buffer.NumBytesReadable())
	}
	if buffer.ReadPos() != 0 {
		t.Fatalf("expected read pos 0, got %v", buffer.ReadPos())
	}
}

func TestParseAll_Int32StreamedByteForByte(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.LittleEndian, 1024)
	buffer.WriteBytes([]byte{42})

	results, err := ParseAll(buffer, IntParser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %v", len(results))
	}

	if buffer.NumBytesReadable() != 1 {
		t.Fatalf("expected 1 byte readable, got %v", buffer.NumBytesReadable())
	}

	buffer.WriteBytes([]byte{0})
	buffer.WriteBytes([]byte{0})
	buffer.WriteBytes([]byte{0})

	results, err = ParseAll(buffer, IntParser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %v", len(results))
	}

	if results[0] != 42 {
		t.Fatalf("expected value 42, got %v", results[0])
	}
}

func TestWriteAll(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	err := WriteAll(buffer, IntWriter, []int{42, 43})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buffer.NumBytesReadable() != 8 {
		t.Fatalf("expected 8 bytes readable, got %v", buffer.NumBytesReadable())
	}

	valuesBack, err := ParseAll(buffer, IntParser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(valuesBack) != 2 {
		t.Fatalf("expected 2 results, got %v", len(valuesBack))
	}

	if diff := cmp.Diff([]int{42, 43}, valuesBack); diff != "" {
		t.Fatalf("unexpected results (-want +got):\n%s", diff)
	}

	if buffer.NumBytesReadable() != 0 {
		t.Fatalf("expected 0 bytes readable, got %v", buffer.NumBytesReadable())
	}

	if buffer.ReadPos() != 0 {
		t.Fatalf("expected read pos 0, got %v", buffer.ReadPos())
	}
}

const (
	testStructMaxTextLen = 1024
)

type testStruct struct {
	Type   int32
	Strlen int32
	Text   string
}

func testStructParser(buffer *snail_buffer.Buffer) ParseOneResult[testStruct] {
	var res ParseOneResult[testStruct]
	if buffer.NumBytesReadable() < 8 {
		res.Status = ParseOneStatusNEB
		return res
	}
	type_, err := buffer.ReadInt32()
	if err != nil {
		res.Err = fmt.Errorf("failed to parse type: %w", err)
		return res
	}
	strlen, err := buffer.ReadInt32()
	if err != nil {
		res.Err = fmt.Errorf("failed to parse strlen: %w", err)
		return res
	}

	if strlen > testStructMaxTextLen {
		res.Err = fmt.Errorf("strlen too large: %v", strlen)
		return res
	}

	if buffer.NumBytesReadable() < int(strlen) {
		res.Status = ParseOneStatusNEB
		return res
	}
	text, err := buffer.ReadString(int(strlen))
	if err != nil {
		res.Err = fmt.Errorf("failed to parse text: %w", err)
		return res
	}

	return ParseOneResult[testStruct]{Value: testStruct{Type: type_, Strlen: strlen, Text: text}, Status: ParseOneStatusOK}
}

func testStructWriter(buffer *snail_buffer.Buffer, ts testStruct) error {

	if len(ts.Text) > testStructMaxTextLen {
		return fmt.Errorf("text too long: %v", len(ts.Text))
	}

	if len(ts.Text) != int(ts.Strlen) {
		return fmt.Errorf("strlen does not match text length: %v != %v", ts.Strlen, len(ts.Text))
	}

	buffer.WriteInt32(ts.Type)
	buffer.WriteInt32(ts.Strlen)
	buffer.WriteString(ts.Text)
	return nil
}

func TestWriteAll_WriteReadStructs(t *testing.T) {
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	err := WriteAll(buffer, testStructWriter, []testStruct{
		{Type: 42, Strlen: 4, Text: "test"},
		{Type: 43, Strlen: 5, Text: "test2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buffer.NumBytesReadable() != 25 {
		t.Fatalf("expected 24 bytes readable, got %v", buffer.NumBytesReadable())
	}

	valuesBack, err := ParseAll(buffer, testStructParser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(valuesBack) != 2 {
		t.Fatalf("expected 2 results, got %v", len(valuesBack))
	}

	if diff := cmp.Diff([]testStruct{
		{Type: 42, Strlen: 4, Text: "test"},
		{Type: 43, Strlen: 5, Text: "test2"},
	}, valuesBack); diff != "" {
		t.Fatalf("unexpected results (-want +got):\n%s", diff)
	}

	if buffer.NumBytesReadable() != 0 {
		t.Fatalf("expected 0 bytes readable, got %v", buffer.NumBytesReadable())
	}

	if buffer.ReadPos() != 0 {
		t.Fatalf("expected read pos 0, got %v", buffer.ReadPos())
	}
}

type jsonTestStruct struct {
	Type int32  `json:"type"`
	Text string `json:"text"`
	Bla  string `json:"bla"`
	Foo  string `json:"foo"`
	Bar  string `json:"bar"`
}

func TestWriteAll_WriteReadJson(t *testing.T) {

	codec := NewJsonLinesCodec[jsonTestStruct]()

	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	err := WriteAll(buffer, codec.Writer, []jsonTestStruct{
		{Type: 42, Text: "test", Bla: "bla", Foo: "foo", Bar: "bar"},
		{Type: 43, Text: "test2", Bla: "bla2", Foo: "foo2", Bar: "bar2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buffer.NumBytesReadable() != 128 {
		t.Fatalf("expected 24 bytes readable, got %v", buffer.NumBytesReadable())
	}

	thirdObject := jsonTestStruct{Type: 44, Text: "test3", Bla: "bla3", Foo: "foo3", Bar: "bar3"}
	thirdObjectBytes, err := json.Marshal(thirdObject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	thirdObjectBytes = append(thirdObjectBytes, 0x0A)

	// write the first half of the third object
	buffer.WriteBytes(thirdObjectBytes[:len(thirdObjectBytes)/2])

	valuesBack, err := ParseAll(buffer, codec.Parser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(valuesBack) != 2 {
		t.Fatalf("expected 2 results, got %v", len(valuesBack))
	}

	if diff := cmp.Diff([]jsonTestStruct{
		{Type: 42, Text: "test", Bla: "bla", Foo: "foo", Bar: "bar"},
		{Type: 43, Text: "test2", Bla: "bla2", Foo: "foo2", Bar: "bar2"},
	}, valuesBack); diff != "" {
		t.Fatalf("unexpected results (-want +got):\n%s", diff)
	}

	// write the second half of the third object
	buffer.WriteBytes(thirdObjectBytes[len(thirdObjectBytes)/2:])

	valuesBack, err = ParseAll(buffer, codec.Parser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(valuesBack) != 1 {
		t.Fatalf("expected 1 result, got %v", len(valuesBack))
	}

	if diff := cmp.Diff([]jsonTestStruct{
		{Type: 44, Text: "test3", Bla: "bla3", Foo: "foo3", Bar: "bar3"},
	}, valuesBack); diff != "" {
		t.Fatalf("unexpected results (-want +got):\n%s", diff)
	}

	if buffer.NumBytesReadable() != 0 {
		t.Fatalf("expected 0 bytes readable, got %v", buffer.NumBytesReadable())
	}

	if buffer.ReadPos() != 0 {
		t.Fatalf("expected read pos 0, got %v", buffer.ReadPos())
	}
}
