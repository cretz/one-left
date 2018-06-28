package player

import (
	"context"

	"github.com/cretz/one-left/oneleft/pb"
	"github.com/golang/protobuf/proto"
)

func (p *handler) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	ident := &pb.PlayerIdentity{
		Id:          p.player.keyPair.PublicKey(),
		RandomNonce: req.RandomNonce,
		Name:        p.player.name,
	}
	identBytes, err := proto.Marshal(ident)
	if err != nil {
		return nil, err
	}
	ident.Sig = p.player.sign(identBytes)
	return &pb.JoinResponse{Player: ident}, nil
}

func (p *handler) GameStart(ctx context.Context, req *pb.GameStartRequest) (*pb.GameStartResponse, error) {
	panic("TODO")
}

func (p *handler) GameEnd(ctx context.Context, req *pb.GameEndRequest) (*pb.GameEndResponse, error) {
	panic("TODO")
}

func (p *handler) HandStart(ctx context.Context, req *pb.HandStartRequest) (*pb.HandStartResponse, error) {
	panic("TODO")
}

func (p *handler) HandEnd(ctx context.Context, req *pb.HandEndRequest) (*pb.HandEndResponse, error) {
	panic("TODO")
}

func (p *handler) Shuffle(ctx context.Context, req *pb.ShuffleRequest) (*pb.ShuffleResponse, error) {
	panic("TODO")
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
