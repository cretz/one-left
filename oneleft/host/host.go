package host

import (
	"sync"
	"time"

	"github.com/cretz/one-left/oneleft/host/client"
	"github.com/cretz/one-left/oneleft/host/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type Host struct {
	lock sync.RWMutex
	// Map can be added or deleted from, but val is never mutated, always replaced
	clients map[uint64]*game.PlayerInfo
	// Never mutated, always replaced
	protoPlayers []*pb.PlayerIdentity
	gamePlayers  []*game.PlayerInfo
	// Never mutated, always replaced
	chatMessages []*pb.ChatMessage
	// Never mutated, always replaced
	lastGameEvent *pb.HostMessage_GameEvent
	gameRunning   bool
}

const maxClientRPCWait = 1 * time.Minute

func (h *Host) Stream(stream pb.Host_StreamServer) error {
	// Just run the client
	return client.New(&requestHandler{h}, stream, maxClientRPCWait).Run()
}

func (h *Host) PlayGame() error {
	h.lock.RLock()
	g := game.New(h.gamePlayers)
	h.lock.RUnlock()
	return g.Play()
}
