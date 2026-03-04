package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	Version1 byte = 1

	FrameHandshake byte = 1
	FrameData      byte = 2
	FrameControl   byte = 3
)

type Header struct {
	Version   byte
	FrameType byte
	SessionID uint32
	StreamID  uint32
	Payload   uint16
}

const HeaderSize = 12

func EncodeHeader(h Header) ([]byte, error) {
	if h.Payload == 0 {
		return nil, errors.New("payload size must be > 0")
	}
	out := make([]byte, HeaderSize)
	out[0] = h.Version
	out[1] = h.FrameType
	binary.BigEndian.PutUint32(out[2:6], h.SessionID)
	binary.BigEndian.PutUint32(out[6:10], h.StreamID)
	binary.BigEndian.PutUint16(out[10:12], h.Payload)
	return out, nil
}

func DecodeHeader(b []byte) (Header, error) {
	if len(b) < HeaderSize {
		return Header{}, fmt.Errorf("short header: %d", len(b))
	}
	h := Header{
		Version:   b[0],
		FrameType: b[1],
		SessionID: binary.BigEndian.Uint32(b[2:6]),
		StreamID:  binary.BigEndian.Uint32(b[6:10]),
		Payload:   binary.BigEndian.Uint16(b[10:12]),
	}
	if h.Version != Version1 {
		return Header{}, fmt.Errorf("unsupported version: %d", h.Version)
	}
	if h.Payload == 0 {
		return Header{}, errors.New("empty payload")
	}
	return h, nil
}
