package host

import "github.com/cretz/one-left/oneleft/pb"

type Player struct {
	*pb.Player
	clientNum int64
}
