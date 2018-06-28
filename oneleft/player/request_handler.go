package player

import (
	"context"
	"time"

	"github.com/cretz/one-left/oneleft/pb"
	"github.com/cretz/one-left/oneleft/player/client"
	"github.com/cretz/one-left/oneleft/player/iface"
)

type requestHandler struct {
	player *player
	ui     iface.Interface
}

// TODO: config
const maxIfaceHandleTime = 1 * time.Minute

func (p *requestHandler) OnRun(c client.Client) {
	// Do nothing
}

func (p *requestHandler) OnWelcome(c client.Client, v *pb.HostMessage_Welcome) {
	players, err := convertPlayers(v.Players)
	if err != nil {
		c.FailNonBlocking(err)
		return
	}
	chatMessages, err := convertChatMessages(v.ChatMessages)
	if err != nil {
		c.FailNonBlocking(err)
		return
	}
	lastEvent, err := convertGameEvent(v.LastGameEvent)
	if err != nil {
		c.FailNonBlocking(err)
		return
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), maxIfaceHandleTime)
	defer cancelFn()
	if err := p.ui.Connected(ctx, players, chatMessages, lastEvent); err != nil {
		c.FailNonBlocking(err)
	}
}

func (p *requestHandler) OnPlayersUpdate(c client.Client, v *pb.HostMessage_Players) {
	panic("TODO")
}

func (p *requestHandler) OnChatMessage(c client.Client, v *pb.ChatMessage) {
	panic("TODO")
}

func (p *requestHandler) OnGameEvent(c client.Client, v *pb.HostMessage_GameEvent) {
	panic("TODO")
}

func (p *requestHandler) OnError(c client.Client, v *pb.HostMessage_Error) {
	panic("TODO")
}
