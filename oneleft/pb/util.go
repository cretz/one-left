package pb

import (
	"fmt"

	"github.com/cretz/bine/torutil/ed25519"
	"github.com/golang/protobuf/proto"
)

func (p *PlayerIdentity) VerifyIdentity() bool {
	// Clone the identity, remove the sig, make the bytes, verify the sig
	cloned := proto.Clone(p).(*PlayerIdentity)
	cloned.Sig = nil
	clonedBytes, err := proto.Marshal(cloned)
	if err != nil {
		panic(fmt.Errorf("Failed cloning: %v", err))
	}
	return p.VerifySig(clonedBytes, p.Sig)
}

func (p *PlayerIdentity) VerifySig(contents []byte, sig []byte) bool {
	// The ID is an ed25519 pub key
	return ed25519.PublicKey(p.Id).Verify(contents, sig)
}

func (c *ChatMessage) Verify() bool {
	// Clone, remove some stuff, validate with ID
	cloned := proto.Clone(c).(*ChatMessage)
	cloned.Sig = nil
	cloned.HostUtcMs = 0
	clonedBytes, err := proto.Marshal(cloned)
	if err != nil {
		panic(fmt.Errorf("Failed cloning: %v", err))
	}
	return ed25519.PublicKey(c.PlayerId).Verify(clonedBytes, c.Sig)
}
