package player

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"

	"github.com/cretz/one-left/oneleft/crypto"
	"github.com/cretz/one-left/oneleft/crypto/sra"
	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
)

func (p *handler) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	ident := &pb.PlayerIdentity{
		Id:          p.player.keyPair.PublicKey(),
		RandomNonce: req.RandomNonce,
		Name:        p.player.name,
	}
	var err error
	if ident.Sig, err = p.player.signProto(ident); err != nil {
		return nil, err
	}
	return &pb.JoinResponse{Player: ident}, nil
}

func (p *handler) GameStart(ctx context.Context, req *pb.GameStartRequest) (*pb.GameStartResponse, error) {
	// Find my index
	myIndex := -1
	for i, player := range req.Players {
		if bytes.Equal(player.Id, p.player.keyPair.PublicKey()) {
			myIndex = i
			break
		}
	}
	if myIndex == -1 {
		return nil, fmt.Errorf("Unable to find myself")
	}
	// Update data
	p.dataLock.Lock()
	p.myIndex = myIndex
	p.sharedPrime = nil
	p.shuffleStage0Pair = nil
	p.shuffleStage1Pairs = nil
	p.cardPairs = map[string]*sra.KeyPair{}
	p.encryptedDeckCards = nil
	p.encryptedCardsGivenToPlayers = map[string]int{}
	p.myCards = nil
	p.lastEvent = nil
	p.lastGameStart = req
	p.lastHandStart = nil
	p.lastHandEnd = nil
	p.dataLock.Unlock()

	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if id, err := uuid.FromBytes(req.Id); err != nil {
		return nil, err
	} else if players, err := convertPlayers(req.Players); err != nil {
		return nil, err
	} else if err := p.ui.GameStart(ctx, id, players); err != nil {
		return nil, err
	} else if sig, err := p.player.signProto(req); err != nil {
		return nil, err
	} else {
		return &pb.GameStartResponse{Sig: sig}, nil
	}
}

func (p *handler) GameEnd(ctx context.Context, req *pb.GameEndRequest) (*pb.GameEndResponse, error) {
	panic("TODO")
}

func (p *handler) HandStart(ctx context.Context, req *pb.HandStartRequest) (*pb.HandStartResponse, error) {
	// Check prime (n = 20 like the crypto rand's prime generator)
	sharedPrime := new(big.Int).SetBytes(req.SharedCardPrime)
	if sharedPrime.BitLen() < minPrimeBitLen || !sharedPrime.ProbablyPrime(20) {
		return nil, fmt.Errorf("Invalid shared prime")
	}
	// Get hand ID
	handID, err := uuid.FromBytes(req.Id)
	if err != nil {
		return nil, fmt.Errorf("Invalid hand ID: %v", handID)
	}
	// Grab some data, set some data
	p.dataLock.Lock()
	lastEvent := p.lastEvent
	lastGameStart := p.lastGameStart
	lastHandEnd := p.lastHandEnd
	lastHandStart := p.lastHandStart
	p.sharedPrime = sharedPrime
	p.lastHandStart = req
	p.lastHandID = handID
	p.dataLock.Unlock()
	// Do some validation
	if lastEvent == nil {
		return nil, fmt.Errorf("No previous event")
	} else if lastEvent.Type != game.EventGameStart && lastEvent.Type != game.EventHandEnd {
		return nil, fmt.Errorf("Unexpected hand start")
	} else if lastGameStart == nil ||
		(lastEvent.Type == game.EventHandEnd && (lastHandEnd == nil || lastHandStart == nil)) {
		return nil, fmt.Errorf("Missing previous game start or hand start/end")
	} else if len(lastEvent.PlayerScores) != len(req.PlayerScores) {
		return nil, fmt.Errorf("Invalid player scores")
	}
	// Check scores
	for i, s := range lastEvent.PlayerScores {
		if req.PlayerScores[i] != uint32(s) {
			return nil, fmt.Errorf("Invalid player score")
		}
	}
	// Check dealer index
	expectedDealerIndex := uint32(0)
	if lastHandStart != nil {
		if expectedDealerIndex = lastHandStart.DealerIndex + 1; int(expectedDealerIndex) == len(lastGameStart.Players) {
			expectedDealerIndex = 0
		}
	}
	if expectedDealerIndex != req.DealerIndex {
		return nil, fmt.Errorf("Invalid dealer index")
	}
	// Check game start sigs
	if len(req.GameStartPlayerSigs) != len(lastGameStart.Players) {
		return nil, fmt.Errorf("Invalid game start sigs")
	}
	gameStartBytes, err := proto.Marshal(lastGameStart)
	if err != nil {
		return nil, fmt.Errorf("Failed marshalling: %v", err)
	}
	for i, sig := range req.GameStartPlayerSigs {
		if !lastGameStart.Players[i].VerifySig(gameStartBytes, sig) {
			return nil, fmt.Errorf("Invalid game start sig")
		}
	}
	// Check hand start sigs
	if lastHandEnd != nil {
		if len(req.LastHandEndPlayerSigs) != len(lastGameStart.Players) {
			return nil, fmt.Errorf("Invalid hand end sigs")
		}
		handEndBytes, err := proto.Marshal(lastHandEnd)
		if err != nil {
			return nil, fmt.Errorf("Failed marshalling: %v", err)
		}
		for i, sig := range req.LastHandEndPlayerSigs {
			if !lastGameStart.Players[i].VerifySig(handEndBytes, sig) {
				return nil, fmt.Errorf("Invalid hand end sig")
			}
		}
	}
	// Build response
	resp := &pb.HandStartResponse{}
	if resp.Sig, err = p.player.signProto(req); err != nil {
		return nil, err
	}
	// Call downstream
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if err := p.ui.HandStart(ctx, int(req.DealerIndex)); err != nil {
		return nil, err
	}
	return resp, nil
}

func (p *handler) HandEnd(ctx context.Context, req *pb.HandEndRequest) (*pb.HandEndResponse, error) {
	// TODO: lock
	panic("TODO")
}

func (p *handler) Shuffle(ctx context.Context, req *pb.ShuffleRequest) (*pb.ShuffleResponse, error) {
	// Lock the whole thing during shuffle
	p.dataLock.Lock()
	defer p.dataLock.Unlock()
	// Some validation
	if p.sharedPrime == nil {
		return nil, fmt.Errorf("Never provided shared prime")
	}
	if len(req.UnencryptedStartCards) != len(req.WorkingCardSet) {
		return nil, fmt.Errorf("Working set not same size as orig set")
	}
	// Validate hand start sigs
	if p.lastGameStart == nil || p.lastHandStart == nil {
		return nil, fmt.Errorf("Missing game/hand start")
	} else if len(req.HandStartPlayerSigs) != len(p.lastGameStart.Players) {
		return nil, fmt.Errorf("Invalid hand start sigs")
	}
	handStartBytes, err := proto.Marshal(p.lastHandStart)
	if err != nil {
		return nil, err
	}
	for i, sig := range req.HandStartPlayerSigs {
		if !p.lastGameStart.Players[i].VerifySig(handStartBytes, sig) {
			return nil, fmt.Errorf("Invalid hand start sig")
		}
	}
	// Make sure these are the cards we expect...first deal is all 108, discard stack if deck is empty
	if p.lastEvent != nil && p.lastEvent.Hand != nil {
		if p.lastEvent.Hand.DeckCardsRemaining != 0 {
			return nil, fmt.Errorf("Reshuffling before deck is empty")
		} else if len(req.UnencryptedStartCards) != len(p.lastEvent.Hand.DiscardStack)-1 {
			return nil, fmt.Errorf("Expected reshuffle amount to be one less than discard")
		}
		for i, discard := range p.lastEvent.Hand.DiscardStack[:len(p.lastEvent.Hand.DiscardStack)-1] {
			if game.Card(req.UnencryptedStartCards[i]) != discard {
				return nil, fmt.Errorf("Invalid card to shuffle")
			}
		}
	} else if len(req.UnencryptedStartCards) != 108 {
		return nil, fmt.Errorf("Expected all 108 unencrypted cards")
	} else {
		for i := 0; i < 108; i++ {
			if req.UnencryptedStartCards[i] != uint32(i) {
				return nil, fmt.Errorf("First set of cards not in 0-107 order")
			}
		}
	}
	// Do the stages
	switch req.Stage {
	case 0:
		// Create stage 0 pair
		if p.shuffleStage0Pair, err = sra.GenerateKeyPair(rand.Reader, p.sharedPrime, sraKeyPairBits); err != nil {
			return nil, err
		}
		// Encrypt all the cards
		resp := &pb.ShuffleResponse{WorkingCardSet: make([][]byte, len(req.WorkingCardSet))}
		for i, workingCard := range req.WorkingCardSet {
			resp.WorkingCardSet[i] = p.shuffleStage0Pair.EncryptInt(new(big.Int).SetBytes(workingCard)).Bytes()
		}
		// Shuffle em
		crypto.NewCryptoRand().Shuffle(len(resp.WorkingCardSet), func(i, j int) {
			resp.WorkingCardSet[i], resp.WorkingCardSet[j] = resp.WorkingCardSet[j], resp.WorkingCardSet[i]
		})
		return resp, nil
	case 1:
		if p.shuffleStage0Pair == nil {
			return nil, fmt.Errorf("Haven't run stage 0")
		}
		// Decrypt each card and re-encrypt with specific encryption key
		resp := &pb.ShuffleResponse{WorkingCardSet: make([][]byte, len(req.WorkingCardSet))}
		p.shuffleStage1Pairs = make([]*sra.KeyPair, len(req.WorkingCardSet))
		for i, workingCard := range req.WorkingCardSet {
			// Generate key pair for card
			pair, err := sra.GenerateKeyPair(rand.Reader, p.sharedPrime, sraKeyPairBits)
			if err != nil {
				return nil, err
			}
			p.shuffleStage1Pairs[i] = pair
			// Decrypt other key, re-encrypt with this per-card one
			resp.WorkingCardSet[i] = pair.EncryptInt(p.shuffleStage0Pair.DecryptInt(
				new(big.Int).SetBytes(workingCard))).Bytes()
		}
		p.shuffleStage0Pair = nil
		return resp, nil
	case 2:
		if len(p.shuffleStage1Pairs) != len(req.WorkingCardSet) {
			return nil, fmt.Errorf("Haven't run stage 1")
		}
		// Just store a mapping of each of our pairs to the encrypted card
		p.encryptedDeckCards = make([]*big.Int, len(req.WorkingCardSet))
		for i, workingCard := range req.WorkingCardSet {
			card := new(big.Int).SetBytes(workingCard)
			// This appends/overwrites instead of completely replaces the map by intention
			p.cardPairs[card.String()] = p.shuffleStage1Pairs[i]
			p.encryptedDeckCards[i] = card
		}
		p.shuffleStage1Pairs = nil
		return &pb.ShuffleResponse{}, nil
	default:
		return nil, fmt.Errorf("Unrecognized stage")
	}
}

func (p *handler) ChooseColorSinceFirstCardIsWild(
	ctx context.Context, req *pb.ChooseColorSinceFirstCardIsWildRequest,
) (*pb.ChooseColorSinceFirstCardIsWildResponse, error) {
	p.dataLock.RLock()
	myIndex := p.myIndex
	lastGameStart := p.lastGameStart
	lastHandStart := p.lastHandStart
	lastEvent := p.lastEvent
	p.dataLock.RUnlock()
	if lastGameStart == nil || lastHandStart == nil {
		return nil, fmt.Errorf("Missing game/hand start")
	} else if lastEvent.Type != game.EventHandStartCardDealt {
		return nil, fmt.Errorf("Expected last event to be card deal")
	}
	// Check that the player after the dealer is me
	indexAfterDealer := int(lastHandStart.DealerIndex) + 1
	if indexAfterDealer >= len(lastGameStart.Players) {
		indexAfterDealer = 0
	}
	if indexAfterDealer != myIndex {
		return nil, fmt.Errorf("I am not the first player to go")
	}
	// Ask
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	color, err := p.ui.ChooseColorSinceFirstCardIsWild(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ChooseColorSinceFirstCardIsWildResponse{Color: uint32(color)}, nil
}

func (p *handler) GetDeckTopDecryptionKey(
	ctx context.Context, req *pb.GetDeckTopDecryptionKeyRequest,
) (*pb.GetDeckTopDecryptionKeyResponse, error) {
	// Locked throughout
	p.dataLock.Lock()
	defer p.dataLock.Unlock()
	// Get key and pop card
	encCard := p.encryptedDeckCards[len(p.encryptedDeckCards)-1]
	encCardStr := encCard.String()
	pair := p.cardPairs[encCardStr]
	if pair == nil {
		return nil, fmt.Errorf("Unable to find card pair")
	}
	p.encryptedDeckCards = p.encryptedDeckCards[:len(p.encryptedDeckCards)-1]
	// If it's for nobody (-1), then make sure the discard stack is empty or only full of wild draw fours
	if req.ForPlayerIndex == -1 {
		for _, card := range p.lastEvent.Hand.DiscardStack {
			if card.Value() != game.WildDrawFour {
				return nil, fmt.Errorf("Unexpected card on discard when popping for first go")
			}
		}
	} else if req.ForPlayerIndex == int32(p.myIndex) {
		return nil, fmt.Errorf("Asked for decryption key for myself")
	} else if _, ok := p.encryptedCardsGivenToPlayers[encCardStr]; ok {
		return nil, fmt.Errorf("Card already given to another player")
	} else {
		// Since it's for someone else, just mark it as such, we'll check at end of game
		p.encryptedCardsGivenToPlayers[encCardStr] = int(req.ForPlayerIndex)
	}
	// Give the key
	return &pb.GetDeckTopDecryptionKeyResponse{DecryptionKey: pair.Dec.Bytes()}, nil
}

func (p *handler) GiveDeckTopCard(
	ctx context.Context, req *pb.GiveDeckTopCardRequest,
) (*pb.GiveDeckTopCardResponse, error) {
	p.dataLock.Lock()
	myIndex := p.myIndex
	sharedPrime := p.sharedPrime
	// Get key and pop card
	encCard := p.encryptedDeckCards[len(p.encryptedDeckCards)-1]
	encCardStr := encCard.String()
	pair := p.cardPairs[encCardStr]
	p.encryptedDeckCards = p.encryptedDeckCards[:len(p.encryptedDeckCards)-1]
	_, previouslyGiven := p.encryptedCardsGivenToPlayers[encCardStr]
	p.encryptedCardsGivenToPlayers[encCardStr] = myIndex
	p.dataLock.Unlock()
	if pair == nil {
		return nil, fmt.Errorf("Unable to find card pair")
	} else if previouslyGiven {
		return nil, fmt.Errorf("Card already given out")
	}
	// Just take it, we'll have the event handler check normal game state
	// Decrypt the card with all keys
	myCard := &myCardInfo{encryptedCard: encCard, decryptionKeys: make([]*big.Int, len(req.DecryptionKeys))}
	for i, otherDecKey := range req.DecryptionKeys {
		// My index should be empty and I'll use my pair
		if i == myIndex {
			if len(otherDecKey) != 0 {
				return nil, fmt.Errorf("A key was given for my index")
			}
			myCard.decryptionKeys[i] = pair.Dec
			encCard = pair.DecryptInt(encCard)
		} else if len(otherDecKey) == 0 {
			return nil, fmt.Errorf("Missing decryption key")
		} else {
			decKey := new(big.Int).SetBytes(otherDecKey)
			myCard.decryptionKeys[i] = decKey
			encCard = sra.DecryptInt(sharedPrime, decKey, encCard)
		}
	}
	if encCard.BitLen() > 32 {
		return nil, fmt.Errorf("Invalid card decryption")
	}
	myCard.card = game.Card(int(encCard.Int64()))
	if !myCard.card.Valid() {
		return nil, fmt.Errorf("Invalid card")
	}
	// Add the card
	p.dataLock.Lock()
	p.myCards = append(p.myCards, myCard)
	p.dataLock.Unlock()
	// Send downstream
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	if err := p.ui.ReceiveCard(ctx, myCard.card); err != nil {
		return nil, err
	}
	return &pb.GiveDeckTopCardResponse{}, nil
}

func (p *handler) Play(ctx context.Context, req *pb.PlayRequest) (*pb.PlayResponse, error) {
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	// Ask first
	card, wildColor, err := p.ui.Play(ctx)
	if err != nil {
		return nil, err
	} else if card.Wild() {
		wildColor = 0
	}
	// Lock the rest of the way
	p.dataLock.Lock()
	defer p.dataLock.Unlock()
	// Find the card info based on the card
	var myCard *myCardInfo
	myCardIndex := -1
	for i, c := range p.myCards {
		if c.card == card {
			myCard = c
			myCardIndex = i
			break
		}
	}
	if myCard == nil {
		return nil, fmt.Errorf("Invalid card")
	}
	p.myCards = append(p.myCards[:myCardIndex], p.myCards[myCardIndex+1:]...)
	resp := &pb.PlayResponse{
		EncryptedCard:      myCard.encryptedCard.Bytes(),
		UnencryptedCard:    uint32(myCard.card),
		CardDecryptionKeys: make([][]byte, len(myCard.decryptionKeys)),
		WildColor:          uint32(wildColor),
	}
	for i, decKey := range myCard.decryptionKeys {
		resp.CardDecryptionKeys[i] = decKey.Bytes()
	}
	return resp, nil
}

func (p *handler) ShouldChallengeWildDrawFour(
	ctx context.Context, req *pb.ShouldChallengeWildDrawFourRequest,
) (*pb.ShouldChallengeWildDrawFourResponse, error) {
	// Make sure color is the last wild color
	p.dataLock.RLock()
	lastEvent := p.lastEvent
	p.dataLock.RUnlock()
	if lastEvent == nil || lastEvent.Hand == nil || uint32(lastEvent.Hand.LastDiscardWildColor) != req.PrevColor {
		return nil, fmt.Errorf("Invalid color")
	}
	// Ask
	ctx, cancelFn := context.WithTimeout(ctx, maxIfaceHandleTime)
	defer cancelFn()
	var err error
	resp := &pb.ShouldChallengeWildDrawFourResponse{}
	if resp.Challenge, err = p.ui.ShouldChallengeWildDrawFour(); err != nil {
		return nil, err
	}
	return resp, nil
}

func (p *handler) RevealCardsForChallenge(
	ctx context.Context, req *pb.RevealCardsForChallengeRequest,
) (*pb.RevealCardsForChallengeResponse, error) {
	panic("TODO")
}

func (p *handler) RevealedCardsForChallenge(
	ctx context.Context, req *pb.RevealedCardsForChallengeRequest,
) (*pb.RevealedCardsForChallengeResponse, error) {
	panic("TODO")
}
