package secureproto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
)

type DataFrameV2 struct {
	Type         string `json:"type"`
	ProtoVersion string `json:"proto_version"`
	Channel      uint8  `json:"channel"`
	Sequence     uint32 `json:"sequence"`
	Mode         string `json:"mode"`
	Ciphertext   string `json:"ciphertext"`
	Padding      string `json:"padding"`
}

func BuildDataFrameV2(sessionKey []byte, channel uint8, seq uint32, mode string, payload []byte, maxPad int) ([]byte, error) {
	enc, err := Encrypt(sessionKey, payload)
	if err != nil {
		return nil, err
	}
	pad, err := randomPadding(maxPad)
	if err != nil {
		return nil, err
	}
	frame := DataFrameV2{
		Type:         TypeData,
		ProtoVersion: "v2",
		Channel:      channel,
		Sequence:     seq,
		Mode:         mode,
		Ciphertext:   enc,
		Padding:      base64.StdEncoding.EncodeToString(pad),
	}
	return json.Marshal(frame)
}

func ParseDataFrameV2(sessionKey []byte, raw []byte) (DataFrameV2, []byte, error) {
	var frame DataFrameV2
	if err := json.Unmarshal(raw, &frame); err != nil {
		return DataFrameV2{}, nil, err
	}
	if frame.Type != TypeData || frame.ProtoVersion != "v2" {
		return DataFrameV2{}, nil, fmt.Errorf("invalid v2 frame")
	}
	payload, err := Decrypt(sessionKey, frame.Ciphertext)
	if err != nil {
		return DataFrameV2{}, nil, err
	}
	return frame, payload, nil
}

func randomPadding(maxPad int) ([]byte, error) {
	if maxPad <= 0 {
		return nil, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(maxPad+1)))
	if err != nil {
		return nil, err
	}
	pad := make([]byte, int(n.Int64()))
	if len(pad) == 0 {
		return nil, nil
	}
	if _, err := io.ReadFull(rand.Reader, pad); err != nil {
		return nil, err
	}
	return pad, nil
}
