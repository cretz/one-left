package host

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"sync"

	"crypto/rand"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
	"github.com/golang/protobuf/proto"
)

type Deck struct {
	game        *Game
	sharedPrime *big.Int
	// Keyed by orig encrypted card big.int serialized to string
	seenDecryptionKeys  map[string][]*big.Int
	handStartPlayerSigs [][]byte
	// Must be sorted and the start since the start of the hand
	origStartCards []game.Card
	// Just since last shuffle
	unencryptedStartCards       []game.Card
	encryptedCards              []*big.Int
	encryptedCardsHeldByPlayers map[string]int
}

// TODO: conf
const sharedPrimeBits = 256

func (g *Game) NewDeck(handStartSigs [][]byte) (*Deck, error) {
	deck := &Deck{
		game:                        g,
		seenDecryptionKeys:          map[string][]*big.Int{},
		handStartPlayerSigs:         handStartSigs,
		origStartCards:              make([]game.Card, 108),
		encryptedCardsHeldByPlayers: map[string]int{},
	}
	var err error
	if deck.sharedPrime, err = rand.Prime(rand.Reader, sharedPrimeBits); err != nil {
		return nil, fmt.Errorf("Failed generating shared prime: %v", err)
	}
	// Set up the orig deck
	// TODO: It'd be nice if we could make a big list of large, random values to represent cards
	// to mitigate card number leaks based on low values during first encrypt.
	for i := 0; i < 108; i++ {
		deck.origStartCards[i] = game.Card(i)
	}
	panic("TODO")
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
		d.unencryptedStartCards = d.origStartCards
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
	topCard, decryptionKeys, err := d.popTopCardForDeal(playerIndex)
	if err != nil {
		return err
	}
	// Now send off to the player as a deal
	giveReq := &pb.GiveDeckTopCardRequest{DecryptionKeys: make([][]byte, len(d.game.players))}
	for i, decryptionKey := range decryptionKeys {
		giveReq.DecryptionKeys[i] = decryptionKey.Bytes()
	}
	d.encryptedCardsHeldByPlayers[topCard.String()] = playerIndex
	_, err = d.game.players[playerIndex].client.GiveDeckTopCard(context.Background(), giveReq)
	return err
}

// This also updates seen decryption keys...do not mutate the result. Doesn't give encryption keys for playerIndex or
// gives em all if playerIndex out of player array bounds.
func (d *Deck) popTopCardForDeal(playerIndex int) (topCard *big.Int, decryptionKeys []*big.Int, err error) {
	decryptionKeys = make([]*big.Int, len(d.game.players))
	getTopReq := &pb.GetDeckTopDecryptionKeyRequest{ForPlayerIndex: int32(playerIndex)}
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

func (d *Deck) CompleteHand() (game.CardDeckHandCompleteReveal, error) {
	// Find winner
	winnerIndex := -1
	for i, p := range d.game.players {
		if p.cardCount == 0 {
			if winnerIndex != -1 {
				return nil, fmt.Errorf("At least two players with 0 cards left, %v and %v", winnerIndex, i)
			}
			winnerIndex = i
		}
	}
	if winnerIndex == -1 {
		return nil, fmt.Errorf("Could not find winner")
	}
	// We go in two hand-end stages. First get player infos, second have everyone verify and sign em.
	// Stage 0, just get player info
	req := &pb.HandEndRequest{
		Stage:              0,
		WinnerIndex:        uint32(winnerIndex),
		EncryptedDeckCards: make([][]byte, len(d.encryptedCards)),
	}
	for i, encryptedDeckCard := range d.encryptedCards {
		req.EncryptedDeckCards[i] = encryptedDeckCard.Bytes()
	}
	resps, err := d.doAllHandEnds(req)
	if err != nil {
		return nil, err
	}
	// Now, we traverse the responses, building up player infos and the score
	completeReveal := &handCompleteReveal{
		playerCards: make([][]game.Card, len(d.game.players)),
		endInfo:     req,
	}
	allCardsTogether := []game.Card{}
	for i, resp := range resps {
		respReveal, ok := resp.Message.(*pb.HandEndResponse_Reveal)
		if !ok || respReveal == nil || respReveal.Reveal == nil {
			return nil, game.PlayerErrorf(i, "Invalid player hand end first stage response")
		}
		info := &pb.HandEndRequest_PlayerInfo{
			EncryptedCardsInHand:   respReveal.Reveal.EncryptedCardsInHand,
			UnencryptedCardsInHand: respReveal.Reveal.UnencryptedCardsInHand,
			CardDecryptionKeys:     respReveal.Reveal.CardDecryptionKeys,
			Score:                  uint32(d.game.players[i].score),
		}
		// Some quick checks compared to what we know...
		if cardCount := len(info.EncryptedCardsInHand); cardCount != len(info.UnencryptedCardsInHand) {
			return nil, game.PlayerErrorf(i, "Number of encrypted and unencrypted cards don't match")
		} else if cardCount != d.game.players[i].cardCount {
			return nil, game.PlayerErrorf(i, "We expected %v cards, got %v", d.game.players[i].cardCount, cardCount)
		}
		// Check that we had given it to them
		encCardsStrsInHand := map[string]struct{}{}
		for _, encCard := range info.EncryptedCardsInHand {
			encCardStr := new(big.Int).SetBytes(encCard).String()
			encCardsStrsInHand[encCardStr] = struct{}{}
			// Held by player?
			if playerIndex, ok := d.encryptedCardsHeldByPlayers[encCardStr]; !ok || playerIndex != i {
				return nil, game.PlayerErrorf(i, "Found card we hadn't given")
			}
		}
		// Check all decryption keys to make sure we've either seen them or they are for a card in hand
		for encCardStr, decKey := range info.CardDecryptionKeys {
			_, myCard := encCardsStrsInHand[encCardStr]
			mySeenKey := d.seenDecryptionKeys[encCardStr][i]
			if myCard && mySeenKey != nil {
				return nil, game.PlayerErrorf(i, "Already seen player's dec key for player card")
			} else if !myCard && mySeenKey.Cmp(new(big.Int).SetBytes(decKey)) != 0 {
				return nil, game.PlayerErrorf(i, "Haven't seen player's dec key before for non-self card")
			}
		}
		// Add to the score and complete-reveal card set
		completeReveal.playerCards[i] = make([]game.Card, len(info.UnencryptedCardsInHand))
		for cardIndex, unencCard := range info.UnencryptedCardsInHand {
			bigCard := big.NewInt(int64(unencCard))
			if bigCard.BitLen() > 32 {
				return nil, game.PlayerErrorf(i, "Invalid unencrypted card")
			}
			card := game.Card(bigCard.Int64())
			if !card.Valid() {
				return nil, game.PlayerErrorf(i, "Invalid card value")
			}
			allCardsTogether = append(allCardsTogether, card)
			completeReveal.playerCards[i][cardIndex] = card
			req.Score += uint32(card.Score())
		}
		// Set the info
		req.PlayerInfos = append(req.PlayerInfos, info)
	}
	// Now, with all infos, decrypt cards and verify they match the unencrypted
	decryptCard := func(encCard *big.Int) (game.Card, error) {
		encCardStr := encCard.String()
		decKeys := make([]*big.Int, len(d.game.players))
		for decKeyIndex, otherInfo := range req.PlayerInfos {
			decKeys[decKeyIndex] = new(big.Int).SetBytes(otherInfo.CardDecryptionKeys[encCardStr])
		}
		return d.decryptCard(encCard, decKeys)
	}
	// First, check each hand
	for playerIndex, info := range req.PlayerInfos {
		for cardIndex, encCard := range info.EncryptedCardsInHand {
			unencCard := game.Card(big.NewInt(int64(info.UnencryptedCardsInHand[cardIndex])).Int64())
			if decCard, err := decryptCard(new(big.Int).SetBytes(encCard)); err != nil {
				return nil, game.PlayerErrorf(playerIndex, "Unable to decrypt hand card: %v", err)
			} else if decCard != unencCard {
				return nil, game.PlayerErrorf(playerIndex, "Encrypted card didn't match unencrypted card")
			}
		}
	}
	// Now check the deck
	for _, deckEncCard := range d.encryptedCards {
		if card, err := decryptCard(deckEncCard); err != nil {
			return nil, fmt.Errorf("Unable to decrypt deck card: %v", err)
		} else if !card.Valid() {
			return nil, fmt.Errorf("Deck card invalid")
		} else {
			allCardsTogether = append(allCardsTogether, card)
		}
	}
	// Now sort the all the cards seen and make sure they exactly match the original set
	sort.Slice(allCardsTogether, func(i, j int) bool { return allCardsTogether[i] < allCardsTogether[j] })
	if len(allCardsTogether) != len(d.origStartCards) {
		return nil, fmt.Errorf("Started with %v cards, ended with %v", len(d.origStartCards), len(allCardsTogether))
	}
	for i, origCard := range d.origStartCards {
		if origCard != allCardsTogether[i] {
			return nil, fmt.Errorf("Card %v ended as %v, expected %v", i, allCardsTogether[i], origCard)
		}
	}
	// Add the score to the winner info score
	d.game.players[winnerIndex].score += int(req.Score)
	req.PlayerInfos[winnerIndex].Score += req.Score
	// Stage 1, send what we've learned and check sigs
	req.Stage = 1
	if resps, err = d.doAllHandEnds(req); err != nil {
		return nil, err
	}
	// Check sigs
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling req: %v", err)
	}
	for i, resp := range resps {
		respSig, ok := resp.Message.(*pb.HandEndResponse_Sig)
		if !ok || respSig == nil {
			return nil, game.PlayerErrorf(i, "Invalid player hand end second response")
		} else if !d.game.players[i].client.VerifySig(reqBytes, respSig.Sig) {
			return nil, game.PlayerErrorf(i, "Hand end signature verification failed")
		}
		completeReveal.endSigs = append(completeReveal.endSigs, respSig.Sig)
	}
	return completeReveal, nil
}

func (d *Deck) doAllHandEnds(req *pb.HandEndRequest) ([]*pb.HandEndResponse, error) {
	// Send em all async, first err causes failure
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	errCh := make(chan error, len(d.game.players))
	resps := make([]*pb.HandEndResponse, len(d.game.players))
	var wg sync.WaitGroup
	for i, p := range d.game.players {
		wg.Add(1)
		go func(i int, p *ClientPlayer) {
			defer wg.Done()
			if resp, err := p.client.HandEnd(ctx, req); err != nil {
				errCh <- game.PlayerErrorf(i, "Failed getting hand end: %v", err)
			} else {
				resps[i] = resp
			}
		}(i, p)
	}
	doneCh := make(chan struct{}, 1)
	go func() { wg.Wait(); doneCh <- struct{}{} }()
	select {
	case err := <-errCh:
		return nil, err
	case <-doneCh:
		return resps, nil
	}
}

type handCompleteReveal struct {
	playerCards [][]game.Card
	endInfo     *pb.HandEndRequest
	endSigs     [][]byte
}

func (h *handCompleteReveal) PlayerCards() [][]game.Card { return h.playerCards }
