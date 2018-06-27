package host

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type ClientPlayer struct {
	client    *Client
	cardCount int
	index     int
	currGame  *Game
}

func (c *ClientPlayer) CardsRemaining() int { return c.cardCount }

func (c *ClientPlayer) ChooseColorSinceFirstCardIsWild() (game.CardColor, error) {
	req := &pb.ChooseColorSinceFirstCardIsWildRequest{}
	resp, err := c.client.ChooseColorSinceFirstCardIsWild(context.Background(), req)
	if err != nil {
		return 0, err
	}
	return game.CardColor(resp.Color), nil
}

func (c *ClientPlayer) Play() (*game.PlayerPlay, error) {
	req := &pb.PlayRequest{}
	resp, err := c.client.Play(context.Background(), req)
	if err != nil {
		return nil, err
	} else if len(resp.EncryptedCard) == 0 {
		return &game.PlayerPlay{Card: game.NoCard}, nil
	}
	// Convert all bytes to big ints
	bigCard := new(big.Int).SetBytes(resp.EncryptedCard)
	bigKeys := make([]*big.Int, len(resp.CardDecryptionKeys))
	for i, decKey := range resp.CardDecryptionKeys {
		// We need to verify that the deck has seen all decryption keys but this players' index
		bigKeys[i] = new(big.Int).SetBytes(decKey)
	}
	// Decrypt the card
	card, err := c.currGame.deck.decryptCard(bigCard, bigKeys)
	if err != nil {
		return nil, err
	}
	// We verify that we've seen all decryption keys *except* the one playing here to prevent spoofing
	seenKeys := c.currGame.deck.seenDecryptionKeys[bigCard.String()]
	if len(seenKeys) != len(bigKeys) {
		return nil, fmt.Errorf("Invalid decryption key set size")
	}
	for i, seenKey := range seenKeys {
		if i == c.index && seenKey != nil {
			return nil, fmt.Errorf("We have already seen this player's decryption key before")
		} else if decKey := bigKeys[i]; i != c.index && (seenKey == nil || seenKey.Cmp(decKey) != 0) {
			return nil, fmt.Errorf("Decryption key mismatch")
		}
	}
	// Update the decryption keys so the full set it present
	c.currGame.deck.seenDecryptionKeys[bigCard.String()] = bigKeys
	return &game.PlayerPlay{Card: card, WildColor: game.CardColor(resp.WildColor)}, nil
}

func (c *ClientPlayer) ShouldChallengeWildDrawFour() (bool, error) {
	topColor, err := c.currGame.TopDiscardColor()
	if err != nil {
		return false, err
	}
	req := &pb.ShouldChallengeWildDrawFourRequest{PrevColor: uint32(topColor)}
	resp, err := c.client.ShouldChallengeWildDrawFour(context.Background(), req)
	if err != nil {
		return false, err
	}
	return resp.Challenge, nil
}

func (c *ClientPlayer) ChallengedWildDrawFour(challengerIndex int) (bool, error) {
	topColor, err := c.currGame.TopDiscardColor()
	if err != nil {
		return false, err
	}
	// First, ask this player for card reveal info
	meReq := &pb.RevealCardsForChallengeRequest{ChallengerIndex: uint32(challengerIndex), PrevColor: uint32(topColor)}
	meResp, err := c.client.RevealCardsForChallenge(context.Background(), meReq)
	if err != nil {
		return false, err
	}
	// Now, tell the other player of the cards and see if they agree on whether this will fail
	themReq := &pb.RevealedCardsForChallengeRequest{
		EncryptedCards:       meResp.EncryptedCards,
		CardDecryptionKeys:   meResp.CardDecryptionKeys,
		ChallengeWillSucceed: meResp.ChallengeWillSucceed,
	}
	themResp, err := c.client.RevealedCardsForChallenge(context.Background(), themReq)
	if err != nil {
		// This reassigns blame for the error
		return false, game.PlayerErrorf(challengerIndex, "%v", err)
	}
	// We both have to agree on whether the challenge was successful. This is just one of those human things in the
	// game though we could of course overcome it at the cost of exposing the cards to the host or more complicated
	// logic.
	if themResp.ChallengeSucceeded != meResp.ChallengeWillSucceed {
		return false, fmt.Errorf("Challenger has success as %v but challengee has it as %v",
			themResp.ChallengeSucceeded, meResp.ChallengeWillSucceed)
	}
	return themResp.ChallengeSucceeded, nil
}

func (c *ClientPlayer) SetOneLeftCallback(justGotOneLeftIndex int, callOneLeft func(target int)) {
	// TODO
}
