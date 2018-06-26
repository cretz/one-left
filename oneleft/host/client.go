package host

import (
	"sync"

	"github.com/cretz/one-left/oneleft/pb"
)

type Client struct {
	Num         int64
	playerLock  sync.RWMutex
	playerIndex int
	playerID    []byte

	sendCh           chan *pb.HostMessage
	terminatingErrCh chan error

	reqRespLock       sync.Mutex
	receivedRespValCh chan<- *pb.ClientMessage_PlayerResponse
	receivedRespErrCh chan<- error
}

func newClient(num int64) *Client {
	ret := &Client{
		Num:              num,
		playerIndex:      -1,
		sendCh:           make(chan *pb.HostMessage, 1),
		terminatingErrCh: make(chan error, 1),
	}
	// Just a quick check to make sure the pb iface is properly impld
	var _ pb.PlayerServer = ret
	return ret
}

func (c *Client) sendNonBlocking(msg *pb.HostMessage) {
	go func() { c.sendCh <- msg }()
}

func (c *Client) PlayerIndex() int {
	c.playerLock.RLock()
	defer c.playerLock.RUnlock()
	return c.playerIndex
}

func (c *Client) PlayerID() []byte {
	c.playerLock.RLock()
	defer c.playerLock.RUnlock()
	return c.playerID
}
