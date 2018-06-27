package gametest

import (
	"fmt"
	"math/rand"

	"github.com/cretz/one-left/oneleft/game"
)

type SimpleDeck struct {
	*HandState
	Cards      []game.Card
	AllPlayers []game.Player
}

type SimpleDeckComplete struct {
	playerCards [][]game.Card
}

func (s *SimpleDeckComplete) PlayerCards() [][]game.Card { return s.playerCards }

func (s *SimpleDeck) CardsRemaining() int { return len(s.Cards) }

func (s *SimpleDeck) Shuffle(cards []game.Card) error {
	s.Cards = cards
	if s.Cards == nil {
		s.Cards = make([]game.Card, 108)
		for i := 0; i < 108; i++ {
			s.Cards[i] = game.Card(i)
		}
	}
	rand.Shuffle(len(s.Cards), func(i, j int) { s.Cards[i], s.Cards[j] = s.Cards[j], s.Cards[i] })
	return nil
}

func (s *SimpleDeck) DealTo(playerIndex int) error {
	player, ok := s.AllPlayers[playerIndex].(*PracticalPlayer)
	if !ok {
		return fmt.Errorf("Unexpected player type to deal to")
	}
	player.Cards = append(player.Cards, s.Cards[len(s.Cards)-1])
	rand.Shuffle(len(player.Cards), func(i, j int) {
		player.Cards[i], player.Cards[j] = player.Cards[j], player.Cards[i]
	})
	s.Cards = s.Cards[:len(s.Cards)-1]
	return nil
}

func (s *SimpleDeck) PopForFirstDiscard() (game.Card, error) {
	s.TopDiscard = s.Cards[len(s.Cards)-1]
	s.Cards = s.Cards[:len(s.Cards)-1]
	return s.TopDiscard, nil
}

func (s *SimpleDeck) CompleteHand() (game.CardDeckHandCompleteReveal, error) {
	ret := &SimpleDeckComplete{}
	for _, player := range s.AllPlayers {
		if p, ok := player.(*PracticalPlayer); !ok {
			return nil, fmt.Errorf("Unexpected player type")
		} else {
			ret.playerCards = append(ret.playerCards, p.Cards)
		}
	}
	return ret, nil
}
