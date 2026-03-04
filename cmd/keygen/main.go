package main

import (
	"encoding/json"
	"fmt"
	"log"

	"darksideprotocol/internal/secureproto"
)

type output struct {
	Server secureproto.KeyPair `json:"server"`
	Client secureproto.KeyPair `json:"client"`
}

func main() {
	serverPair, err := secureproto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("server keygen failed: %v", err)
	}
	clientPair, err := secureproto.GenerateKeyPair()
	if err != nil {
		log.Fatalf("client keygen failed: %v", err)
	}
	out := output{Server: serverPair, Client: clientPair}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}
