package host

import (
	"sync"

	"github.com/cretz/one-left/oneleft/pb"
)

type Client struct {
	reqRespLock       sync.Mutex
	sendReqCh         chan *pb.HostMessage_PlayerRequest
	receivedRespValCh chan<- *pb.ClientMessage_PlayerResponse
	receivedRespErrCh chan<- error
}
