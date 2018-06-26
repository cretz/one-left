package game

import "fmt"

type Player interface {
	CardsRemaining() int
	ChooseColorSinceFirstCardIsWild() (CardColor, error)
	// Card can be -1 (DrawCard)
	Play() (*PlayerPlay, error)
	ShouldChallengeWildDrawFour() (bool, error)
	ChallengedWildDrawFour(challenger Player) (bool, error)
	SetOneLeftCallback(justGotOneLeftIndex int, callOneLeft func(target int))
}

type PlayerPlay struct {
	Card Card
	// Leave as unset (0) when card is not wild
	WildColor CardColor
}

func (p *PlayerPlay) AssertValid() error {
	if p.Card < -1 || p.Card > 107 {
		return fmt.Errorf("Unknown card number from raw value %v", p.Card)
	} else if p.Card < 100 && p.WildColor != 0 {
		return fmt.Errorf("Wild color set on non-wild card")
	} else if p.WildColor < 0 || p.WildColor > 3 {
		return fmt.Errorf("Unknown wild color %v", p.WildColor)
	}
	return nil
}
