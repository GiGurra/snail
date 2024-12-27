package snail_legacy_prototype

import (
	"fmt"
	"github.com/GiGurra/snail/pkg/snail_buffer"
	"unsafe"
)

type MsgType uint8

const (
	MaxMsgLength = 1024
)

const (
	CstRequest  MsgType = iota
	CstResponse MsgType = iota
)

type CstDecision uint8

const (
	Approved CstDecision = 0
	Denied   CstDecision = 1
	Pending  CstDecision = 2
)

type CustomProtoMsg struct {
	RequestLength   int32
	Type            MsgType
	RequestIdLength uint8
	RequestId       string
	CstDecision     CstDecision
	// New params go here. Switch to protobuf? :D
}

var minMsgSize = calculateRequestLength(CustomProtoMsg{})

func TryReadMsgs(accumBuf *snail_buffer.Buffer) ([]CustomProtoMsg, error) {

	msgsOut := make([]CustomProtoMsg, 0, 2)

	for {

		accumBuf.MarkReadPos()

		// parse messages if there is enough info. Shift the buffer if there is extra data at the end.
		if accumBuf.NumBytesReadable() < int(minMsgSize) {
			//slog.Warn("Not enough data to read a message")
			break
		}

		// first we read the request length, big endian
		requestLength, err := accumBuf.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("failed to read request length: %w", err)
		}

		if requestLength > MaxMsgLength {
			return nil, fmt.Errorf("request length is too long: %d, stream corrupt :(", requestLength)
		}

		// Now we check if we have enough data to read the whole message
		if accumBuf.NumBytesReadable()+4 < int(requestLength) {
			accumBuf.ResetReadPosToMark()
			break
		}

		// Then we read the type
		tpe, err := accumBuf.ReadUInt8()
		if err != nil {
			return nil, fmt.Errorf("failed to read type: %w", err)
		}
		msgType := MsgType(tpe)
		if msgType != CstRequest && msgType != CstResponse {
			return nil, fmt.Errorf("unknown message type: %d", msgType)
		}

		// Then we read the request id length
		requestIdLength, err := accumBuf.ReadUInt8()
		if err != nil {
			return nil, fmt.Errorf("failed to read request id length: %w", err)
		}

		// Then we read the request id
		requestId, err := accumBuf.ReadString(int(requestIdLength))
		if err != nil {
			return nil, fmt.Errorf("failed to read request id: %w", err)
		}

		// Then we read the decision
		decisionU8, err := accumBuf.ReadUInt8()
		if err != nil {
			return nil, fmt.Errorf("failed to read decision: %w", err)
		}

		msgsOut = append(msgsOut, CustomProtoMsg{
			RequestLength:   requestLength,
			Type:            msgType,
			RequestIdLength: requestIdLength,
			RequestId:       requestId,
			CstDecision:     CstDecision(decisionU8),
		})

	}

	// Now we have read all the data, we can shift the buffer
	accumBuf.DiscardReadBytes()

	return msgsOut, nil

}

func WriteMsg(buf *snail_buffer.Buffer, msg CustomProtoMsg) error {
	if msg.RequestLength > MaxMsgLength {
		return fmt.Errorf("request length is too long: %d", msg.RequestLength)
	}
	if msg.RequestLength != calculateRequestLength(msg) {
		return fmt.Errorf("request length does not match calculated request length: %d != %d", msg.RequestLength, calculateRequestLength(msg))
	}
	if msg.Type != CstRequest && msg.Type != CstResponse {
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}
	if len(msg.RequestId) != int(msg.RequestIdLength) {
		return fmt.Errorf("request id length does not match request id length: %d != %d", len(msg.RequestId), msg.RequestIdLength)
	}
	buf.WriteInt32(msg.RequestLength)
	buf.WriteUInt8(uint8(msg.Type))
	buf.WriteUInt8(msg.RequestIdLength)
	buf.WriteBytes([]byte(msg.RequestId))
	buf.WriteUInt8(uint8(msg.CstDecision))

	return nil
}

func calculateRequestLength(msg CustomProtoMsg) int32 {
	return int32(SizeOf(msg.RequestLength) +
		SizeOf(msg.Type) +
		SizeOf(msg.RequestIdLength) +
		len(msg.RequestId) +
		SizeOf(msg.CstDecision))
}

func BuildCustomProtoMsg(
	msgType MsgType,
	requestId string,
	decision CstDecision,
) CustomProtoMsg {

	res := CustomProtoMsg{
		RequestLength:   0, // calculated below
		Type:            msgType,
		RequestIdLength: uint8(len(requestId)),
		RequestId:       requestId,
		CstDecision:     decision,
	}

	res.RequestLength = calculateRequestLength(res)

	return res
}

func SizeOf[T any](t T) int {
	return int(unsafe.Sizeof(t))
}
