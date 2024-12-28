package snail_parser

import (
	"encoding/json"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
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
		t.Fatalf("expected 25 bytes readable, got %v", buffer.NumBytesReadable())
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
		t.Fatalf("expected 128 bytes readable, got %v", buffer.NumBytesReadable())
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

func TestJsonCanSerializePtrs(t *testing.T) {
	codec := NewJsonLinesCodec[*jsonTestStruct]()
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)

	// Write a pointer to a struct
	data := &jsonTestStruct{Type: 42, Text: "test", Bla: "bla", Foo: "foo", Bar: "bar"}
	err := codec.Writer(buffer, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read the pointer back
	res := codec.Parser(buffer)
	if res.Status != ParseOneStatusOK {
		t.Fatalf("expected status OK, got %v", res.Status)
	}

	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}

	if diff := cmp.Diff(data, res.Value); diff != "" {
		t.Fatalf("unexpected results (-want +got):\n%s", diff)
	}
}

func TestIntParserPerformance(t *testing.T) {

	testLength := 1 * time.Second

	t0 := time.Now()
	codec := NewInt32Codec()
	buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
	numReqs := int64(0)
	for i := 0; time.Since(t0) < testLength; i++ {

		for side := 0; side < 2; side++ {

			// write
			err := codec.Writer(buffer, int32(i))
			if err != nil {
				panic(fmt.Errorf("failed to encode int: %w", err))
			}

			// read it back
			res := codec.Parser(buffer)
			if res.Status != ParseOneStatusOK {
				panic(fmt.Errorf("failed to parse int: %v", res.Status))
			}
			if res.Err != nil {
				panic(fmt.Errorf("failed to parse int: %w", res.Err))
			}
			if res.Value != int32(i) {
				panic(fmt.Errorf("unexpected value: %v", res.Value))
			}
		}
		numReqs++
	}

	rate := float64(numReqs) / testLength.Seconds()

	slog.Info(fmt.Sprintf("Parsed %v ints in %v", prettyInt3Digits(int64(numReqs)), time.Since(t0)))
	slog.Info(fmt.Sprintf("Rate: %s ints/sec", prettyInt3Digits(int64(rate))))

}

func TestIntParserPerformance_routines(t *testing.T) {

	numGoRoutines := 512
	testLength := 1 * time.Second

	t0 := time.Now()
	codec := NewInt32Codec()

	numReqsTot := atomic.Int64{}

	lop.ForEach(lo.Range(numGoRoutines), func(i int, _ int) {
		buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
		numReqs := int64(0)
		for i := 0; time.Since(t0) < testLength; i++ {

			for side := 0; side < 2; side++ {

				// write
				err := codec.Writer(buffer, int32(i))
				if err != nil {
					panic(fmt.Errorf("failed to encode int: %w", err))
				}

				// read it back
				res := codec.Parser(buffer)
				if res.Status != ParseOneStatusOK {
					panic(fmt.Errorf("failed to parse int: %v", res.Status))
				}
				if res.Err != nil {
					panic(fmt.Errorf("failed to parse int: %w", res.Err))
				}
				if res.Value != int32(i) {
					panic(fmt.Errorf("unexpected value: %v", res.Value))
				}
			}
			numReqs++
		}

		numReqsTot.Add(numReqs)
	})

	numReqs := numReqsTot.Load()
	rate := float64(numReqs) / testLength.Seconds()

	slog.Info(fmt.Sprintf("Parsed %v ints in %v", prettyInt3Digits(int64(numReqs)), time.Since(t0)))
	slog.Info(fmt.Sprintf("Rate: %s ints/sec", prettyInt3Digits(int64(rate))))

}

func TestJsonParserPerformance_routines(t *testing.T) {

	numGoRoutines := 512
	testLength := 1 * time.Second

	t0 := time.Now()
	codec := NewJsonLinesCodec[jsonTestStruct]()

	numReqsTot := atomic.Int64{}

	sides := 1

	lop.ForEach(lo.Range(numGoRoutines), func(i int, _ int) {
		buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
		numReqs := int64(0)
		for i := 0; time.Since(t0) < testLength; i++ {

			for side := 0; side < sides; side++ {

				// write
				err := codec.Writer(buffer, jsonTestStruct{Type: int32(i), Text: "test", Bla: "bla", Foo: "foo", Bar: "bar"})
				if err != nil {
					panic(fmt.Errorf("failed to encode int: %w", err))
				}

				// read it back
				res := codec.Parser(buffer)
				if res.Status != ParseOneStatusOK {
					panic(fmt.Errorf("failed to parse int: %v", res.Status))
				}
				if res.Err != nil {
					panic(fmt.Errorf("failed to parse int: %w", res.Err))
				}
				if res.Value.Type != int32(i) {
					panic(fmt.Errorf("unexpected value: %v", res.Value.Type))
				}
			}
			numReqs++
		}

		numReqsTot.Add(numReqs)
	})

	numReqs := numReqsTot.Load()
	rate := float64(numReqs) / testLength.Seconds()

	slog.Info(fmt.Sprintf("Sides: %v", sides))
	slog.Info(fmt.Sprintf("Parsed sides x pairs of %v json structs in %v", prettyInt3Digits(int64(numReqs)), time.Since(t0)))
	slog.Info(fmt.Sprintf("Rate: %s sides x pairs/sec", prettyInt3Digits(int64(rate))))

}

func TestCustomStructParserPerformance_routines(t *testing.T) {

	numGoRoutines := 512
	testLength := 1 * time.Second

	t0 := time.Now()
	codec := newCustomStructCodec()

	numReqsTot := atomic.Int64{}

	sides := 1

	lop.ForEach(lo.Range(numGoRoutines), func(i int, _ int) {
		buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
		numReqs := int64(0)
		for i := 0; time.Since(t0) < testLength; i++ {

			for side := 0; side < sides; side++ {

				// write
				err := codec.Writer(buffer, jsonTestStruct{Type: int32(i), Text: "test", Bla: "bla", Foo: "foo", Bar: "bar"})
				if err != nil {
					panic(fmt.Errorf("failed to encode int: %w", err))
				}

				// read it back
				res := codec.Parser(buffer)
				if res.Status != ParseOneStatusOK {
					panic(fmt.Errorf("failed to parse int: %v", res.Status))
				}
				if res.Err != nil {
					panic(fmt.Errorf("failed to parse int: %w", res.Err))
				}
				if res.Value.Type != int32(i) {
					panic(fmt.Errorf("unexpected value: %v", res.Value.Type))
				}
				if res.Value.Text != "test" {
					panic(fmt.Errorf("unexpected value: %v", res.Value.Text))
				}
				if res.Value.Bla != "bla" {
					panic(fmt.Errorf("unexpected value: %v", res.Value.Bla))
				}
				if res.Value.Foo != "foo" {
					panic(fmt.Errorf("unexpected value: %v", res.Value.Foo))
				}
				if res.Value.Bar != "bar" {
					panic(fmt.Errorf("unexpected value: %v", res.Value.Bar))
				}
			}
			numReqs++
		}

		numReqsTot.Add(numReqs)
	})

	numReqs := numReqsTot.Load()
	rate := float64(numReqs) / testLength.Seconds()

	slog.Info(fmt.Sprintf("Sides: %v", sides))
	slog.Info(fmt.Sprintf("Parsed sides x pairs of %v structs in %v", prettyInt3Digits(int64(numReqs)), time.Since(t0)))
	slog.Info(fmt.Sprintf("Rate: %s sides x pairs/sec", prettyInt3Digits(int64(rate))))

}

type requestTestStruct struct {
	Header int32
	ID     int16
}

func TestSmallCustomStructParserPerformance_routines(t *testing.T) {

	numGoRoutines := 512
	testLength := 1 * time.Second

	t0 := time.Now()
	codec := newRequestTestStructCodec()

	structData := requestTestStruct{
		Header: 1241423151,
		ID:     12332,
	}

	numReqsTot := atomic.Int64{}

	sides := 1

	lop.ForEach(lo.Range(numGoRoutines), func(i int, _ int) {
		buffer := snail_buffer.New(snail_buffer.BigEndian, 1024)
		numReqs := int64(0)
		for i := 0; time.Since(t0) < testLength; i++ {

			for side := 0; side < sides; side++ {

				// write
				err := codec.Writer(buffer, structData)
				if err != nil {
					panic(fmt.Errorf("failed to encode int: %w", err))
				}

				// read it back
				res := codec.Parser(buffer)
				if res.Status != ParseOneStatusOK {
					panic(fmt.Errorf("failed to parse int: %v", res.Status))
				}
				if res.Err != nil {
					panic(fmt.Errorf("failed to parse int: %w", res.Err))
				}
				if res.Value.Header != structData.Header {
					panic(fmt.Errorf("unexpected value: %v", res.Value.Header))
				}
				if res.Value.ID != structData.ID {
					panic(fmt.Errorf("unexpected value: %v", res.Value.ID))
				}
			}
			numReqs++
		}

		numReqsTot.Add(numReqs)
	})

	numReqs := numReqsTot.Load()
	rate := float64(numReqs) / testLength.Seconds()

	slog.Info(fmt.Sprintf("Sides: %v", sides))
	slog.Info(fmt.Sprintf("Parsed sides x pairs of %v structs in %v", prettyInt3Digits(int64(numReqs)), time.Since(t0)))
	slog.Info(fmt.Sprintf("Rate: %s sides x pairs/sec", prettyInt3Digits(int64(rate))))

}

var prettyPrinter = message.NewPrinter(language.English)

func prettyInt3Digits(n int64) string {
	return prettyPrinter.Sprintf("%d", n)
}

func newCustomStructCodec() Codec[jsonTestStruct] {
	return Codec[jsonTestStruct]{
		Parser: func(buffer *snail_buffer.Buffer) ParseOneResult[jsonTestStruct] {

			if buffer.NumBytesReadable() < 5*4 {
				return ParseOneResult[jsonTestStruct]{Status: ParseOneStatusNEB}
			}

			res := ParseOneResult[jsonTestStruct]{}

			readPosBefore := buffer.ReadPos()

			notEnoughBytes := func() ParseOneResult[jsonTestStruct] {
				res.Status = ParseOneStatusNEB
				buffer.SetReadPos(readPosBefore)
				return res
			}

			invalid := func(err error) ParseOneResult[jsonTestStruct] {
				res.Err = err
				return res
			}

			var err error
			res.Value.Type, err = buffer.ReadInt32()
			if err != nil {
				return invalid(fmt.Errorf("failed to parse type: %w", err))
			}

			//Text string `json:"text"`
			textLen, err := buffer.ReadInt32()
			if err != nil {
				return invalid(fmt.Errorf("failed to parse text len: %w", err))
			}
			if buffer.NumBytesReadable() < int(textLen) {
				return notEnoughBytes()
			}
			res.Value.Text, err = buffer.ReadString(int(textLen))
			if err != nil {
				return invalid(fmt.Errorf("failed to parse text: %w", err))
			}

			//Bla  string `json:"bla"`
			blaLen, err := buffer.ReadInt32()
			if err != nil {
				return invalid(fmt.Errorf("failed to parse bla len: %w", err))
			}
			if buffer.NumBytesReadable() < int(blaLen) {
				return notEnoughBytes()
			}
			res.Value.Bla, err = buffer.ReadString(int(blaLen))
			if err != nil {
				return invalid(fmt.Errorf("failed to parse bla: %w", err))
			}

			//Foo  string `json:"foo"`
			fooLen, err := buffer.ReadInt32()
			if err != nil {
				return invalid(fmt.Errorf("failed to parse foo len: %w", err))
			}
			if buffer.NumBytesReadable() < int(fooLen) {
				return notEnoughBytes()
			}
			res.Value.Foo, err = buffer.ReadString(int(fooLen))
			if err != nil {
				return invalid(fmt.Errorf("failed to parse foo: %w", err))
			}

			//Bar  string `json:"bar"`
			barLen, err := buffer.ReadInt32()
			if err != nil {
				return invalid(fmt.Errorf("failed to parse bar len: %w", err))
			}
			if buffer.NumBytesReadable() < int(barLen) {
				return notEnoughBytes()
			}
			res.Value.Bar, err = buffer.ReadString(int(barLen))
			if err != nil {
				return invalid(fmt.Errorf("failed to parse bar: %w", err))
			}

			res.Status = ParseOneStatusOK
			return res
		},

		Writer: func(buffer *snail_buffer.Buffer, t jsonTestStruct) error {

			buffer.WriteInt32(t.Type)
			buffer.WriteInt32(int32(len(t.Text)))
			buffer.WriteString(t.Text)
			buffer.WriteInt32(int32(len(t.Bla)))
			buffer.WriteString(t.Bla)
			buffer.WriteInt32(int32(len(t.Foo)))
			buffer.WriteString(t.Foo)
			buffer.WriteInt32(int32(len(t.Bar)))
			buffer.WriteString(t.Bar)

			return nil
		},
	}
}

func newRequestTestStructCodec() Codec[requestTestStruct] {

	serializedSize := 4 + 2

	return Codec[requestTestStruct]{
		Parser: func(buffer *snail_buffer.Buffer) ParseOneResult[requestTestStruct] {

			if buffer.NumBytesReadable() < serializedSize {
				return ParseOneResult[requestTestStruct]{Status: ParseOneStatusNEB}
			}

			res := ParseOneResult[requestTestStruct]{}

			invalid := func(err error) ParseOneResult[requestTestStruct] {
				res.Err = err
				return res
			}

			var err error
			res.Value.Header, err = buffer.ReadInt32()
			if err != nil {
				return invalid(fmt.Errorf("failed to parse header: %w", err))
			}

			res.Value.ID, err = buffer.ReadInt16()
			if err != nil {
				return invalid(fmt.Errorf("failed to parse ID: %w", err))
			}

			res.Status = ParseOneStatusOK
			return res
		},

		Writer: func(buffer *snail_buffer.Buffer, t requestTestStruct) error {

			buffer.WriteInt32(t.Header)
			buffer.WriteInt16(t.ID)

			return nil
		},
	}
}
