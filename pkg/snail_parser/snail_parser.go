package snail_parser

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
)

type ParseOneStatus int

const (
	// ParseOneStatusOK means that all data was parsed
	ParseOneStatusOK ParseOneStatus = iota
	// ParseOneStatusNEB means that not enough data was available to parse all the data.
	// This is normal in streamed data environments. However, some of the data may have been was parsed,
	// and the rest will be available in the next call
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
