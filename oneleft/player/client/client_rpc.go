package client

import (
	"context"
	"fmt"

	"github.com/cretz/one-left/oneleft/pb"
)

func (c *client) doRPC(ctx context.Context, req *pb.HostMessage_PlayerRequest) {
	if err := c.makeRPCCall(ctx, req); err != nil {
		// Completely fail the entire client on even the slightest error
		c.FailNonBlocking(err)
	}
}

func (c *client) makeRPCCall(ctx context.Context, req *pb.HostMessage_PlayerRequest) error {
	switch msg := req.Message.(type) {
	case *pb.HostMessage_PlayerRequest_JoinRequest:
		resp, err := c.handler.Join(ctx, msg.JoinRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_GameStartRequest:
		resp, err := c.handler.GameStart(ctx, msg.GameStartRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_HandStartRequest:
		resp, err := c.handler.HandStart(ctx, msg.HandStartRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_ShuffleRequest:
		resp, err := c.handler.Shuffle(ctx, msg.ShuffleRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_ChooseColorSinceFirstCardIsWildRequest:
		resp, err := c.handler.ChooseColorSinceFirstCardIsWild(ctx, msg.ChooseColorSinceFirstCardIsWildRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_GetDeckTopDecryptionKeyRequest:
		resp, err := c.handler.GetDeckTopDecryptionKey(ctx, msg.GetDeckTopDecryptionKeyRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_GiveDeckTopCardRequest:
		resp, err := c.handler.GiveDeckTopCard(ctx, msg.GiveDeckTopCardRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_PlayRequest:
		resp, err := c.handler.Play(ctx, msg.PlayRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_ShouldChallengeWildDrawFourRequest:
		resp, err := c.handler.ShouldChallengeWildDrawFour(ctx, msg.ShouldChallengeWildDrawFourRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_RevealCardsForChallengeRequest:
		resp, err := c.handler.RevealCardsForChallenge(ctx, msg.RevealCardsForChallengeRequest)
		return c.sendRPCResponse(resp, err)
	case *pb.HostMessage_PlayerRequest_RevealedCardsForChallengeRequest:
		resp, err := c.handler.RevealedCardsForChallenge(ctx, msg.RevealedCardsForChallengeRequest)
		return c.sendRPCResponse(resp, err)
	default:
		return fmt.Errorf("Unrecognized message type: %T", msg)
	}
}

func (c *client) sendRPCResponse(resp interface{}, err error) error {
	if err != nil {
		return err
	}
	playerResp := &pb.ClientMessage_PlayerResponse{}
	switch resp := resp.(type) {
	case *pb.JoinResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_JoinResponse{resp}
	case *pb.GameStartResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_GameStartResponse{resp}
	case *pb.HandStartResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_HandStartResponse{resp}
	case *pb.ChooseColorSinceFirstCardIsWildResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_ChooseColorSinceFirstCardIsWildResponse{resp}
	case *pb.GetDeckTopDecryptionKeyResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_GetDeckTopDecryptionKeyResponse{resp}
	case *pb.GiveDeckTopCardResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_GiveDeckTopCardResponse{resp}
	case *pb.PlayResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_PlayResponse{resp}
	case *pb.ShouldChallengeWildDrawFourResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_ShouldChallengeWildDrawFourResponse{resp}
	case *pb.RevealCardsForChallengeResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_RevealCardsForChallengeResponse{resp}
	case *pb.RevealedCardsForChallengeResponse:
		playerResp.Message = &pb.ClientMessage_PlayerResponse_RevealedCardsForChallengeResponse{resp}
	default:
		return fmt.Errorf("Unrecognized client response type: %T", resp)
	}
	return c.SendNonBlocking(&pb.ClientMessage{Message: &pb.ClientMessage_PlayerResponse_{playerResp}})
}
