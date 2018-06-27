package game

import "fmt"

type Game struct {
	players []Player
	newDeck func() (CardDeck, error)
	eventCb func(*Event) error

	dealerIndex  int
	hand         *hand
	playerScores []int
}

type GameError struct {
	Message     string
	Cause       error
	PlayerIndex int
}

func (h *GameError) Error() string { return h.Message }

type GameComplete struct {
	PlayerScores []int
}

func New(players []Player, newDeck func() (CardDeck, error), eventCb func(*Event) error) *Game {
	return &Game{players: players, newDeck: newDeck, eventCb: eventCb}
}

func (g *Game) Play(initialDealerIndex int) (*GameComplete, *GameError) {
	g.dealerIndex = initialDealerIndex
	g.playerScores = make([]int, len(g.players))
	if err := g.sendEvent(EventGameStart, nil, nil); err != nil {
		return nil, err
	}
	// Play until someone gets 500
	for {
		// Create the hand
		deck, err := g.newDeck()
		if err != nil {
			return nil, Errorf("Failed creating deck: %v", err)
		}
		hand := &hand{
			game:        g,
			deck:        deck,
			playerIndex: g.dealerIndex,
			forward:     true,
		}
		// Play it
		handComplete, gameErr := hand.play()
		if gameErr != nil {
			return nil, gameErr
		}
		// Add the score to the winning player and check if 500 reached
		g.playerScores[handComplete.WinnerIndex] += handComplete.Score
		g.sendEvent(EventHandEnd, hand.eventState(), handComplete)
		if g.playerScores[handComplete.WinnerIndex] >= 500 {
			break
		}
		// Next player becomes dealer
		g.dealerIndex++
		if g.dealerIndex == len(g.players) {
			g.dealerIndex = 0
		}
	}
	if err := g.sendEvent(EventGameEnd, nil, nil); err != nil {
		return nil, err
	}
	return &GameComplete{PlayerScores: g.playerScores}, nil
}

// if last param is err, it is cause
func Errorf(format string, args ...interface{}) *GameError {
	ret := &GameError{Message: fmt.Sprintf(format, args...)}
	if len(args) > 0 {
		if cause, ok := args[len(args)-1].(error); ok {
			ret.Cause = cause
		}
	}
	return ret
}

func PlayerErrorf(playerIndex int, format string, args ...interface{}) *GameError {
	err := Errorf(format, args...)
	err.PlayerIndex = playerIndex
	return err
}

func (g *Game) sendEvent(typ EventType, hand *EventHand, handComplete *HandComplete) *GameError {
	if g.eventCb == nil {
		return nil
	}
	event := &Event{
		Type:         typ,
		PlayerScores: make([]int, len(g.playerScores)),
		DealerIndex:  g.dealerIndex,
		Hand:         hand,
		HandComplete: handComplete,
	}
	copy(event.PlayerScores, g.playerScores)
	if err := g.eventCb(event); err != nil {
		return Errorf("Failed sending event %v: %v", typ, err)
	}
	return nil
}
