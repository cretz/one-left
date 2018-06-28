package player

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

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
	// TODO: clear out old stuff
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
	// TODO: clear out old stuff
	// TODO check primality, min prime bits, etc
	panic("TODO")
}

func (p *handler) HandEnd(ctx context.Context, req *pb.HandEndRequest) (*pb.HandEndResponse, error) {
	panic("TODO")
}

func (p *handler) Shuffle(ctx context.Context, req *pb.ShuffleRequest) (*pb.ShuffleResponse, error) {
	p.dataLock.Lock()
	defer p.dataLock.Unlock()
	// Some validation
	if p.sharedPrime == nil {
		return nil, fmt.Errorf("Never provided shared prime")
	}
	if len(req.UnencryptedStartCards) != len(req.WorkingCardSet) {
		return nil, fmt.Errorf("Working set not same size as orig set")
	}
	// Make sure the hand start sigs are what we've seen
	if len(p.lastHandStartSigs) != len(req.HandStartPlayerSigs) {
		return nil, fmt.Errorf("Invalid hand start sig count")
	} else {
		for i, sig := range p.lastHandStartSigs {
			if !bytes.Equal(sig, req.HandStartPlayerSigs[i]) {
				return nil, fmt.Errorf("Invalid hand start sig")
			}
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
	var err error
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
		p.cardPairs = make(map[string]*sra.KeyPair, len(req.WorkingCardSet))
		for i, workingCard := range req.WorkingCardSet {
			p.cardPairs[new(big.Int).SetBytes(workingCard).String()] = p.shuffleStage1Pairs[i]
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
	panic("TODO")
}

func (p *handler) GetDeckTopDecryptionKey(
	ctx context.Context, req *pb.GetDeckTopDecryptionKeyRequest,
) (*pb.GetDeckTopDecryptionKeyResponse, error) {
	panic("TODO")
}

func (p *handler) GiveDeckTopCard(
	ctx context.Context, req *pb.GiveDeckTopCardRequest,
) (*pb.GiveDeckTopCardResponse, error) {
	panic("TODO")
}

func (p *handler) Play(ctx context.Context, req *pb.PlayRequest) (*pb.PlayResponse, error) {
	panic("TODO")
}

func (p *handler) ShouldChallengeWildDrawFour(
	ctx context.Context, req *pb.ShouldChallengeWildDrawFourRequest,
) (*pb.ShouldChallengeWildDrawFourResponse, error) {
	panic("TODO")
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
