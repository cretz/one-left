package client

import (
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

	OnRun(Client)
	OnWelcome(Client, *pb.HostMessage_Welcome)
	OnPlayersUpdate(Client, *pb.HostMessage_Players)
	OnChatMessage(Client, *pb.ChatMessage)
	OnGameEvent(Client, *pb.HostMessage_GameEvent)
	OnError(Client, *pb.HostMessage_Error)
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
	// Stream requests and responses
	var err error
MainLoop:
	for {
		select {
		case sendMsg := <-c.sendCh:
			if err = c.stream.Send(sendMsg); err != nil {
				break MainLoop
			}
		case recvMsg := <-recvMsgCh:
			switch recvMsg := recvMsg.Message.(type) {
			case *pb.HostMessage_Welcome_:
				go c.handler.OnWelcome(c, recvMsg.Welcome)
			case *pb.HostMessage_PlayersUpdate:
				go c.handler.OnPlayersUpdate(c, recvMsg.PlayersUpdate)
			case *pb.HostMessage_ChatMessageAdded:
				go c.handler.OnChatMessage(c, recvMsg.ChatMessageAdded)
			case *pb.HostMessage_GameEvent_:
				go c.handler.OnGameEvent(c, recvMsg.GameEvent)
			case *pb.HostMessage_Error_:
				go c.handler.OnError(c, recvMsg.Error)
			case *pb.HostMessage_PlayerRequest_:
				go c.doRPC(c.stream.Context(), recvMsg.PlayerRequest)
			default:
				err = fmt.Errorf("Unrecognized message type: %T", recvMsg)
				break MainLoop
			}
		case err = <-recvErrCh:
			break MainLoop
		case err = <-c.terminatingErrCh:
			break MainLoop
		}
	}
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
