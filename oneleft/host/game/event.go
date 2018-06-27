package game

import (
	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

func (g *Game) gameEventToPbEvent(event *game.Event) *pb.HostMessage_GameEvent {
	ret := &pb.HostMessage_GameEvent{
		GameId:       g.id[:],
		Type:         pb.HostMessage_GameEvent_Type(event.Type),
		PlayerScores: make([]uint32, len(event.PlayerScores)),
		DealerIndex:  uint32(event.DealerIndex),
	}
	for i, s := range event.PlayerScores {
		ret.PlayerScores[i] = uint32(s)
	}
	if event.Hand != nil {
		ret.Hand = &pb.HostMessage_GameEvent_Hand{
			HandId:               g.deck.handID[:],
			PlayerIndex:          uint32(event.Hand.PlayerIndex),
			PlayerCardsRemaining: make([]uint32, len(event.Hand.PlayerCardsRemaining)),
			DeckCardsRemaining:   uint32(event.Hand.DeckCardsRemaining),
			DiscardStack:         make([]uint32, len(event.Hand.DiscardStack)),
			LastDiscardWildColor: int32(event.Hand.LastDiscardWildColor),
			Forward:              event.Hand.Forward,
			OneLeftTarget:        int32(event.Hand.OneLeftTarget),
		}
		for i, c := range event.Hand.PlayerCardsRemaining {
			ret.Hand.PlayerCardsRemaining[i] = uint32(c)
		}
		for i, c := range event.Hand.DiscardStack {
			ret.Hand.DiscardStack[i] = uint32(c)
		}
	}
	if event.HandComplete != nil {
		reveal := event.HandComplete.DeckReveal.(*handCompleteReveal)
		ret.HandComplete = &pb.HostMessage_GameEvent_HandComplete{
			WinnerIndex: uint32(event.HandComplete.WinnerIndex),
			Score:       uint32(event.HandComplete.Score),
			DeckCards:   make([]uint32, len(reveal.deckCards)),
			PlayerCards: make([]*pb.HostMessage_GameEvent_HandComplete_PlayerCards, len(reveal.playerCards)),
		}
		for i, c := range reveal.deckCards {
			ret.HandComplete.DeckCards[i] = uint32(c)
		}
		for i, cards := range reveal.playerCards {
			playerCards := make([]uint32, len(cards))
			for j, c := range cards {
				playerCards[j] = uint32(c)
			}
			ret.HandComplete.PlayerCards[i] = &pb.HostMessage_GameEvent_HandComplete_PlayerCards{
				PlayerCards: playerCards,
			}
		}
	}
	return ret
}
