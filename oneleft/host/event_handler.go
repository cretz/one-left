package host

import "github.com/cretz/one-left/oneleft/pb"

type eventHandler struct {
	*Host
}

func (h *eventHandler) OnEvent(event *pb.HostMessage_GameEvent) error {
	// Set as last event
	h.lock.Lock()
	h.lastGameEvent = event
	h.lock.Unlock()
	// Just send it to all clients
	msg := &pb.HostMessage{Message: &pb.HostMessage_GameEvent_{GameEvent: event}}
	h.lock.RLock()
	defer h.lock.RUnlock()
	for _, client := range h.clients {
		client.Client.SendNonBlocking(msg)
	}
	return nil
}
