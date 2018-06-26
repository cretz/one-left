package host

import "github.com/cretz/one-left/oneleft/pb"

type Host struct {
}

func (h *Host) Stream(stream pb.Host_StreamServer) error {
	panic("TODO")
}
