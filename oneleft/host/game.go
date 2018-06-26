package host

import (
	"fmt"
	"sync"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type Game struct {
	players []*ClientPlayer
	host    *Host

	mutex     sync.Mutex
	deck      *Deck
	lastEvent *pb.HostMessage_GameEvent
}

func (h *Host) PlayGame() error {
	// Mark the game as running and grab a couple of things
	h.mutex.Lock()
	playerClients := h.playerClients
	h.gameRunning = true
	h.lastGameEvent = nil
	h.mutex.Unlock()
	// Mark the game as no longer running when we're done
	defer func() {
		h.mutex.Lock()
		h.gameRunning = false
		h.lastGameEvent = nil
		h.mutex.Unlock()
	}()
	// Create the game obj
	g := &Game{players: make([]*ClientPlayer, len(playerClients))}
	gamePlayers := make([]game.Player, len(playerClients))
	for i, player := range playerClients {
		if player.playerIndex != i {
			return fmt.Errorf("Unexpectedly invalid index")
		}
		clientPlayer := &ClientPlayer{
			client:   player,
			index:    i,
			currGame: g,
		}
		g.players[i] = clientPlayer
		gamePlayers[i] = clientPlayer
	}
	// Run the game
	gameComplete, gameError := game.New(gamePlayers, g.newDeck, g.onEvent).Play(0)
	if gameError != nil {
		return gameError
	}
	panic(fmt.Errorf("TODO: %v", gameComplete))
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

func (g *Game) onEvent(event *game.Event) error {
	pbEvent := gameEventToPbEvent(event)
	g.mutex.Lock()
	g.lastEvent = pbEvent
	g.mutex.Unlock()
	g.host.mutex.Lock()
	g.host.lastGameEvent = pbEvent
	g.host.mutex.Unlock()
	// TODO: I know we have things to do like handling game and hand start
	return nil
}

func (g *Game) newDeck() (game.CardDeck, error) {
	panic("TODO")
}

func gameEventToPbEvent(event *game.Event) *pb.HostMessage_GameEvent {
	panic("TODO")
}
