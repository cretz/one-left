package game

type hand struct {
	game          *Game
	deck          CardDeck
	playerIndex   int
	discard       []Card
	lastWildColor int
	forward       bool
}

type HandComplete struct {
	WinnerIndex int
	Score       int
	DeckReveal  CardDeckHandCompleteReveal
}

func (h *hand) play() (*HandComplete, *GameError) {
	if err := h.shuffleAndDeal(); err != nil {
		return nil, err
	}
	if err := h.createDiscardWithFirstCard(); err != nil {
		return nil, err
	}
	// Main game loop
	for {
		// Do play
		play, err := h.currentPlayer().Play()
		// Draw if necessary
		if err == nil && play.Card == NoCard {
			if err := h.draw(1); err != nil {
				return nil, err
			}
			h.sendEvent(EventHandPlayerDrewOne)
			// Let the player try again to play it
			play, err = h.currentPlayer().Play()
		}
		if err != nil {
			return nil, h.playerErrorf("Failure to play: %v", err)
		} else if err = play.AssertValid(); err != nil {
			return nil, h.playerErrorf("Invalid play: %v", err)
		} else if play.Card == NoCard {
			h.sendEvent(EventHandPlayerPlayedNothing)
		} else if !play.Card.CanPlayOn(h.topCard(), h.lastWildColor) {
			return nil, h.playerErrorf("Invalid card, tried to play %v on %v", play.Card, h.topCard())
		} else {
			// Otherwise, handle discard
			h.discard = append(h.discard, play.Card)
			h.lastWildColor = play.WildColor
			h.sendEvent(EventHandPlayerDiscarded)
			// Handle play
			switch play.Card.Value() {
			case Skip:
				h.moveNextPlayer()
				h.sendEvent(EventHandPlayerSkipped)
			case Reverse:
				h.forward = !h.forward
				h.sendEvent(EventHandPlayReversed)
			case DrawTwo:
				h.moveNextPlayer()
				if err := h.draw(2); err != nil {
					return nil, err
				}
				h.sendEvent(EventHandPlayerDrewTwo)
			case WildDrawFour:
				// Before moving, we need to see if they want to challenge
				if challenge, err := h.peekNextPlayer().ShouldChallengeWildDrawFour(); err != nil {
					h.moveNextPlayer()
					return nil, h.playerErrorf("Failed checking draw four challenge: %v", err)
				} else if !challenge {
					h.moveNextPlayer()
					if err := h.draw(4); err != nil {
						return nil, err
					}
					h.sendEvent(EventHandPlayerNoChallengeDrewFour)
				} else if success, err := h.currentPlayer().ChallengedWildDrawFour(h.peekNextPlayer()); err != nil {
					return nil, h.playerErrorf("Failure during challenge: %v", err)
				} else if success {
					if err := h.draw(4); err != nil {
						return nil, err
					}
					h.sendEvent(EventHandPlayerChallengeSuccessDrewFour)
				} else {
					h.moveNextPlayer()
					if err := h.draw(6); err != nil {
						return nil, err
					}
					h.sendEvent(EventHandPlayerChallengeFailedDrewSix)
				}
			}
		}
		h.moveNextPlayer()
		// Do victory check
		if complete, err := h.checkComplete(); complete != nil || err != nil {
			return complete, err
		}
	}
}

func (h *hand) shuffleAndDeal() *GameError {
	// Shuffle full deck
	if err := h.deck.Shuffle(nil); err != nil {
		return h.errorf("Failed shuffling: %v", err)
	}
	h.sendEvent(EventHandStartShuffled)
	// Deal to all players
	for i := 0; i < 7; i++ {
		for j := 0; j < len(h.game.players); j++ {
			h.moveNextPlayer()
			if err := h.draw(1); err != nil {
				return err
			}
			h.sendEvent(EventHandStartCardDealt)
		}
	}
	return nil
}

func (h *hand) createDiscardWithFirstCard() *GameError {
	// Put top card of deck on discard
	for {
		topCard, err := h.deck.PopForFirstDiscard()
		if err != nil {
			return h.errorf("Unable to put top deck card on discard pile: %v", err)
		}
		h.discard = append(h.discard, topCard)
		// Action cards have effects at the beginning
		switch v := topCard.Value(); v {
		case Skip:
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
			h.sendEvent(EventHandPlayerSkipped)
			h.moveNextPlayer()
		case DrawTwo:
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
			if err := h.draw(2); err != nil {
				return err
			}
			h.sendEvent(EventHandPlayerDrewTwo)
			h.moveNextPlayer()
		case Reverse:
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
			h.forward = !h.forward
			h.sendEvent(EventHandPlayReversed)
		case Wild:
			// Wild means first player gets to choose
			if h.lastWildColor, err = h.currentPlayer().ChooseColorSinceFirstFirstCardIsWild(); err != nil {
				return h.playerErrorf("Failure to get color for first wild from %v: %v", err)
			}
			// Do this after the color is selected
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
		case WildDrawFour:
			// Can't be wild draw four
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
			continue
		}
		return nil
	}
}

func (h *hand) peekNextPlayer() Player {
	if h.forward {
		if h.playerIndex == len(h.game.players)-1 {
			return h.game.players[0]
		}
		return h.game.players[h.playerIndex+1]
	}
	if h.playerIndex == 0 {
		return h.game.players[len(h.game.players)-1]
	}
	return h.game.players[h.playerIndex-1]
}

func (h *hand) moveNextPlayer() {
	if h.forward {
		if h.playerIndex == len(h.game.players)-1 {
			h.playerIndex = 0
		} else {
			h.playerIndex++
		}
	} else {
		if h.playerIndex == 0 {
			h.playerIndex = len(h.game.players) - 1
		} else {
			h.playerIndex--
		}
	}
}

func (h *hand) currentPlayer() Player {
	return h.game.players[h.playerIndex]
}

func (h *hand) draw(amount int) *GameError {
	for i := 0; i < amount; i++ {
		// If the deck is empty, we have to take the last discard, make that the only discard, and re-shuffle
		if h.deck.CardsRemaining() == 0 {
			// TODO: what if all the players have all the cards?
			if err := h.deck.Shuffle(h.discard[:len(h.discard)-1]); err != nil {
				return h.errorf("Failed shuffling: %v", err)
			}
			h.discard = []Card{h.discard[0]}
			h.sendEvent(EventHandReshuffled)
		}
		if err := h.deck.DealTo(h.currentPlayer()); err != nil {
			return h.playerErrorf("Failed dealing: %v", err)
		}
	}
	return nil
}

func (h *hand) topCard() Card {
	return h.discard[len(h.discard)-1]
}

// if last param is err, it is cause
func (h *hand) playerErrorf(format string, args ...interface{}) *GameError {
	err := h.errorf(format, args...)
	err.Player = h.currentPlayer()
	return err
}

// if last param is err, it is cause
func (h *hand) errorf(format string, args ...interface{}) *GameError {
	return h.game.errorf(format, args...)
}

func (h *hand) checkComplete() (*HandComplete, *GameError) {
	var complete *HandComplete
	for index, player := range h.game.players {
		if player.CardsRemaining() == 0 {
			complete = &HandComplete{WinnerIndex: index}
			break
		}
	}
	if complete == nil {
		return nil, nil
	}
	var err error
	if complete.DeckReveal, err = h.deck.CompleteHand(h.game.players); err != nil {
		return nil, h.errorf("Failed revealing deck: %v", err)
	}
	for _, cards := range complete.DeckReveal.PlayerCards() {
		for _, card := range cards {
			complete.Score += card.Score()
		}
	}
	return complete, nil
}

func (h *hand) sendEvent(typ EventType) {
	if h.game.eventChan == nil {
		return
	}
	h.game.sendEvent(typ, h.eventState(), nil)
}

func (h *hand) eventState() *EventHand {
	hand := &EventHand{
		PlayerIndex:          h.playerIndex,
		PlayerCardsRemaining: make([]int, len(h.game.players)),
		DeckCardsRemaining:   h.deck.CardsRemaining(),
		DiscardStack:         make([]Card, len(h.discard)),
		LastDiscardWildColor: h.lastWildColor,
		Forward:              h.forward,
	}
	for i, player := range h.game.players {
		hand.PlayerCardsRemaining[i] = player.CardsRemaining()
	}
	copy(hand.DiscardStack, h.discard)
	return hand
}
