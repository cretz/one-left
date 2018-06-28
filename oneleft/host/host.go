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
	// Maps can be added or deleted from, but val is never mutated, always replaced
	clients            map[uint64]*game.PlayerInfo
	clientChatCounters map[uint64]uint32
	// Never mutated, always replaced
	protoPlayers []*pb.PlayerIdentity
	// Never mutated, always replaced
	gamePlayers []*game.PlayerInfo
	// Never mutated, always replaced
	chatMessages []*pb.ChatMessage
	// Never mutated, always replaced
	lastGameEvent *pb.HostMessage_GameEvent
	gameRunning   bool
}

// TODO config
const maxClientRPCWait = 1 * time.Minute
const maxPlayers = 10
const maxChatMessagesKept = 50
const randomNonceSize = 10
const maxNameLen = 80
const maxChatContentLen = 500

func New() *Host {
	return &Host{
		clients:            map[uint64]*game.PlayerInfo{},
		clientChatCounters: map[uint64]uint32{},
	}
}

func (h *Host) Stream(stream pb.Host_StreamServer) error {
	// Just run the client
	return client.New(&requestHandler{h}, stream, maxClientRPCWait).Run()
}

func (h *Host) PlayGame() error {
	h.lock.Lock()
	g := game.New(&eventHandler{h}, h.gamePlayers)
	h.gameRunning = true
	h.lock.Unlock()
	defer func() {
		h.lock.Lock()
		defer h.lock.Unlock()
		h.gameRunning = false
	}()
	// TODO: what to do with game complete scores?
	_, err := g.Play()
	if err != nil {
		// If there was an error in the game, tell everyone
		h.sendGameError(g, err)
	}
	return err
}

func (h *Host) GameRunning() bool {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.gameRunning
}

func (h *Host) sendGameError(g *game.Game, err error) {
	pbErr := g.MakePbError(err)
	msg := &pb.HostMessage{Message: &pb.HostMessage_Error_{Error: pbErr}}
	h.lock.RLock()
	defer h.lock.RUnlock()
	// Send it to everyone
	for _, client := range h.clients {
		client.Client.SendNonBlocking(msg)
	}
	// And it if was caused by a player from a certain index, fail the player
	if pbErr.PlayerIndex >= 0 {
		if player := g.Player(int(pbErr.PlayerIndex)); player != nil {
			player.Client.FailNonBlocking(err)
		}
	}
}
