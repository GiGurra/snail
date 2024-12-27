package snail_parser

import (
	"encoding/json"
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
)

func NewJsonLinesCodec[T any]() Codec[T] {

	return Codec[T]{

		Parser: func(buffer *snail_buffer.Buffer) ParseOneResult[T] {

			start := buffer.ReadPos()
			bytes := buffer.Underlying()
			ln := len(bytes)

			end := -1
			for i := start; i < ln; i++ {
				if bytes[i] == 0x0A {
					end = i
					break
				}
			}

			res := ParseOneResult[T]{}
			if end == -1 {
				res.Status = ParseOneStatusNEB
				return res
			}

			err := json.Unmarshal(bytes[start:end], &res.Value)
			if err != nil {
				res.Err = fmt.Errorf("failed to unmarshal json: %w", err)
				return res
			}

			buffer.SetReadPos(end + 1)

			res.Status = ParseOneStatusOK
			return res
		},

		Writer: func(buffer *snail_buffer.Buffer, t T) error {

			bytes, err := json.Marshal(t)
			if err != nil {
				return fmt.Errorf("failed to marshal json: %w", err)
			}

			buffer.WriteBytes(bytes)
			buffer.WriteByteNoE(0x0A)

			return nil
		},
	}
}
