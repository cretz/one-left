package gametest

import (
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/cretz/one-left/oneleft/game"
)

func BenchmarkSomeGames(b *testing.B) {
	for i := 0; i < b.N; i++ {
		seed := time.Now().UnixNano()
		rand.Seed(seed)
		if err := runGame(5); err != nil {
			b.Fatalf("Failure with seed %v: %v", seed, err)
		}
	}
}

func TestGame(t *testing.T) {
	// rand.Seed(1529995356611101700)
	if err := runGame(5); err != nil {
		t.Fatal(err)
	}
}

func runGame(playerCount int) error {
	// Build deck and players
	players := make([]game.Player, playerCount)
	for i := 0; i < len(players); i++ {
		players[i] = &PracticalPlayer{Index: i, AllPlayers: players}
	}
	newDeck := func() (game.CardDeck, error) {
		handState := &HandState{}
		for _, player := range players {
			player.(*PracticalPlayer).HandState = handState
		}
		return &SimpleDeck{HandState: handState}, nil
	}
	// Log events
	logEventCb := func(event *game.Event) error {
		debugf("Event: %v - Hand: %v", event, event.Hand)
		return nil
	}
	// Begin
	debugf("------- New game -------")
	gameComplete, gameError := game.New(players, newDeck, logEventCb).Play(0)
	if gameError != nil {
		debugf("ERR: %v", gameError)
		return gameError
	}
	debugf("END: %v", gameComplete)
	return nil
}

const debug = false

func debugf(format string, args ...interface{}) {
	if debug {
		log.Printf(format, args...)
	}
}
