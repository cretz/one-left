package player

import (
	"context"
	"time"

	"github.com/cretz/one-left/oneleft/pb"
	"github.com/cretz/one-left/oneleft/player/iface"
)

type handler struct {
	player *player
	ui     iface.Interface
}

// TODO: config
const maxIfaceHandleTime = 1 * time.Minute

func (p *handler) OnRun(ctx context.Context) error {
	// Do nothing
	return nil
}

func (p *handler) OnWelcome(ctx context.Context, v *pb.HostMessage_Welcome) error {
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if players, err := convertPlayers(v.Players); err != nil {
		return err
	} else if chatMessages, err := convertChatMessages(v.ChatMessages); err != nil {
		return err
	} else if lastEvent, err := convertGameEvent(v.LastGameEvent); err != nil {
		return err
	} else {
		return p.ui.Connected(ctx, players, chatMessages, lastEvent)
	}
}

func (p *handler) OnPlayersUpdate(ctx context.Context, v *pb.HostMessage_Players) error {
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if players, err := convertPlayers(v.Players); err != nil {
		return err
	} else {
		return p.ui.PlayersUpdated(ctx, players)
	}
}

func (p *handler) OnChatMessage(ctx context.Context, v *pb.ChatMessage) error {
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if msg, err := convertChatMessage(v); err != nil {
		return err
	} else {
		return p.ui.ChatMessage(ctx, msg)
	}
}

func (p *handler) OnGameEvent(ctx context.Context, v *pb.HostMessage_GameEvent) error {
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if event, err := convertGameEvent(v); err != nil {
		return err
	} else {
		return p.ui.GameEvent(ctx, event)
	}
}

func (p *handler) OnError(ctx context.Context, v *pb.HostMessage_Error) error {
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if e, err := convertError(v); err != nil {
		return err
	} else {
		return p.ui.Error(ctx, e)
	}
}
