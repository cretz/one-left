package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/cretz/one-left/oneleft/pb"
)

type Client interface {
	Running() bool
	Run() error
	SendNonBlocking(*pb.ClientMessage) error
	FailNonBlocking(error) error
}

type RequestHandler interface {
	pb.PlayerServer

	OnRun(context.Context) error
	OnWelcome(context.Context, *pb.HostMessage_Welcome) error
	OnPlayersUpdate(context.Context, *pb.HostMessage_Players) error
	OnChatMessage(context.Context, *pb.ChatMessage) error
	OnGameEvent(context.Context, *pb.HostMessage_GameEvent) error
	OnError(context.Context, *pb.HostMessage_Error) error
}

type client struct {
	handler RequestHandler
	stream  pb.Host_StreamClient

	chLock           sync.RWMutex
	sendCh           chan *pb.ClientMessage
	terminatingErrCh chan error
}

func New(handler RequestHandler, stream pb.Host_StreamClient) Client {
	return &client{handler: handler, stream: stream}
}

func (c *client) Running() bool {
	c.chLock.RLock()
	defer c.chLock.RUnlock()
	return c.sendCh != nil
}

func (c *client) Run() error {
	defer c.stream.CloseSend()
	// Create channels
	c.chLock.Lock()
	if c.sendCh != nil {
		c.chLock.Unlock()
		return fmt.Errorf("Already running or have run")
	}
	c.sendCh = make(chan *pb.ClientMessage)
	c.terminatingErrCh = make(chan error)
	c.chLock.Unlock()
	recvMsgCh := make(chan *pb.HostMessage)
	recvErrCh := make(chan error)
	// Close the chans when done
	defer func() {
		c.chLock.Lock()
		defer c.chLock.Unlock()
		close(c.sendCh)
		close(c.terminatingErrCh)
		close(recvMsgCh)
		close(recvErrCh)
	}()
	// Receive messages asynchronously
	go func() {
		for {
			if msg, err := c.stream.Recv(); err != nil {
				recvErrCh <- err
				break
			} else {
				recvMsgCh <- msg
			}
		}
	}()
	// Notify start
	err := c.handler.OnRun(c.stream.Context())
	// Stream requests and responses
	for err == nil {
		select {
		case sendMsg := <-c.sendCh:
			err = c.stream.Send(sendMsg)
		case recvMsg := <-recvMsgCh:
			switch recvMsg := recvMsg.Message.(type) {
			case *pb.HostMessage_Welcome_:
				err = c.handler.OnWelcome(c.stream.Context(), recvMsg.Welcome)
			case *pb.HostMessage_PlayersUpdate:
				err = c.handler.OnPlayersUpdate(c.stream.Context(), recvMsg.PlayersUpdate)
			case *pb.HostMessage_ChatMessageAdded:
				err = c.handler.OnChatMessage(c.stream.Context(), recvMsg.ChatMessageAdded)
			case *pb.HostMessage_GameEvent_:
				err = c.handler.OnGameEvent(c.stream.Context(), recvMsg.GameEvent)
			case *pb.HostMessage_Error_:
				err = c.handler.OnError(c.stream.Context(), recvMsg.Error)
			case *pb.HostMessage_PlayerRequest_:
				err = c.doRPC(c.stream.Context(), recvMsg.PlayerRequest)
			default:
				err = fmt.Errorf("Unrecognized message type: %T", recvMsg)
			}
		case err = <-recvErrCh:
		case err = <-c.terminatingErrCh:
		}
	}
	// TODO: we should log this
	return err
}

func (c *client) SendNonBlocking(msg *pb.ClientMessage) error {
	c.chLock.RLock()
	defer c.chLock.RUnlock()
	if c.sendCh == nil {
		return fmt.Errorf("Not running")
	}
	go func(ch chan *pb.ClientMessage) { ch <- msg }(c.sendCh)
	return nil
}

func (c *client) FailNonBlocking(err error) error {
	c.chLock.RLock()
	defer c.chLock.RUnlock()
	if c.terminatingErrCh == nil {
		return fmt.Errorf("Not running")
	}
	go func(ch chan error) { ch <- err }(c.terminatingErrCh)
	return nil
}
