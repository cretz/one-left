package host

import (
	"fmt"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type Game struct {
	players   []*ClientPlayer
	lastEvent *pb.HostMessage_GameEvent
	deck      *Deck
}

func (h *Host) PlayGame() error {
	// Create the game
	panic("TODO")
}

func (g *Game) TopDiscardColor() (game.CardColor, error) {
	if g.lastEvent == nil || g.lastEvent.Hand == nil {
		return 0, fmt.Errorf("No hand")
	}
	topCard := game.Card(g.lastEvent.Hand.DiscardStack[len(g.lastEvent.Hand.DiscardStack)-1])
	color := topCard.Color()
	if topCard.Wild() {
		color = game.CardColor(g.lastEvent.Hand.LastDiscardWildColor)
	}
	return color, nil
}
