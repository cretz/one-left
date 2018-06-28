package game

import (
	"github.com/cretz/one-left/oneleft/host/client"
	"github.com/cretz/one-left/oneleft/pb"
)

type PlayerInfo struct {
	Client   client.Client
	Identity *pb.PlayerIdentity
}
