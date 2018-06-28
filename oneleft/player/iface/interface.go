package iface

import (
	"context"
	"time"

	"github.com/cretz/bine/torutil/ed25519"
	"github.com/cretz/one-left/oneleft/game"
	"github.com/google/uuid"
)

type Interface interface {
	Connected(ctx context.Context, players []*Player, chatMessages []*ChatMessage, lastEvent *GameEvent) error
	PlayersUpdated(context.Context, []*Player) error
	ChatMessage(context.Context, *ChatMessage) error
	GameEvent(context.Context, *GameEvent) error
	Error(context.Context, *Error) error

	GameStart(ctx context.Context, id uuid.UUID, players []*Player) error
	GameEnd(ctx context.Context, scores []int) error
	HandStart(ctx context.Context, dealerIndex int) error
	HandEnd(
		ctx context.Context, winnerIndex int, winnerScore int, deckCards []game.Card, playerCards [][]game.Card,
	) error
	ChooseColorSinceFirstCardIsWild(context.Context) (game.CardColor, error)
	ReceiveCard(ctx context.Context, card game.Card) error
	Play(ctx context.Context) (card game.Card, wildColor game.CardColor, err error)
	ShouldChallengeWildDrawFour() error
}

type Player struct {
	ID   ed25519.PublicKey
	Name string
}

type ChatMessage struct {
	Player   Player
	Contents string
	Time     time.Time
}

type GameEvent struct {
	GameID       uuid.UUID
	Type         game.EventType
	PlayerScores []int
	DealerIndex  int
	Hand         *GameEventHand
	HandComplete *GameEventHandComplete
}

type GameEventHand struct {
	HandID               uuid.UUID
	PlayerIndex          int
	PlayerCardsRemaining []int
	DeckCardsRemaining   int
	DiscardStack         []game.Card
	LastDiscardWildColor game.CardColor
	Forward              bool
	OneLeftTarget        int
}

type GameEventHandComplete struct {
	WinnerIndex int
	Score       int
	DeckCards   []game.Card
	PlayerCards [][]game.Card
}

type Error struct {
	GameID         uuid.UUID
	Message        string
	PlayerIndex    int
	TerminatesGame bool
}
