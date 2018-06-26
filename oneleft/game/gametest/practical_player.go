package gametest

import (
	"fmt"
	"time"

	"github.com/cretz/one-left/oneleft/game"
)

type HandState struct {
	TopDiscard          game.Card
	TopDiscardWildColor game.CardColor
}

type PracticalPlayer struct {
	*HandState
	Index int
	// Randomized
	Cards []game.Card
}

func (p *PracticalPlayer) CardsRemaining() int {
	return len(p.Cards)
}

func (p *PracticalPlayer) mostPopularColor() game.CardColor {
	countsByColor := map[game.CardColor]int{}
	maxCountSoFar := 0
	maxColorSoFar := game.ColorRed
	for _, card := range p.Cards {
		if color := card.Color(); color != game.ColorUnknown {
			countsByColor[color]++
			if count := countsByColor[color]; count > maxCountSoFar {
				maxColorSoFar = color
			}
		}
	}
	return maxColorSoFar
}

func (p *PracticalPlayer) ChooseColorSinceFirstCardIsWild() (game.CardColor, error) {
	p.TopDiscardWildColor = p.mostPopularColor()
	return p.TopDiscardWildColor, nil
}

func (p *PracticalPlayer) Play() (*game.PlayerPlay, error) {
	// Try same color or symbol, then try any wild, then draw
	for cardIndex, card := range p.Cards {
		if !card.Wild() && card.CanPlayOn(p.TopDiscard, p.TopDiscardWildColor) {
			p.TopDiscard = card
			p.TopDiscardWildColor = game.ColorUnknown
			p.Cards = append(p.Cards[:cardIndex], p.Cards[cardIndex+1:]...)
			return &game.PlayerPlay{Card: card}, nil
		}
	}
	for cardIndex, card := range p.Cards {
		if card.Wild() {
			p.TopDiscard = card
			p.TopDiscardWildColor = p.mostPopularColor()
			p.Cards = append(p.Cards[:cardIndex], p.Cards[cardIndex+1:]...)
			return &game.PlayerPlay{Card: card, WildColor: p.TopDiscardWildColor}, nil
		}
	}
	return &game.PlayerPlay{Card: game.NoCard}, nil
}

func (p *PracticalPlayer) ShouldChallengeWildDrawFour() (bool, error) {
	return false, nil
}

func (p *PracticalPlayer) ChallengedWildDrawFour(challenger game.Player) (bool, error) {
	if _, ok := challenger.(*PracticalPlayer); !ok {
		return false, fmt.Errorf("Other player is not same player type")
	}
	return false, nil
}

func (p *PracticalPlayer) SetOneLeftCallback(justGotOneLeftIndex int, callOneLeft func(target int)) {
	// If it's me do it now, otherwise wait 3 seconds
	if justGotOneLeftIndex == p.Index {
		callOneLeft(justGotOneLeftIndex)
	} else {
		time.AfterFunc(3*time.Second, func() { callOneLeft(justGotOneLeftIndex) })
	}
}
