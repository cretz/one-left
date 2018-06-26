package host

import (
	"fmt"

	"github.com/cretz/one-left/oneleft/pb"
)

type Client struct {
	Num              int64
	toSendCh         chan *pb.HostMessage
	terminatingErrCh chan error
	playerIndex      int
}

func newClient(num int64) *Client {
	return &Client{
		Num:              num,
		toSendCh:         make(chan *pb.HostMessage),
		terminatingErrCh: make(chan error),
		playerIndex:      -1,
	}
}

func (c *Client) Close() {
	close(c.toSendCh)
	close(c.terminatingErrCh)
}

func (c *Client) sendNonBlocking(msg *pb.HostMessage) {
	go func() { c.toSendCh <- msg }()
}

func (h *Host) Stream(stream pb.Host_StreamServer) error {
	// Send welcome
	if err := h.SendWelcome(stream); err != nil {
		return err
	}
	// Add to client list and remove on finish
	client := h.createAndAddClient()
	defer h.removeAndCloseClient(client)
	// Receive messages asynchronously
	recvMsgCh := make(chan *pb.ClientMessage)
	recvErrCh := make(chan error)
	go func() {
		for {
			if msg, err := stream.Recv(); err != nil {
				recvErrCh <- err
				break
			} else {
				recvMsgCh <- msg
			}
		}
	}()
	// Run until there's an error
	for {
		select {
		case err := <-client.terminatingErrCh:
			return err
		case err := <-recvErrCh:
			return err
		// TODO: case err := <-client.someExternalErrCh
		case msg := <-recvMsgCh:
			switch msg := msg.Message.(type) {
			case *pb.ClientMessage_JoinRequest:
				if err := h.join(client, msg.JoinRequest); err != nil {
					client.toSendCh <- &pb.HostMessage{Message: &pb.HostMessage_Error{
						Error: fmt.Sprintf("Failed to join: %v", err),
					}}
				}
				// TODO: more
			}
		case msg := <-client.toSendCh:
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *Host) SendWelcome(stream pb.Host_StreamServer) error {
	return stream.Send(&pb.HostMessage{Message: &pb.HostMessage_Welcome_{&pb.HostMessage_Welcome{
		Players:      h.ProtoPlayers(),
		GameUpdate:   h.GameUpdate(),
		ChatMessages: h.chatMessages,
	}}})
}
