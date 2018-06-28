package player

import (
	"github.com/cretz/bine/torutil/ed25519"
	"github.com/cretz/one-left/oneleft/player/client"
)

type player struct {
	client  client.Client
	keyPair ed25519.KeyPair
	name    string
}

func (p *player) sign(contents []byte) []byte {
	return ed25519.Sign(p.keyPair, contents)
}