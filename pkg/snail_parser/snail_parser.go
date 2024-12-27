package snail_parser

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
)

type ParseOneStatus int

const (
	ParseOneStatusOK  ParseOneStatus = iota
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

func parseOne[T any](
	buffer *snail_buffer.Buffer,
	parseFunc ParseFunc[T],
) ParseOneResult[T] {
	return parseFunc(buffer)
}

func ParseAll[T any](
	buffer *snail_buffer.Buffer,
	parseFunc ParseFunc[T],
) ([]T, error) {
	var results []T
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

func WriteAll[T any](
	buffer *snail_buffer.Buffer,
	writeFunc WriteFunc[T],
	items []T,
) error {
	for _, item := range items {
		if err := writeFunc(buffer, item); err != nil {
			return fmt.Errorf("failed to write: %w", err)
		}
	}
	return nil
}
