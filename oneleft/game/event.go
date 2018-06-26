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

var eventTypeNames = map[EventType]string{
	EventGameStart:                          "GameStart",
	EventGameEnd:                            "GameEnd",
	EventHandStartShuffled:                  "HandStartShuffled",
	EventHandStartCardDealt:                 "HandStartCardDealt",
	EventHandStartTopCardAddedToDiscard:     "HandStartTopCardAddedToDiscard",
	EventHandReshuffled:                     "HandReshuffled",
	EventHandPlayerSkipped:                  "HandPlayerSkipped",
	EventHandPlayerDrewTwo:                  "HandPlayerDrewTwo",
	EventHandPlayReversed:                   "HandPlayReversed",
	EventHandPlayerDrewOne:                  "HandPlayerDrewOne",
	EventHandPlayerPlayedNothing:            "HandPlayerPlayedNothing",
	EventHandPlayerDiscarded:                "HandPlayerDiscarded",
	EventHandPlayerNoChallengeDrewFour:      "HandPlayerNoChallengeDrewFour",
	EventHandPlayerChallengeSuccessDrewFour: "PlayerChallengeSuccessDrewFour",
	EventHandPlayerChallengeFailedDrewSix:   "PlayerChallengeFailedDrewSix",
	EventHandOneLeftCalled:                  "HandOneLeftCalled",
	EventHandPlayerOneLeftPenaltyDrewTwo:    "HandPlayerOneLeftPenaltyDrewTwo",
	EventHandEnd:                            "HandEnd",
}

func (e EventType) String() string { return eventTypeNames[e] }

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
	LastDiscardWildColor CardColor
	Forward              bool
	OneLeftTarget        int
}
