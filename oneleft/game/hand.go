package game

type hand struct {
	game          *Game
	deck          CardDeck
	playerIndex   int
	discard       []Card
	lastWildColor CardColor
	forward       bool
}

type oneLeftCall struct {
	callerIndex int
	targetIndex int
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
	playerIndexJustGotOneLeft := -1
	oneLeftCallbackChan := h.resetOneLeftCallbacks(-1)
	// Main game loop
	for {
		// Do play or one-left call, whichever first
		playCh := make(chan *PlayerPlay, 1)
		errCh := make(chan error, 1)
		go func() {
			if play, err := h.currentPlayer().Play(); err != nil {
				errCh <- err
			} else {
				playCh <- play
			}
		}()
		var play *PlayerPlay
		select {
		case call := <-oneLeftCallbackChan:
			// If it's called on a pending one-left, check it
			if call.targetIndex == playerIndexJustGotOneLeft {
				h.sendOneLeftCalledEvent(EventHandOneLeftCalled, call)
				// If it wasn't the one with one left, it's a penalty
				if call.callerIndex != call.targetIndex {
					if err := h.playerDraw(2, h.game.players[call.targetIndex]); err != nil {
						return nil, err
					}
					h.sendPlayerEvent(EventHandPlayerOneLeftPenaltyDrewTwo, call.targetIndex)
				}
			} else if call.callerIndex == call.targetIndex && h.game.players[call.callerIndex].CardsRemaining() != 1 {
				// It was called for myself out of turn when I didn't have one left
				h.sendOneLeftCalledEvent(EventHandOneLeftCalled, call)
				if err := h.playerDraw(2, h.game.players[call.callerIndex]); err != nil {
					return nil, err
				}
				h.sendPlayerEvent(EventHandPlayerOneLeftPenaltyDrewTwo, call.callerIndex)
			}
		case play = <-playCh:
			// All good, do nothing
		case err := <-errCh:
			return nil, h.playerErrorf("Failure to play: %v", err)
		}
		// Reset one left if there was a player with it
		if playerIndexJustGotOneLeft >= 0 {
			playerIndexJustGotOneLeft = -1
			oneLeftCallbackChan = h.resetOneLeftCallbacks(playerIndexJustGotOneLeft)
		}
		// If play was not set because one-left was called, wait for it
		if play == nil {
			select {
			case play = <-playCh:
				// All good, do nothing
			case err := <-errCh:
				return nil, h.playerErrorf("Failure to play: %v", err)
			}
		}
		// Draw if necessary
		if play.Card == NoCard {
			if err := h.draw(1); err != nil {
				return nil, err
			}
			h.sendEvent(EventHandPlayerDrewOne)
			// Let the player try again to play it
			var err error
			if play, err = h.currentPlayer().Play(); err != nil {
				return nil, h.playerErrorf("Failure to play: %v", err)
			}
		}
		if err := play.AssertValid(); err != nil {
			return nil, h.playerErrorf("Invalid play: %v", err)
		} else if play.Card == NoCard {
			h.sendEvent(EventHandPlayerPlayedNothing)
		} else if !play.Card.CanPlayOn(h.topCard(), h.lastWildColor) {
			return nil, h.playerErrorf("Invalid card, tried to play %v on %v", play.Card, h.topCard())
		} else {
			// Otherwise, handle discard
			h.discard = append(h.discard, play.Card)
			h.lastWildColor = play.WildColor
			// If this player now has one left, set up the opportunity
			if h.currentPlayer().CardsRemaining() == 1 {
				playerIndexJustGotOneLeft = h.playerIndex
				oneLeftCallbackChan = h.resetOneLeftCallbacks(playerIndexJustGotOneLeft)
			}
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
					// If this was a one-left player, we have to undo what we did with that
					if playerIndexJustGotOneLeft >= 0 {
						playerIndexJustGotOneLeft = -1
						oneLeftCallbackChan = h.resetOneLeftCallbacks(playerIndexJustGotOneLeft)
					}
					// Make the current player draw four
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
			if h.lastWildColor, err = h.currentPlayer().ChooseColorSinceFirstCardIsWild(); err != nil {
				return h.playerErrorf("Failure to get color for first wild from %v: %v", err)
			}
			// Do this after the color is selected
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
		case WildDrawFour:
			// Can't be wild draw four
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
			continue
		default:
			h.sendEvent(EventHandStartTopCardAddedToDiscard)
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
	return h.playerDraw(amount, h.currentPlayer())
}

func (h *hand) playerDraw(amount int, player Player) *GameError {
	for i := 0; i < amount; i++ {
		// If the deck is empty, we have to take the last discard, make that the only discard, and re-shuffle
		if h.deck.CardsRemaining() == 0 {
			// TODO: what if all the players have all the cards?
			if err := h.deck.Shuffle(h.discard[:len(h.discard)-1]); err != nil {
				return h.errorf("Failed shuffling: %v", err)
			}
			h.discard = []Card{h.discard[len(h.discard)-1]}
			h.sendEvent(EventHandReshuffled)
		}
		if err := h.deck.DealTo(player); err != nil {
			return h.playerErrorf("Failed dealing: %v", err)
		}
	}
	return nil
}

func (h *hand) topCard() Card {
	return h.discard[len(h.discard)-1]
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

func (h *hand) resetOneLeftCallbacks(hasOneLeftIndex int) chan oneLeftCall {
	ret := make(chan oneLeftCall, len(h.game.players))
	for i, player := range h.game.players {
		playerIndex := i
		player.SetOneLeftCallback(hasOneLeftIndex, func(targetIndex int) {
			ret <- oneLeftCall{callerIndex: playerIndex, targetIndex: targetIndex}
		})
	}
	return ret
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

func (h *hand) sendEvent(typ EventType) {
	if h.game.eventChan == nil {
		return
	}
	h.game.sendEvent(typ, h.eventState(), nil)
}

func (h *hand) sendPlayerEvent(typ EventType, playerIndexOverride int) {
	if h.game.eventChan == nil {
		return
	}
	state := h.eventState()
	state.PlayerIndex = playerIndexOverride
	h.game.sendEvent(typ, state, nil)
}

func (h *hand) sendOneLeftCalledEvent(typ EventType, call oneLeftCall) {
	if h.game.eventChan == nil {
		return
	}
	state := h.eventState()
	state.PlayerIndex = call.callerIndex
	state.OneLeftTarget = call.targetIndex
	h.game.sendEvent(typ, state, nil)
}

func (h *hand) eventState() *EventHand {
	hand := &EventHand{
		PlayerIndex:          h.playerIndex,
		PlayerCardsRemaining: make([]int, len(h.game.players)),
		DeckCardsRemaining:   h.deck.CardsRemaining(),
		DiscardStack:         make([]Card, len(h.discard)),
		LastDiscardWildColor: h.lastWildColor,
		Forward:              h.forward,
		OneLeftTarget:        -1,
	}
	for i, player := range h.game.players {
		hand.PlayerCardsRemaining[i] = player.CardsRemaining()
	}
	copy(hand.DiscardStack, h.discard)
	return hand
}
