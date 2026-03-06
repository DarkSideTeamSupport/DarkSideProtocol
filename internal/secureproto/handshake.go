package secureproto

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const (
	TypeHello = "hello_v1"
	TypeAck   = "ack_v1"
	TypeData  = "data_v1"
)

type HelloFrame struct {
	Type            string `json:"type"`
	ProtoVersion    string `json:"proto_version"`
	ClientPublicKey string `json:"client_public_key"`
	ClientNonce     string `json:"client_nonce"`
	Timestamp       int64  `json:"timestamp"`
	AuthTag         string `json:"auth_tag"`
}

type AckFrame struct {
	Type        string `json:"type"`
	ServerNonce string `json:"server_nonce"`
	Mac         string `json:"mac"`
}

type DataFrame struct {
	Type       string `json:"type"`
	Ciphertext string `json:"ciphertext"`
}

func NewHello(clientPublicKey string, preSharedKey string, protoVersion string) (HelloFrame, []byte, error) {
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return HelloFrame{}, nil, err
	}
	if protoVersion == "" {
		protoVersion = "v1"
	}
	timestamp := time.Now().Unix()
	authTag := BuildHelloAuth(preSharedKey, clientPublicKey, nonce, timestamp)
	return HelloFrame{
		Type:            TypeHello,
		ProtoVersion:    protoVersion,
		ClientPublicKey: clientPublicKey,
		ClientNonce:     base64.StdEncoding.EncodeToString(nonce),
		Timestamp:       timestamp,
		AuthTag:         authTag,
	}, nonce, nil
}

func HelloJSON(frame HelloFrame) ([]byte, error) {
	return json.Marshal(frame)
}

func ParseHello(payload []byte) (HelloFrame, []byte, error) {
	var h HelloFrame
	if err := json.Unmarshal(payload, &h); err != nil {
		return HelloFrame{}, nil, err
	}
	if h.Type != TypeHello || h.ClientPublicKey == "" || h.ClientNonce == "" {
		return HelloFrame{}, nil, fmt.Errorf("invalid hello")
	}
	if h.ProtoVersion == "" {
		h.ProtoVersion = "v1"
	}
	nonce, err := base64.StdEncoding.DecodeString(h.ClientNonce)
	if err != nil || len(nonce) != 16 {
		return HelloFrame{}, nil, fmt.Errorf("invalid client nonce")
	}
	return h, nonce, nil
}

func BuildHelloAuth(preSharedKey string, clientPublicKey string, clientNonce []byte, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(preSharedKey))
	mac.Write([]byte("hello-v1"))
	mac.Write([]byte(clientPublicKey))
	mac.Write(clientNonce)
	mac.Write([]byte(fmt.Sprintf("%d", timestamp)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyHelloAuth(preSharedKey string, h HelloFrame, clientNonce []byte) bool {
	if preSharedKey == "" {
		return true
	}
	want := BuildHelloAuth(preSharedKey, h.ClientPublicKey, clientNonce, h.Timestamp)
	return hmac.Equal([]byte(want), []byte(h.AuthTag))
}

func ValidateTimestamp(ts int64, allowedSkewSec int64) bool {
	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	return diff <= allowedSkewSec
}

func BuildAck(sessionKey []byte, serverNonce []byte) AckFrame {
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte("ack-v1"))
	mac.Write(serverNonce)
	return AckFrame{
		Type:        TypeAck,
		ServerNonce: base64.StdEncoding.EncodeToString(serverNonce),
		Mac:         base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}
}

func VerifyAck(sessionKey []byte, ack AckFrame) error {
	if ack.Type != TypeAck {
		return fmt.Errorf("invalid ack type")
	}
	serverNonce, err := base64.StdEncoding.DecodeString(ack.ServerNonce)
	if err != nil || len(serverNonce) != 16 {
		return fmt.Errorf("invalid server nonce")
	}
	want := BuildAck(sessionKey, serverNonce)
	if !hmac.Equal([]byte(want.Mac), []byte(ack.Mac)) {
		return fmt.Errorf("ack mac mismatch")
	}
	return nil
}

func DeriveSessionKey(shared []byte, clientNonce []byte, serverNonce []byte) []byte {
	hash := sha256.New()
	hash.Write([]byte("dsp-session-v1"))
	hash.Write(shared)
	hash.Write(clientNonce)
	hash.Write(serverNonce)
	return hash.Sum(nil)
}
