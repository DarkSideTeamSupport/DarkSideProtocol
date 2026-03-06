package secureproto

import (
	"encoding/json"
	"fmt"
)

type DatagramFrameV2 struct {
	Type         string `json:"type"`
	ProtoVersion string `json:"proto_version"`
	SessionID    string `json:"session_id"`
	Channel      uint8  `json:"channel"`
	Sequence     uint32 `json:"sequence"`
	Mode         string `json:"mode"`
	Ciphertext   string `json:"ciphertext"`
	Padding      string `json:"padding"`
}

func BuildDatagramFrameV2(sessionKey []byte, sessionID string, channel uint8, seq uint32, mode string, payload []byte, maxPad int) ([]byte, error) {
	frameRaw, err := BuildDataFrameV2(sessionKey, channel, seq, mode, payload, maxPad)
	if err != nil {
		return nil, err
	}
	var dataFrame DataFrameV2
	if err := json.Unmarshal(frameRaw, &dataFrame); err != nil {
		return nil, err
	}
	out := DatagramFrameV2{
		Type:         dataFrame.Type,
		ProtoVersion: dataFrame.ProtoVersion,
		SessionID:    sessionID,
		Channel:      dataFrame.Channel,
		Sequence:     dataFrame.Sequence,
		Mode:         dataFrame.Mode,
		Ciphertext:   dataFrame.Ciphertext,
		Padding:      dataFrame.Padding,
	}
	return json.Marshal(out)
}

func ParseDatagramSessionID(raw []byte) (string, error) {
	var header struct {
		Type         string `json:"type"`
		ProtoVersion string `json:"proto_version"`
		SessionID    string `json:"session_id"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return "", err
	}
	if header.Type != TypeData || header.ProtoVersion != "v2" || header.SessionID == "" {
		return "", fmt.Errorf("invalid datagram v2 header")
	}
	return header.SessionID, nil
}

func ParseDatagramFrameV2(sessionKey []byte, raw []byte) (DatagramFrameV2, []byte, error) {
	var d DatagramFrameV2
	if err := json.Unmarshal(raw, &d); err != nil {
		return DatagramFrameV2{}, nil, err
	}
	if d.Type != TypeData || d.ProtoVersion != "v2" || d.SessionID == "" {
		return DatagramFrameV2{}, nil, fmt.Errorf("invalid datagram v2")
	}
	plain, err := Decrypt(sessionKey, d.Ciphertext)
	if err != nil {
		return DatagramFrameV2{}, nil, err
	}
	return d, plain, nil
}
