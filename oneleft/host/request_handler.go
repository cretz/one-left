package host

import (
	"bytes"
	"fmt"

	"github.com/cretz/one-left/oneleft/host/client"
	"github.com/cretz/one-left/oneleft/host/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type requestHandler struct {
	*Host
}

func (h *requestHandler) OnRun(c *client.Client) {
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

func (h *requestHandler) OnChatMessage(c *client.Client, msg *pb.ChatMessage) {
	// Get player info
	h.lock.RLock()
	info := h.clients[c.Num()]
	h.lock.RUnlock()
	// Eager player ID check
	if info == nil || info.Identity == nil || !bytes.Equal(info.Identity.Id, msg.PlayerId) {
		c.Fail(fmt.Errorf("Only players can chat"))
		return
	}
	// TODO: check sig, do update, etc
	panic("TODO")
}

func (h *requestHandler) OnStartJoin(c *client.Client) {
	panic("TODO")
}

func (h *requestHandler) OnStop(c *client.Client) {
	panic("TODO")
}
