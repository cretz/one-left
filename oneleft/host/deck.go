package host

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type Deck struct {
	game        *Game
	sharedPrime *big.Int
	// TODO: we really need to check this at the end
	seenDecryptionKeys    map[game.Card][]*big.Int
	handStartPlayerSigs   [][]byte
	unencryptedStartCards []game.Card
	encryptedCards        []*big.Int
}

func (d *Deck) decryptCard(card *big.Int, decryptionKeys []*big.Int) (game.Card, error) {
	for _, decryptionKey := range decryptionKeys {
		card = new(big.Int).Exp(card, decryptionKey, d.sharedPrime)
	}
	if card.BitLen() <= 32 {
		if ret := game.Card(card.Int64()); ret.Valid() {
			return ret, nil
		}
	}
	return 0, fmt.Errorf("Decryption failed, resulting card: %v", card)
}

/*CardsRemaining() int
Shuffle([]Card) error
DealTo(Player) error
PopForFirstDiscard() (Card, error)
CompleteHand([]Player) (CardDeckHandCompleteReveal, error)*/

func (d *Deck) CardsRemaining() int { return len(d.encryptedCards) }

func (d *Deck) Shuffle(startCards []game.Card) error {
	// If cards are empty, assume new full set
	d.unencryptedStartCards = startCards
	if len(d.unencryptedStartCards) == 0 {
		// TODO: It'd be nice if we could make a big list of large, random values to represent cards
		// to mitigate card number leaks based on low indices.
		d.unencryptedStartCards = make([]game.Card, 108)
		for i := 0; i < 108; i++ {
			d.unencryptedStartCards[i] = game.Card(i)
		}
	}
	// Build stage 0 request
	req := &pb.ShuffleRequest{
		HandStartPlayerSigs: d.handStartPlayerSigs,
		Stage:               0,
		UnencryptedStartCards: make([]uint32, len(d.unencryptedStartCards)),
		WorkingCardSet:        make([][]byte, len(d.unencryptedStartCards)),
	}
	for i, card := range d.unencryptedStartCards {
		req.UnencryptedStartCards[i] = uint32(card)
		req.WorkingCardSet[i] = big.NewInt(int64(card)).Bytes()
	}
	// Pass it around
	ctx := context.Background()
	for _, player := range d.game.players {
		resp, err := player.client.Shuffle(ctx, req)
		if err != nil {
			// TODO: enforce blame
			return err
		}
		req.WorkingCardSet = resp.WorkingCardSet
	}
	// Pass around again for stage 1
	req.Stage = 1
	for _, player := range d.game.players {
		resp, err := player.client.Shuffle(ctx, req)
		if err != nil {
			// TODO: enforce blame
			return err
		}
		req.WorkingCardSet = resp.WorkingCardSet
	}
	if len(req.WorkingCardSet) != len(d.unencryptedStartCards) {
		return fmt.Errorf("The deck size changed during encryption")
	}
	// Pass around at end just so they can record the result
	req.Stage = 2
	for _, player := range d.game.players {
		if _, err := player.client.Shuffle(ctx, req); err != nil {
			// TODO: enforce blame
			return err
		}
	}
	// Now store the new encrypted deck
	d.encryptedCards = make([]*big.Int, len(req.WorkingCardSet))
	for i, card := range req.WorkingCardSet {
		d.encryptedCards[i] = new(big.Int).SetBytes(card)
	}
	return nil
}

func (d *Deck) DealTo(playerIndex int) error {
	panic("TODO")
}

func (d *Deck) PopForFirstDiscard() (game.Card, error) {
	panic("TODO")
}

func (d *Deck) CompleteHand(players []game.Player) (game.CardDeckHandCompleteReveal, error) {
	panic("TODO")
}
