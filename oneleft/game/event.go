package game

type EventType int

const (
	EventGameStart EventType = iota
	EventGameEnd
	EventHandStartShuffled
	EventHandStartCardDealt
	// This can happen multiple times
	EventHandStartTopCardAddedToDiscard
	EventHandReshuffled
	EventHandPlayerSkipped
	EventHandPlayerDrewTwo
	EventHandPlayReversed
	EventHandPlayerDrewOne
	EventHandPlayerPlayedNothing
	EventHandPlayerDiscarded
	EventHandPlayerNoChallengeDrewFour
	EventHandPlayerChallengeSuccessDrewFour
	EventHandPlayerChallengeFailedDrewSix
	EventHandOneLeftCalled
	EventHandPlayerOneLeftPenaltyDrewTwo
	EventHandEnd
)

type Event struct {
	Type         EventType
	PlayerScores []int
	DealerIndex  int
	Hand         *EventHand
	HandComplete *HandComplete
}

type EventHand struct {
	PlayerIndex          int
	PlayerCardsRemaining []int
	DeckCardsRemaining   int
	DiscardStack         []Card
	LastDiscardWildColor int
	Forward              bool
	OneLeftTarget        int
}
