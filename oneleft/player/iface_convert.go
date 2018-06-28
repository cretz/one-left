package player

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cretz/bine/torutil/ed25519"
	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
	"github.com/cretz/one-left/oneleft/player/iface"
)

func convertPlayers(v []*pb.PlayerIdentity) ([]*iface.Player, error) {
	ret := make([]*iface.Player, len(v))
	for i, p := range v {
		if !p.VerifyIdentity() {
			return nil, fmt.Errorf("Invalid player identity for player %v", i)
		}
		ret[i] = &iface.Player{
			ID:   ed25519.PublicKey(p.Id),
			Name: p.Name,
		}
	}
	return ret, nil
}

func convertChatMessages(v []*pb.ChatMessage) ([]*iface.ChatMessage, error) {
	ret := make([]*iface.ChatMessage, len(v))
	var err error
	for i, c := range v {
		if ret[i], err = convertChatMessage(c); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func convertChatMessage(v *pb.ChatMessage) (*iface.ChatMessage, error) {
	if !v.Verify() {
		return nil, fmt.Errorf("Signature failed")
	}
	return &iface.ChatMessage{
		Player:   iface.Player{ID: ed25519.PublicKey(v.PlayerId), Name: v.PlayerName},
		Contents: v.Contents,
		Time:     time.Unix(int64(v.HostUtcMs)/1000, int64(v.HostUtcMs%1000)*int64(time.Millisecond)),
	}, nil
}

func convertUInt32sToInts(v []uint32) []int {
	ret := make([]int, len(v))
	for i, s := range v {
		ret[i] = int(s)
	}
	return ret
}

func convertUInt32sToCards(v []uint32) []game.Card {
	ret := make([]game.Card, len(v))
	for i, s := range v {
		ret[i] = game.Card(s)
	}
	return ret
}

func convertGameEvent(v *pb.HostMessage_GameEvent) (*iface.GameEvent, error) {
	if v == nil {
		return nil, nil
	}
	event := &iface.GameEvent{
		Type:         game.EventType(v.Type),
		PlayerScores: convertUInt32sToInts(v.PlayerScores),
		DealerIndex:  int(v.DealerIndex),
	}
	var err error
	if event.GameID, err = uuid.FromBytes(v.GameId); err != nil {
		return nil, err
	}
	if v.Hand != nil {
		event.Hand = &iface.GameEventHand{
			PlayerIndex:          int(v.Hand.PlayerIndex),
			PlayerCardsRemaining: convertUInt32sToInts(v.Hand.PlayerCardsRemaining),
			DeckCardsRemaining:   int(v.Hand.DeckCardsRemaining),
			DiscardStack:         convertUInt32sToCards(v.Hand.DiscardStack),
			LastDiscardWildColor: game.CardColor(v.Hand.LastDiscardWildColor),
			Forward:              v.Hand.Forward,
			OneLeftTarget:        int(v.Hand.OneLeftTarget),
		}
		if event.Hand.HandID, err = uuid.FromBytes(v.Hand.HandId); err != nil {
			return nil, err
		}
	}
	if v.HandComplete != nil {
		event.HandComplete = &iface.GameEventHandComplete{
			WinnerIndex: int(v.HandComplete.WinnerIndex),
			Score:       int(v.HandComplete.Score),
			DeckCards:   convertUInt32sToCards(v.HandComplete.DeckCards),
			PlayerCards: make([][]game.Card, len(v.HandComplete.PlayerCards)),
		}
		for i, playerCards := range v.HandComplete.PlayerCards {
			event.HandComplete.PlayerCards[i] = convertUInt32sToCards(playerCards.PlayerCards)
		}
	}
	return event, nil
}
