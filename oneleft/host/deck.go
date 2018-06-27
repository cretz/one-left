package host

import (
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

type Deck struct {
	game        *Game
	sharedPrime *big.Int
	// Keyed by orig encrypted card big.int serialized to string
	seenDecryptionKeys    map[string][]*big.Int
	handStartPlayerSigs   [][]byte
	unencryptedStartCards []game.Card
	// Byte arrays are just big.Int bytes
	encryptedCards []*big.Int
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
	for playerIndex, player := range d.game.players {
		resp, err := player.client.Shuffle(ctx, req)
		if err != nil {
			// This assigns blame for the error
			return game.PlayerErrorf(playerIndex, "Failed shuffle stage 0: %v", err)
		}
		req.WorkingCardSet = resp.WorkingCardSet
	}
	// Pass around again for stage 1
	req.Stage = 1
	for playerIndex, player := range d.game.players {
		resp, err := player.client.Shuffle(ctx, req)
		if err != nil {
			// This assigns blame for the error
			return game.PlayerErrorf(playerIndex, "Failed shuffle stage 1: %v", err)
		}
		req.WorkingCardSet = resp.WorkingCardSet
	}
	if len(req.WorkingCardSet) != len(d.unencryptedStartCards) {
		return fmt.Errorf("The deck size changed during encryption")
	}
	// Pass around at end just so they can record the result
	req.Stage = 2
	for playerIndex, player := range d.game.players {
		if _, err := player.client.Shuffle(ctx, req); err != nil {
			// This assigns blame for the error
			return game.PlayerErrorf(playerIndex, "Failed shuffle stage 2: %v", err)
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
	// Grab all decryption keys except this one
	_, decryptionKeys, err := d.popTopCardForDeal(playerIndex)
	if err != nil {
		return err
	}
	// Now send off to the player as a deal
	giveReq := &pb.GiveDeckTopCardRequest{DecryptionKeys: make([][]byte, len(d.game.players))}
	for i, decryptionKey := range decryptionKeys {
		giveReq.DecryptionKeys[i] = decryptionKey.Bytes()
	}
	_, err = d.game.players[playerIndex].client.GiveDeckTopCard(context.Background(), giveReq)
	return err
}

// This also updates seen decryption keys...do not mutate the result. Doesn't give encryption keys for playerIndex or
// gives em all if playerIndex out of player array bounds.
func (d *Deck) popTopCardForDeal(playerIndex int) (topCard *big.Int, decryptionKeys []*big.Int, err error) {
	decryptionKeys = make([]*big.Int, len(d.game.players))
	getTopReq := &pb.GetDeckTopDecryptionKeyRequest{ForPlayerIndex: uint32(playerIndex)}
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	// Ask all players except curr one for the top-card decryption...do async, first err fails
	errCh := make(chan error, len(d.game.players))
	var wg sync.WaitGroup
	for i, p := range d.game.players {
		if i != playerIndex {
			wg.Add(1)
			go func(playerIndex int, player *ClientPlayer) {
				defer wg.Done()
				if resp, err := player.client.GetDeckTopDecryptionKey(ctx, getTopReq); err != nil {
					errCh <- game.PlayerErrorf(playerIndex, "Failed getting dec key: %v", err)
				} else {
					decryptionKeys[playerIndex] = new(big.Int).SetBytes(resp.DecryptionKey)
				}
			}(i, p)
		}
	}
	// Wait for complete or err
	doneCh := make(chan struct{}, 1)
	go func() { wg.Wait(); doneCh <- struct{}{} }()
	select {
	case err = <-errCh:
	case <-doneCh:
		// Pop the top card and set the seen decryption keys
		topCard = d.encryptedCards[len(d.encryptedCards)-1]
		d.encryptedCards = d.encryptedCards[:len(d.encryptedCards)-1]
		d.seenDecryptionKeys[topCard.String()] = decryptionKeys
	}
	return
}

func (d *Deck) PopForFirstDiscard() (game.Card, error) {
	// Grab all decryption keys for -1 index (which is all of em)
	topCard, decryptionKeys, err := d.popTopCardForDeal(-1)
	if err != nil {
		return 0, err
	}
	return d.decryptCard(topCard, decryptionKeys)
}

func (d *Deck) CompleteHand(players []game.Player) (game.CardDeckHandCompleteReveal, error) {
	panic("TODO")
}
