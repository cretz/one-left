package game

import (
	"github.com/cretz/bine/torutil/ed25519"
	"github.com/cretz/one-left/oneleft/host/client"
)

type PlayerInfo struct {
	Client *client.Client
	ID     []byte
}

func (p *PlayerInfo) VerifySig(contents []byte, sig []byte) bool {
	// The ID is an ed25519 pub key
	return ed25519.PublicKey(p.ID).Verify(contents, sig)
}
