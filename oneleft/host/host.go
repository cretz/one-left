package host

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/cretz/one-left/oneleft/pb"
)

type Host struct {
	mutex         sync.RWMutex
	clientCounter int64
	clients       map[int64]*Client
	// Never mutated, always replaced
	players       []*pb.PlayerIdentity
	playerClients []*Client
	// Never mutated, always replaced
	chatMessages []*pb.ChatMessage
	// Never mutated, always replaced
	lastGameEvent *pb.HostMessage_GameEvent
}

func (h *Host) Stream(stream pb.Host_StreamServer) error {
	// Build welcome and add client at same time to prevent races
	h.mutex.Lock()
	h.clientCounter++
	c := newClient(h.clientCounter)
	h.clients[c.Num] = c
	welcomeMsg := &pb.HostMessage{Message: &pb.HostMessage_Welcome_{&pb.HostMessage_Welcome{
		Players:       h.players,
		ChatMessages:  h.chatMessages,
		LastGameEvent: h.lastGameEvent,
	}}}
	h.mutex.Unlock()
	// Defer removal of the client
	defer func() {
		// TODO: remove client and let host know because it may destroy the game
	}()
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
	// Send off welcome
	if err := stream.Send(welcomeMsg); err != nil {
		return nil
	}
	// Stream requests and responses
	var err error
MainLoop:
	for {
		select {
		case sendMsg := <-c.sendCh:
			if err = stream.Send(sendMsg); err != nil {
				break MainLoop
			}
		case recvMsg := <-recvMsgCh:
			switch recvMsg := recvMsg.Message.(type) {
			case *pb.ClientMessage_ChatMessage:
				if err = h.addChatMessage(c, recvMsg.ChatMessage); err != nil {
					break MainLoop
				}
			case *pb.ClientMessage_StartJoin:
				if err = h.startPlayerJoin(c); err != nil {
					break MainLoop
				}
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

func (h *Host) addChatMessage(client *Client, msg *pb.ChatMessage) error {
	// Eager player ID check
	playerID := client.PlayerID()
	if len(playerID) == 0 || !bytes.Equal(playerID, msg.PlayerId) {
		return fmt.Errorf("Only players can chat")
	}
	// Grab player from index to make sure it's still there and the right one
	h.mutex.RLock()
	var player *pb.PlayerIdentity
	if playerIndex := client.PlayerIndex(); playerIndex >= 0 && playerIndex < len(h.players) {
		player = h.players[playerIndex]
	}
	h.mutex.RUnlock()
	if player == nil || !bytes.Equal(player.Id, playerID) {
		return fmt.Errorf("Invalid player")
	}

	// TODO: check sig, do update, etc
	panic("TODO")
}

func (h *Host) startPlayerJoin(c *Client) error {
	panic("TODO")
}
