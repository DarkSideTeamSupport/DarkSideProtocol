package secureproto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

const (
	TypeChallenge = "challenge_v2"
	TypeAuth      = "auth_v2"
	TypeReady     = "ready_v2"
)

type ChallengeFrame struct {
	Type         string `json:"type"`
	ProtoVersion string `json:"proto_version"`
	ServerNonce  string `json:"server_nonce"`
	Ticket       string `json:"ticket"`
}

type AuthFrame struct {
	Type         string `json:"type"`
	ProtoVersion string `json:"proto_version"`
	Proof        string `json:"proof"`
}

type ReadyFrame struct {
	Type         string `json:"type"`
	ProtoVersion string `json:"proto_version"`
	SessionID    string `json:"session_id,omitempty"`
	Mac          string `json:"mac"`
}

func BuildChallenge(sessionKey []byte, serverNonce []byte) ChallengeFrame {
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte("challenge-v2"))
	mac.Write(serverNonce)
	return ChallengeFrame{
		Type:         TypeChallenge,
		ProtoVersion: "v2",
		ServerNonce:  base64.StdEncoding.EncodeToString(serverNonce),
		Ticket:       base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}
}

func ParseChallenge(raw []byte) (ChallengeFrame, []byte, error) {
	var c ChallengeFrame
	if err := json.Unmarshal(raw, &c); err != nil {
		return ChallengeFrame{}, nil, err
	}
	if c.Type != TypeChallenge || c.ProtoVersion != "v2" {
		return ChallengeFrame{}, nil, fmt.Errorf("invalid challenge")
	}
	nonce, err := base64.StdEncoding.DecodeString(c.ServerNonce)
	if err != nil || len(nonce) != 16 {
		return ChallengeFrame{}, nil, fmt.Errorf("invalid challenge nonce")
	}
	return c, nonce, nil
}

func BuildAuthProof(sessionKey []byte, ticket string) string {
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte("auth-v2"))
	mac.Write([]byte(ticket))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyAuthProof(sessionKey []byte, ticket string, proof string) bool {
	want := BuildAuthProof(sessionKey, ticket)
	return hmac.Equal([]byte(want), []byte(proof))
}

func BuildReady(sessionKey []byte, sessionID string) ReadyFrame {
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte("ready-v2"))
	mac.Write([]byte(sessionID))
	return ReadyFrame{
		Type:         TypeReady,
		ProtoVersion: "v2",
		SessionID:    sessionID,
		Mac:          base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}
}

func VerifyReady(sessionKey []byte, frame ReadyFrame) bool {
	if frame.Type != TypeReady || frame.ProtoVersion != "v2" {
		return false
	}
	expectedMAC, err := base64.StdEncoding.DecodeString(frame.Mac)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte("ready-v2"))
	mac.Write([]byte(frame.SessionID))
	want := mac.Sum(nil)
	return hmac.Equal(want, expectedMAC)
}
