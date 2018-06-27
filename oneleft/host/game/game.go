package game

import (
	"fmt"
	"sync"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type Game struct {
	players []*clientPlayer

	dataLock  sync.RWMutex
	deck      *deck
	running   bool
	lastEvent *pb.HostMessage_GameEvent
}

func New(players []*PlayerInfo) *Game {
	ret := &Game{players: make([]*clientPlayer, len(players))}
	for index, playerInfo := range players {
		ret.players[index] = &clientPlayer{PlayerInfo: playerInfo, index: index, currGame: ret}
	}
	return ret
}

func (g *Game) Play() error {
	// Mark as running (don't unmark when done)
	g.dataLock.Lock()
	if g.running {
		g.dataLock.Unlock()
		return fmt.Errorf("Already running or already ran")
	}
	g.dataLock.Unlock()
	// Run the game
	gamePlayers := make([]game.Player, len(g.players))
	for i, p := range g.players {
		gamePlayers[i] = p
	}
	gameComplete, gameError := game.New(gamePlayers, g.newDeck, g.onEvent).Play(0)
	if gameError != nil {
		return gameError
	}
	panic(fmt.Errorf("TODO: %v", gameComplete))
}

func (g *Game) topDiscardColor() (game.CardColor, error) {
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

func (g *Game) onEvent(event *game.Event) error {
	pbEvent := gameEventToPbEvent(event)
	g.dataLock.Lock()
	g.lastEvent = pbEvent
	g.dataLock.Unlock()
	// TODO: I know we have things to do like handling game and hand start
	return nil
}

func (g *Game) newDeck() (game.CardDeck, error) {
	panic("TODO")
}

func gameEventToPbEvent(event *game.Event) *pb.HostMessage_GameEvent {
	panic("TODO")
}
