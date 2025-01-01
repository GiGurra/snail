package snail_parser

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
)

type ParseOneStatus int

const (
	// ParseOneStatusOK means that an object could be parsed
	ParseOneStatusOK ParseOneStatus = iota
	// ParseOneStatusNEB means that not enough data was available to parse an object.
	// This is normal in streamed data environments. It is expected to be called again once more data is available.
	ParseOneStatusNEB ParseOneStatus = iota
)

type ParseFunc[T any] func(buffer *snail_buffer.Buffer) ParseOneResult[T]
type WriteFunc[T any] func(buffer *snail_buffer.Buffer, t T) error

type Codec[T any] struct {
	Parser ParseFunc[T]
	Writer WriteFunc[T]
}

type ParseOneResult[T any] struct {
	Value  T
	Status ParseOneStatus
	Err    error
}

func ParseAll[T any](
	buffer *snail_buffer.Buffer,
	parseFunc ParseFunc[T],
) ([]T, error) {
	results := make([]T, 0, 8) // 4-16 seems ok
	//var results []T
	for {
		bufferReadPosBefore := buffer.ReadPos()
		result := parseFunc(buffer)
		if result.Err != nil {
			return results, fmt.Errorf("failed to parse, stream corrupt: %w", result.Err)
		}
		switch result.Status {
		case ParseOneStatusOK:
			results = append(results, result.Value)
		case ParseOneStatusNEB:
			buffer.SetReadPos(bufferReadPosBefore)
			buffer.DiscardReadBytes()
			return results, nil
		}
	}
}
