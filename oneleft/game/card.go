package game

// There are four groups of 25. Inside each group is 0-9, 1-9, skip, rev, and draw two in that order. After those 100
// there are 4 wilds and four wild draw four
type Card int

const NoCard Card = -1

// -1 if wild
func (c Card) Color() int {
	if c >= 100 {
		return -1
	}
	return int(c / 25)
}

type CardValue int

const Skip = 10
const Reverse = 11
const DrawTwo = 12
const Wild = 13
const WildDrawFour = 14

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

func (c Card) CanPlayOn(other Card, lastWildColor int) bool {
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

type CardDeck interface {
	CardsRemaining() int
	Shuffle([]Card) error
	DealTo(Player) error
	PopForFirstDiscard() (Card, error)
	CompleteHand([]Player) (CardDeckHandCompleteReveal, error)
}

type CardDeckHandCompleteReveal interface {
	PlayerCards() [][]Card
}
