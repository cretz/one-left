package host

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cretz/bine/torutil/ed25519"
	"github.com/cretz/one-left/oneleft/host/client"
	"github.com/cretz/one-left/oneleft/host/game"
	"github.com/cretz/one-left/oneleft/pb"
	"github.com/golang/protobuf/proto"
)

type requestHandler struct {
	*Host
}

func (h *requestHandler) OnRun(c client.Client) {
	h.lock.Lock()
	defer h.lock.Unlock()
	err := c.SendNonBlocking(&pb.HostMessage{Message: &pb.HostMessage_Welcome_{&pb.HostMessage_Welcome{
		Players:       h.protoPlayers,
		ChatMessages:  h.chatMessages,
		LastGameEvent: h.lastGameEvent,
	}}})
	if err == nil {
		h.clients[c.Num()] = &game.PlayerInfo{Client: c}
	}
	// TODO: warn on welcome error?
}

func utcTimestampMs() uint64 {
	return uint64(time.Now().UnixNano()) / uint64(time.Millisecond)
}

func (h *requestHandler) OnChatMessage(c client.Client, msg *pb.ChatMessage) {
	// Get player info
	h.lock.RLock()
	info := h.clients[c.Num()]
	counter := h.clientChatCounters[c.Num()]
	h.clientChatCounters[c.Num()]++
	h.lock.RUnlock()
	// Validate the message
	if info == nil || info.Identity == nil || !bytes.Equal(info.Identity.Id, msg.PlayerId) {
		c.FailNonBlocking(fmt.Errorf("Only players can chat"))
		return
	} else if info.Identity.Name != msg.PlayerName {
		c.FailNonBlocking(fmt.Errorf("Chat player name mismatch"))
		return
	} else if msg.Counter != counter {
		c.FailNonBlocking(fmt.Errorf("Invalid chat counter"))
		return
	} else if msg.Contents == "" || len(msg.Contents) > maxChatContentLen {
		c.FailNonBlocking(fmt.Errorf("Chat content length invalid"))
		return
	} else if msg.HostUtcMs != 0 {
		c.FailNonBlocking(fmt.Errorf("Host UTC MS should not be set on chat"))
		return
	}
	// Check the sig
	cloned := proto.Clone(msg).(*pb.ChatMessage)
	cloned.Sig = nil
	if clonedBytes, err := proto.Marshal(cloned); err != nil {
		panic(err)
	} else if !info.VerifySig(clonedBytes, msg.Sig) {
		c.FailNonBlocking(fmt.Errorf("Signature failed"))
		return
	}
	msg.HostUtcMs = utcTimestampMs()
	hostMsg := &pb.HostMessage{Message: &pb.HostMessage_ChatMessageAdded{ChatMessageAdded: msg}}
	// Lock for the rest
	h.lock.Lock()
	defer h.lock.Unlock()
	// Send it out to everyone
	for _, client := range h.clients {
		client.Client.SendNonBlocking(hostMsg)
	}
	// And copy-on-write add chat message
	var newChatMessages []*pb.ChatMessage
	if len(h.chatMessages) >= maxChatMessagesKept {
		newChatMessages = make([]*pb.ChatMessage, len(h.chatMessages))
		copy(newChatMessages, h.chatMessages[1:])
	} else {
		newChatMessages = make([]*pb.ChatMessage, len(h.chatMessages)+1)
		copy(newChatMessages, h.chatMessages)
	}
	newChatMessages[len(newChatMessages)-1] = msg
	h.chatMessages = newChatMessages
}

func (h *requestHandler) OnStartJoin(c client.Client) {
	sendErr := func(str string) {
		c.SendNonBlocking(&pb.HostMessage{Message: &pb.HostMessage_Error_{Error: &pb.HostMessage_Error{Message: str}}})
	}
	// Make sure game isn't running
	if h.GameRunning() {
		sendErr("Game is already running")
		return
	}
	// Make sure not max
	h.lock.RLock()
	playerCount := len(h.gamePlayers)
	h.lock.RUnlock()
	if playerCount >= maxPlayers {
		sendErr("Already at max player count")
		return
	}
	// Send off join request
	joinReq := &pb.JoinRequest{RandomNonce: make([]byte, randomNonceSize)}
	if _, err := io.ReadFull(rand.Reader, joinReq.RandomNonce); err != nil {
		sendErr("Internal failure building nonce")
		return
	}
	resp, err := c.Join(context.Background(), joinReq)
	if err != nil {
		// TODO: log?
		return
	}
	// Validate and build info
	info := &game.PlayerInfo{Client: c, Identity: resp.Player}
	if !bytes.Equal(joinReq.RandomNonce, info.Identity.RandomNonce) {
		sendErr("Invalid nonce")
		return
	} else if len(info.Identity.Id) != ed25519.PublicKeySize {
		sendErr("Invalid ID")
		return
	} else if !info.VerifyIdentity() {
		sendErr("Invalid sig")
		return
	} else if info.Identity.Name == "" || len(info.Identity.Name) > maxNameLen {
		sendErr("Invalid name size")
		return
	}
	// Now lock the host then do the rest
	h.lock.Lock()
	defer h.lock.Unlock()
	// Check game not running again
	if h.gameRunning {
		sendErr("Game is already running")
		return
	}
	// Check max again
	if len(h.gamePlayers) >= maxPlayers {
		sendErr("Already at max player count")
		return
	}
	// Check client present
	if info := h.clients[c.Num()]; info == nil || info.Identity != nil {
		sendErr("Client no longer present or already a player")
		return
	}
	// Check name and ID uniqueness
	nameLower := strings.ToLower(info.Identity.Name)
	for _, player := range h.gamePlayers {
		if strings.ToLower(player.Identity.Name) == nameLower {
			sendErr("Name taken")
			return
		}
		if bytes.Equal(player.Identity.Id, info.Identity.Id) {
			sendErr("ID taken")
			return
		}
	}
	// Add the player
	h.clients[c.Num()] = info
	// Proto players slice is copy-on-write
	newProtoPlayers := make([]*pb.PlayerIdentity, len(h.protoPlayers)+1)
	copy(newProtoPlayers, h.protoPlayers)
	newProtoPlayers[len(newProtoPlayers)-1] = info.Identity
	h.protoPlayers = newProtoPlayers
	// Game players slice is copy-on-write
	newGamePlayers := make([]*game.PlayerInfo, len(h.gamePlayers)+1)
	copy(newGamePlayers, h.gamePlayers)
	newGamePlayers[len(newGamePlayers)-1] = info
	h.gamePlayers = newGamePlayers
	// Send off the player updates
	h.sendPlayerUpdatesUnsafe()
}

func (h *requestHandler) OnStop(c client.Client) {
	// We don't care if this is a player and we're in the game, we expect the game will stop somewhere else and nothing
	// as part of the game should use the player sets.
	// Lock for all of this
	h.lock.Lock()
	defer h.lock.Unlock()
	info := h.clients[c.Num()]
	delete(h.clients, c.Num())
	delete(h.clientChatCounters, c.Num())
	if info != nil && info.Identity != nil {
		// Proto players slice is copy-on-write, so filter out the stopped client
		newProtoPlayers := []*pb.PlayerIdentity{}
		newGamePlayers := []*game.PlayerInfo{}
		for _, existingInfo := range h.gamePlayers {
			if !bytes.Equal(existingInfo.Identity.Id, info.Identity.Id) {
				newProtoPlayers = append(newProtoPlayers, info.Identity)
				newGamePlayers = append(newGamePlayers, info)
			}
		}
		h.protoPlayers = newProtoPlayers
		h.gamePlayers = newGamePlayers
	}
}

// Unsafe because it expects callers to lock
func (h *requestHandler) sendPlayerUpdatesUnsafe() {
	msg := &pb.HostMessage{Message: &pb.HostMessage_PlayersUpdate{
		PlayersUpdate: &pb.HostMessage_Players{Players: h.protoPlayers},
	}}
	for _, client := range h.clients {
		client.Client.SendNonBlocking(msg)
	}
}
