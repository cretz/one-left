package player

import (
	"github.com/cretz/bine/torutil/ed25519"
	"github.com/cretz/one-left/oneleft/player/client"
	"github.com/golang/protobuf/proto"
)

type player struct {
	client  client.Client
	keyPair ed25519.KeyPair
	name    string
}

func (p *player) sign(contents []byte) []byte {
	return ed25519.Sign(p.keyPair, contents)
}

func (p *player) signProto(msg proto.Message) ([]byte, error) {
	if byts, err := proto.Marshal(msg); err != nil {
		return nil, err
	} else {
		return p.sign(byts), nil
	}
}
