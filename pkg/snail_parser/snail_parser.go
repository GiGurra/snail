package snail_parser

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
)

type ParseOneStatus int

const (
	ParseOneStatusOK      ParseOneStatus = iota
	ParseOneStatusNEB     ParseOneStatus = iota
	ParseOneStatusInvalid ParseOneStatus = iota
)

type ParseOneResult[T any] struct {
	Value  T
	Status ParseOneStatus
	Err    error
}

func ParseOne[T any](
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
		switch result.Status {
		case ParseOneStatusOK:
			results = append(results, result.Value)
		case ParseOneStatusNEB:
			buffer.SetReadPos(bufferReadPosBefore)
			return results, nil
		case ParseOneStatusInvalid:
			return results, fmt.Errorf("failed to parse, stream corrupt: %w", result.Err)
		}
	}
}
