package host

import (
	"fmt"
	"math/big"

	"github.com/cretz/one-left/oneleft/game"
)

type Deck struct {
	sharedPrime *big.Int
	// TODO: we really need to check this at the end
	seenDecryptionKeys map[game.Card][]*big.Int
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
