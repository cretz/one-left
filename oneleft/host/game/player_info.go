package game

import (
	"fmt"

	"github.com/cretz/bine/torutil/ed25519"
	"github.com/cretz/one-left/oneleft/host/client"
	"github.com/cretz/one-left/oneleft/pb"
	"github.com/golang/protobuf/proto"
)

type PlayerInfo struct {
	Client   *client.Client
	Identity *pb.PlayerIdentity
}

func (p *PlayerInfo) VerifyIdentity() bool {
	// Clone the identity, remove the sig, make the bytes, verify the sig
	cloned := proto.Clone(p.Identity).(*pb.PlayerIdentity)
	cloned.Sig = nil
	clonedBytes, err := proto.Marshal(cloned)
	if err != nil {
		panic(fmt.Errorf("Failed cloning: %v", err))
	}
	return p.VerifySig(clonedBytes, p.Identity.Sig)
}

func (p *PlayerInfo) VerifySig(contents []byte, sig []byte) bool {
	// The ID is an ed25519 pub key
	return ed25519.PublicKey(p.Identity.Id).Verify(contents, sig)
}
