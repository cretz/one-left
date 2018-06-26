package host

import (
	"fmt"
	"sync"

	"github.com/cretz/one-left/oneleft/pb"
)

type Host struct {
	rwMutex       sync.RWMutex
	clientCounter int64
	clients       map[int64]*Client
	// Never mutated, always replaced
	players []*Player
	// Never mutated, always replaced
	gameUpdate *pb.HostMessage_GameUpdate
	// Only so many kept. Never mutated, always replaced
	chatMessages []*pb.ChatMessage
}

// TODO config
const maxPlayers = 5

func New() *Host {
	return &Host{clients: make(map[int64]*Client)}
}

func (h *Host) GameUpdate() *pb.HostMessage_GameUpdate {
	h.rwMutex.RLock()
	defer h.rwMutex.RUnlock()
	return h.gameUpdate
}

func (h *Host) ProtoPlayers() []*pb.Player {
	h.rwMutex.RLock()
	defer h.rwMutex.RUnlock()
	ret := make([]*pb.Player, len(h.players))
	for i, player := range h.players {
		ret[i] = player.Player
	}
	return ret
}

func (h *Host) ChatMessages() []*pb.ChatMessage {
	h.rwMutex.RLock()
	defer h.rwMutex.RUnlock()
	return h.chatMessages
}

func (h *Host) createAndAddClient() *Client {
	h.rwMutex.Lock()
	defer h.rwMutex.Unlock()
	h.clientCounter++
	client := newClient(h.clientCounter)
	h.clients[client.Num] = client
	return client
}

func (h *Host) removeAndCloseClient(client *Client) {
	h.rwMutex.Lock()
	defer h.rwMutex.Unlock()
	delete(h.clients, client.Num)
	client.Close()
}

func (h *Host) join(client *Client, pbPlayer *pb.Player) error {
	h.rwMutex.Lock()
	defer h.rwMutex.Unlock()
	if h.gameUpdate != nil {
		return fmt.Errorf("Game running")
	} else if client.playerIndex == -1 {
		return fmt.Errorf("Already a player")
	} else if len(h.players) >= maxPlayers {
		return fmt.Errorf("Max players reached")
	}
	player := &Player{Player: pbPlayer, clientNum: client.Num}
	h.players = append(h.players, player)
	client.playerIndex = len(h.players) - 1
	go h.sendPlayersUpdate(h.players)
	return nil
}

func (h *Host) sendPlayersUpdate(players []*Player) {
	msg := &pb.HostMessage_PlayersUpdate_{&pb.HostMessage_PlayersUpdate{}}
	for _, player := range players {
		msg.PlayersUpdate.Players = append(msg.PlayersUpdate.Players, player.Player)
	}
	hostMsg := &pb.HostMessage{Message: msg}
	h.rwMutex.RLock()
	defer h.rwMutex.RUnlock()
	for _, client := range h.clients {
		client.sendNonBlocking(hostMsg)
	}
}
