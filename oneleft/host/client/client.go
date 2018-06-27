package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/cretz/one-left/oneleft/pb"
)

type Client struct {
	num            uint64
	handler        ClientRequestHandler
	stream         pb.Host_StreamServer
	maxRPCWaitTime time.Duration

	chLock           sync.RWMutex
	sendCh           chan *pb.HostMessage
	terminatingErrCh chan error

	reqRespLock       sync.Mutex
	receivedRespValCh chan<- *pb.ClientMessage_PlayerResponse
	receivedRespErrCh chan<- error
}

type ClientRequestHandler interface {
	OnRun(*Client)
	OnChatMessage(*Client, *pb.ChatMessage)
	OnStartJoin(*Client)
	OnStop(*Client)
}

var clientNumCounterLock sync.Mutex
var clientNumCounter uint64

func nextClientNum() uint64 {
	clientNumCounterLock.Lock()
	defer clientNumCounterLock.Unlock()
	clientNumCounter++
	return clientNumCounter
}

func New(
	handler ClientRequestHandler,
	stream pb.Host_StreamServer,
	maxRPCWaitTime time.Duration,
) *Client {
	return &Client{
		num:            nextClientNum(),
		handler:        handler,
		stream:         stream,
		maxRPCWaitTime: maxRPCWaitTime,
	}
}

func (c *Client) Num() uint64 { return c.num }
func (c *Client) Running() bool {
	c.chLock.RLock()
	defer c.chLock.RUnlock()
	return c.sendCh != nil
}

func (c *Client) Run() error {
	// Create channels
	c.chLock.Lock()
	if c.sendCh == nil {
		c.chLock.Unlock()
		return fmt.Errorf("Already running")
	}
	c.sendCh = make(chan *pb.HostMessage)
	c.terminatingErrCh = make(chan error)
	c.chLock.Unlock()
	recvMsgCh := make(chan *pb.ClientMessage)
	recvErrCh := make(chan error)
	// Close the chans when done
	defer func() {
		c.chLock.Lock()
		defer c.chLock.Unlock()
		close(c.sendCh)
		c.sendCh = nil
		close(c.terminatingErrCh)
		c.terminatingErrCh = nil
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
	// Let the handler know we're running
	go c.handler.OnRun(c)
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
			case *pb.ClientMessage_ChatMessage:
				go c.handler.OnChatMessage(c, recvMsg.ChatMessage)
			case *pb.ClientMessage_StartJoin:
				go c.handler.OnStartJoin(c)
			case *pb.ClientMessage_PlayerResponse_:
				c.reqRespLock.Lock()
				rcpRespCh := c.receivedRespValCh
				c.receivedRespValCh = nil
				c.reqRespLock.Unlock()
				if rcpRespCh == nil {
					err = fmt.Errorf("Sent RCP response without request")
					break MainLoop
				}
				rcpRespCh <- recvMsg.PlayerResponse
			default:
				err = fmt.Errorf("Unrecognized message type: %T", recvMsg)
				break MainLoop
			}
		case err = <-recvErrCh:
			break MainLoop
		case err = <-c.terminatingErrCh:
			// Before terminating, let's try to send an error
			// TODO
			break MainLoop
		}
	}
	c.reqRespLock.Lock()
	rcpRespCh := c.receivedRespErrCh
	c.receivedRespErrCh = nil
	c.reqRespLock.Unlock()
	if rcpRespCh != nil {
		rcpRespCh <- err
	}
	return err
}

func (c *Client) SendNonBlocking(msg *pb.HostMessage) error {
	c.chLock.RLock()
	defer c.chLock.RUnlock()
	if c.sendCh == nil {
		return fmt.Errorf("Send channel gone")
	}
	go func(ch chan *pb.HostMessage) { ch <- msg }(c.sendCh)
	return nil
}

func (c *Client) FailNonBlocking(err error) error {
	c.chLock.RLock()
	defer c.chLock.RUnlock()
	if c.terminatingErrCh == nil {
		return fmt.Errorf("Termination channel gone")
	}
	go func(ch chan error) { ch <- err }(c.terminatingErrCh)
	return nil
}
