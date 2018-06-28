package player

import (
	"context"

	"github.com/cretz/one-left/oneleft/pb"
)

func (p *handler) OnGameEvent(ctx context.Context, v *pb.HostMessage_GameEvent) error {
	// TODO: validate every event in the context of the game and determine accuracy
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if event, err := convertGameEvent(v); err != nil {
		return err
	} else {
		p.dataLock.Lock()
		p.lastEvent = event
		p.dataLock.Unlock()
		return p.ui.GameEvent(ctx, event)
	}
}
