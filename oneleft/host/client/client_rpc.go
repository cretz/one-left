package client

import (
	"context"
	"fmt"

	"github.com/cretz/one-left/oneleft/pb"
)

func (c *client) doRPC(ctx context.Context, req interface{}) (interface{}, error) {
	if !c.Running() {
		return nil, fmt.Errorf("Client not running")
	}
	sendMsg, err := hostMessageFromPlayerRequest(req)
	if err != nil {
		return nil, err
	}
	respValCh := make(chan *pb.ClientMessage_PlayerResponse, 1)
	respErrCh := make(chan error, 1)
	// Set them here, but they are nil'd out in the stream loop
	c.reqRespLock.Lock()
	c.receivedRespValCh = respValCh
	c.receivedRespErrCh = respErrCh
	c.reqRespLock.Unlock()
	// Send the request
	if err = c.SendNonBlocking(&pb.HostMessage{Message: &pb.HostMessage_PlayerRequest_{sendMsg}}); err != nil {
		return nil, err
	}
	// Wait for response
	ctx, cancelFn := context.WithTimeout(ctx, c.maxRPCWaitTime)
	defer cancelFn()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-respErrCh:
		return nil, err
	case resp := <-respValCh:
		return playerResponseFromMatchingRequest(req, resp)
	}
}

func (c *client) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.JoinResponse), nil
}

func (c *client) GameStart(ctx context.Context, req *pb.GameStartRequest) (*pb.GameStartResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.GameStartResponse), nil
}

func (c *client) GameEnd(ctx context.Context, req *pb.GameEndRequest) (*pb.GameEndResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.GameEndResponse), nil
}

func (c *client) HandStart(ctx context.Context, req *pb.HandStartRequest) (*pb.HandStartResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.HandStartResponse), nil
}

func (c *client) HandEnd(ctx context.Context, req *pb.HandEndRequest) (*pb.HandEndResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.HandEndResponse), nil
}

func (c *client) Shuffle(ctx context.Context, req *pb.ShuffleRequest) (*pb.ShuffleResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.ShuffleResponse), nil
}

func (c *client) ChooseColorSinceFirstCardIsWild(
	ctx context.Context, req *pb.ChooseColorSinceFirstCardIsWildRequest,
) (*pb.ChooseColorSinceFirstCardIsWildResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.ChooseColorSinceFirstCardIsWildResponse), nil
}

func (c *client) GetDeckTopDecryptionKey(
	ctx context.Context, req *pb.GetDeckTopDecryptionKeyRequest,
) (*pb.GetDeckTopDecryptionKeyResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.GetDeckTopDecryptionKeyResponse), nil
}

func (c *client) GiveDeckTopCard(
	ctx context.Context, req *pb.GiveDeckTopCardRequest,
) (*pb.GiveDeckTopCardResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.GiveDeckTopCardResponse), nil
}

func (c *client) Play(ctx context.Context, req *pb.PlayRequest) (*pb.PlayResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.PlayResponse), nil
}

func (c *client) ShouldChallengeWildDrawFour(
	ctx context.Context, req *pb.ShouldChallengeWildDrawFourRequest,
) (*pb.ShouldChallengeWildDrawFourResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.ShouldChallengeWildDrawFourResponse), nil
}

func (c *client) RevealCardsForChallenge(
	ctx context.Context, req *pb.RevealCardsForChallengeRequest,
) (*pb.RevealCardsForChallengeResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.RevealCardsForChallengeResponse), nil
}

func (c *client) RevealedCardsForChallenge(
	ctx context.Context, req *pb.RevealedCardsForChallengeRequest,
) (*pb.RevealedCardsForChallengeResponse, error) {
	resp, err := c.doRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.(*pb.RevealedCardsForChallengeResponse), nil
}

func hostMessageFromPlayerRequest(req interface{}) (*pb.HostMessage_PlayerRequest, error) {
	switch req := req.(type) {
	case *pb.JoinRequest:
		return &pb.HostMessage_PlayerRequest{Message: &pb.HostMessage_PlayerRequest_JoinRequest{req}}, nil
	case *pb.GameStartRequest:
		return &pb.HostMessage_PlayerRequest{Message: &pb.HostMessage_PlayerRequest_GameStartRequest{req}}, nil
	case *pb.HandStartRequest:
		return &pb.HostMessage_PlayerRequest{Message: &pb.HostMessage_PlayerRequest_HandStartRequest{req}}, nil
	case *pb.ShuffleRequest:
		return &pb.HostMessage_PlayerRequest{Message: &pb.HostMessage_PlayerRequest_ShuffleRequest{req}}, nil
	case *pb.ChooseColorSinceFirstCardIsWildRequest:
		return &pb.HostMessage_PlayerRequest{
			Message: &pb.HostMessage_PlayerRequest_ChooseColorSinceFirstCardIsWildRequest{req},
		}, nil
	case *pb.GetDeckTopDecryptionKeyRequest:
		return &pb.HostMessage_PlayerRequest{
			Message: &pb.HostMessage_PlayerRequest_GetDeckTopDecryptionKeyRequest{req},
		}, nil
	case *pb.GiveDeckTopCardRequest:
		return &pb.HostMessage_PlayerRequest{
			Message: &pb.HostMessage_PlayerRequest_GiveDeckTopCardRequest{req},
		}, nil
	case *pb.PlayRequest:
		return &pb.HostMessage_PlayerRequest{Message: &pb.HostMessage_PlayerRequest_PlayRequest{req}}, nil
	case *pb.ShouldChallengeWildDrawFourRequest:
		return &pb.HostMessage_PlayerRequest{
			Message: &pb.HostMessage_PlayerRequest_ShouldChallengeWildDrawFourRequest{req},
		}, nil
	case *pb.RevealCardsForChallengeRequest:
		return &pb.HostMessage_PlayerRequest{
			Message: &pb.HostMessage_PlayerRequest_RevealCardsForChallengeRequest{req},
		}, nil
	case *pb.RevealedCardsForChallengeRequest:
		return &pb.HostMessage_PlayerRequest{
			Message: &pb.HostMessage_PlayerRequest_RevealedCardsForChallengeRequest{req},
		}, nil
	default:
		return nil, fmt.Errorf("Unrecognized request type %T", req)
	}
}

func playerResponseFromMatchingRequest(req interface{}, resp *pb.ClientMessage_PlayerResponse) (interface{}, error) {
	var ret interface{}
	switch req.(type) {
	case *pb.JoinRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_JoinResponse); ok {
			ret = respMsg.JoinResponse
		}
	case *pb.GameStartRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_GameStartResponse); ok {
			ret = respMsg.GameStartResponse
		}
	case *pb.HandStartRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_HandStartResponse); ok {
			ret = respMsg.HandStartResponse
		}
	case *pb.ShuffleRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_ShuffleResponse); ok {
			ret = respMsg.ShuffleResponse
		}
	case *pb.ChooseColorSinceFirstCardIsWildRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_ChooseColorSinceFirstCardIsWildResponse); ok {
			ret = respMsg.ChooseColorSinceFirstCardIsWildResponse
		}
	case *pb.HostMessage_PlayerRequest_GetDeckTopDecryptionKeyRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_GetDeckTopDecryptionKeyResponse); ok {
			ret = respMsg.GetDeckTopDecryptionKeyResponse
		}
	case *pb.HostMessage_PlayerRequest_GiveDeckTopCardRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_GiveDeckTopCardResponse); ok {
			ret = respMsg.GiveDeckTopCardResponse
		}
	case *pb.PlayRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_PlayResponse); ok {
			ret = respMsg.PlayResponse
		}
	case *pb.ShouldChallengeWildDrawFourRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_ShouldChallengeWildDrawFourResponse); ok {
			ret = respMsg.ShouldChallengeWildDrawFourResponse
		}
	case *pb.RevealCardsForChallengeRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_RevealCardsForChallengeResponse); ok {
			ret = respMsg.RevealCardsForChallengeResponse
		}
	case *pb.RevealedCardsForChallengeRequest:
		if respMsg, ok := resp.Message.(*pb.ClientMessage_PlayerResponse_RevealedCardsForChallengeResponse); ok {
			ret = respMsg.RevealedCardsForChallengeResponse
		}
	}
	if ret == nil {
		return nil, fmt.Errorf("Response to %T was unrecognized %T", req, resp.Message)
	}
	return ret, nil
}
