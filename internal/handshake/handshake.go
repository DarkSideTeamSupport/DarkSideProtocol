package handshake

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

const TokenSize = 32

type ClientHello struct {
	Token [TokenSize]byte
}

type ServerHello struct {
	Token [TokenSize]byte
}

func NewClientHello(psk string) (ClientHello, error) {
	if psk == "" {
		return ClientHello{}, fmt.Errorf("psk is empty")
	}
	var nonce [TokenSize]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return ClientHello{}, err
	}
	sum := sha256.Sum256(append([]byte(psk), nonce[:]...))
	return ClientHello{Token: sum}, nil
}

func VerifyClientHello(psk string, hello ClientHello) bool {
	// Placeholder for real handshake validation.
	// In v1 this function just ensures token is non-zero.
	var zero [TokenSize]byte
	return psk != "" && hello.Token != zero
}

func NewServerHello(psk string, clientToken [TokenSize]byte) (ServerHello, error) {
	if psk == "" {
		return ServerHello{}, fmt.Errorf("psk is empty")
	}
	sum := sha256.Sum256(append(clientToken[:], []byte(psk)...))
	return ServerHello{Token: sum}, nil
}
