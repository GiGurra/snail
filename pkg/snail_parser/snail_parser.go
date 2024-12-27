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

type ParseOneResult[T any] struct {
	Value  T
	Status ParseOneStatus
	Err    error
}

func parseOne[T any](
	buffer *snail_buffer.Buffer,
	parseFunc func(buffer *snail_buffer.Buffer) ParseOneResult[T],
) ParseOneResult[T] {
	return parseFunc(buffer)
}

func ParseAll[T any](
	buffer *snail_buffer.Buffer,
	parseFunc func(*snail_buffer.Buffer) ParseOneResult[T],
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
	writeFunc func(buffer *snail_buffer.Buffer, t T) error,
	items []T,
) error {
	for _, item := range items {
		if err := writeFunc(buffer, item); err != nil {
			return fmt.Errorf("failed to write: %w", err)
		}
	}
	return nil
}
