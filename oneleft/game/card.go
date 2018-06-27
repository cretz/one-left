package game

import (
	"strconv"
)

// There are four groups of 25. Inside each group is 0-9, 1-9, skip, rev, and draw two in that order. After those 100
// there are 4 wilds and four wild draw four
type Card int

const NoCard Card = -1

type CardColor int

const ColorUnknown CardColor = -1
const ColorRed CardColor = 0
const ColorYellow CardColor = 1
const ColorGreen CardColor = 2
const ColorBlue CardColor = 3

var cardColorNames = map[CardColor]string{
	ColorUnknown: "Unknown",
	ColorRed:     "Red",
	ColorYellow:  "Yellow",
	ColorGreen:   "Green",
	ColorBlue:    "Blue",
}

func (c CardColor) String() string { return cardColorNames[c] }
func (c CardColor) Valid() bool    { return c >= ColorRed && c <= ColorBlue }

func (c Card) Color() CardColor {
	if c >= 100 {
		return ColorUnknown
	}
	return CardColor(c / 25)
}

type CardValue int

const Skip CardValue = 10
const Reverse CardValue = 11
const DrawTwo CardValue = 12
const Wild CardValue = 13
const WildDrawFour CardValue = 14

func (c CardValue) String() string {
	switch c {
	case Skip:
		return "Skip"
	case Reverse:
		return "Reverse"
	case DrawTwo:
		return "DrawTwo"
	case Wild:
		return "Wild"
	case WildDrawFour:
		return "WildDrawFour"
	default:
		return strconv.Itoa(int(c))
	}
}

func (c Card) Value() CardValue {
	switch value := c % 25; {
	case c >= 104:
		return WildDrawFour
	case c >= 100:
		return Wild
	case value >= 23:
		return DrawTwo
	case value >= 21:
		return Reverse
	case value >= 19:
		return Skip
	case value >= 10:
		return CardValue(value - 9)
	default:
		return CardValue(value)
	}
}

func (c Card) Wild() bool {
	v := c.Value()
	return v == Wild || v == WildDrawFour
}

func (c Card) CanPlayOn(other Card, lastWildColor CardColor) bool {
	return c.Wild() ||
		c.Value() == other.Value() ||
		c.Color() == other.Color() ||
		(other.Wild() && lastWildColor == c.Color())
}

func (c Card) Score() int {
	switch v := c.Value(); v {
	case Wild, WildDrawFour:
		return 50
	case Skip, Reverse, DrawTwo:
		return 20
	default:
		return int(v)
	}
}

func (c Card) String() string {
	v := c.Value()
	if v == Wild || v == WildDrawFour {
		return v.String()
	}
	return c.Color().String() + "-" + v.String()
}

func (c Card) Valid() bool {
	return c >= 0 && c <= 107
}

type CardDeck interface {
	CardsRemaining() int
	Shuffle([]Card) error
	DealTo(playerIndex int) error
	PopForFirstDiscard() (Card, error)
	CompleteHand() (CardDeckHandCompleteReveal, error)
}

type CardDeckHandCompleteReveal interface {
	PlayerCards() [][]Card
}
