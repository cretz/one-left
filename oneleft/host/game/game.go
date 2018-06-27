package game

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"

	"github.com/cretz/one-left/oneleft/game"
	"github.com/cretz/one-left/oneleft/pb"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
)

type Game struct {
	id           uuid.UUID
	players      []*clientPlayer
	eventHandler EventHandler

	// Nothing below is ever mutated, always replaced
	dataLock          sync.RWMutex
	deck              *deck
	running           bool
	lastEvent         *pb.HostMessage_GameEvent
	lastGameStartSigs [][]byte
	lastHandEndSigs   [][]byte
}

type EventHandler interface {
	OnEvent(*pb.HostMessage_GameEvent) error
}

func New(eventHandler EventHandler, players []*PlayerInfo) *Game {
	ret := &Game{eventHandler: eventHandler, players: make([]*clientPlayer, len(players))}
	var err error
	if ret.id, err = uuid.NewRandom(); err != nil {
		panic(err)
	}
	for index, playerInfo := range players {
		ret.players[index] = &clientPlayer{PlayerInfo: playerInfo, index: index, currGame: ret}
	}
	return ret
}

func (g *Game) Player(index int) *PlayerInfo {
	if index < 0 || index >= len(g.players) {
		return nil
	}
	return g.players[index].PlayerInfo
}

func (g *Game) Play() (*game.GameComplete, error) {
	// Mark as running (don't unmark when done)
	g.dataLock.Lock()
	if g.running {
		g.dataLock.Unlock()
		return nil, fmt.Errorf("Already running or already ran")
	}
	g.dataLock.Unlock()
	// Run the game
	gamePlayers := make([]game.Player, len(g.players))
	for i, p := range g.players {
		gamePlayers[i] = p
	}
	return game.New(gamePlayers, g.newDeck, g.onEvent).Play(0)
}

func (g *Game) topDiscardColor() (game.CardColor, error) {
	if g.lastEvent == nil || g.lastEvent.Hand == nil {
		return 0, fmt.Errorf("No hand")
	}
	topCard := game.Card(g.lastEvent.Hand.DiscardStack[len(g.lastEvent.Hand.DiscardStack)-1])
	color := topCard.Color()
	if topCard.Wild() {
		color = game.CardColor(g.lastEvent.Hand.LastDiscardWildColor)
	}
	return color, nil
}

func (g *Game) onEvent(event *game.Event) error {
	pbEvent := g.gameEventToPbEvent(event)
	g.dataLock.Lock()
	g.lastEvent = pbEvent
	g.dataLock.Unlock()
	// On game start and end, we confirm our players agree first
	switch event.Type {
	case game.EventGameStart:
		if err := g.doGameStart(); err != nil {
			return err
		}
	case game.EventGameEnd:
		if err := g.doGameEnd(); err != nil {
			return err
		}
	}
	// Send it off to the base handler
	return g.eventHandler.OnEvent(pbEvent)
}

func (g *Game) doGameStart() error {
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	// Build the request, send it off async, update sigs
	req := &pb.GameStartRequest{
		Id:      g.id[:],
		Players: make([]*pb.PlayerIdentity, len(g.players)),
	}
	for i, p := range g.players {
		req.Players[i] = p.Identity
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	// Send off request async and get sigs
	gameStartSigs := make([][]byte, len(g.players))
	errCh := make(chan error, len(g.players))
	var wg sync.WaitGroup
	for i, p := range g.players {
		wg.Add(1)
		go func(i int, p *clientPlayer) {
			defer wg.Done()
			resp, err := p.Client.GameStart(ctx, req)
			if err == nil {
				// Go ahead and verify the sig
				if p.VerifySig(reqBytes, resp.Sig) {
					gameStartSigs[i] = resp.Sig
					return
				}
				err = fmt.Errorf("Signature invalid")
			}
			errCh <- game.PlayerErrorf(i, "Game start err: %v", err)
		}(i, p)
	}
	// Wait for complete or err
	doneCh := make(chan struct{}, 1)
	go func() { wg.Wait(); doneCh <- struct{}{} }()
	select {
	case err := <-errCh:
		return err
	case <-doneCh:
	}
	// Set the sigs
	g.dataLock.Lock()
	g.lastGameStartSigs = gameStartSigs
	g.dataLock.Unlock()
	return nil
}

func (g *Game) doGameEnd() error {
	// Grab game info
	g.dataLock.RLock()
	lastEvent := g.lastEvent
	lastHandEndSigs := g.lastHandEndSigs
	g.dataLock.RUnlock()
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	// Build the request, send it off async, update sigs
	req := &pb.GameEndRequest{
		PlayerScores:          lastEvent.PlayerScores,
		LastHandEndPlayerSigs: lastHandEndSigs,
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	// Send off request async and check sigs
	errCh := make(chan error, len(g.players))
	var wg sync.WaitGroup
	for i, p := range g.players {
		wg.Add(1)
		go func(i int, p *clientPlayer) {
			defer wg.Done()
			resp, err := p.Client.GameEnd(ctx, req)
			if err == nil {
				// Go ahead and verify the sig
				if p.VerifySig(reqBytes, resp.Sig) {
					return
				}
				err = fmt.Errorf("Signature invalid")
			}
			errCh <- game.PlayerErrorf(i, "Hand start err: %v", err)
		}(i, p)
	}
	// Wait for complete or err
	doneCh := make(chan struct{}, 1)
	go func() { wg.Wait(); doneCh <- struct{}{} }()
	select {
	case err := <-errCh:
		return err
	case <-doneCh:
		return nil
	}
}

func (g *Game) doHandStart() (*deckInfo, error) {
	// Grab game info
	g.dataLock.RLock()
	gameStartSigs := g.lastGameStartSigs
	lastEvent := g.lastEvent
	g.dataLock.RUnlock()
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	// Build deck info
	ret := &deckInfo{}
	var err error
	if ret.handID, err = uuid.NewRandom(); err != nil {
		return nil, fmt.Errorf("Failed generating hand ID: %v", err)
	}
	if ret.sharedPrime, err = rand.Prime(rand.Reader, sharedPrimeBits); err != nil {
		return nil, fmt.Errorf("Failed generating shared prime: %v", err)
	}
	// Build the request, send it off async, update sigs
	req := &pb.HandStartRequest{
		Id:                  ret.handID[:],
		SharedCardPrime:     ret.sharedPrime.Bytes(),
		PlayerScores:        lastEvent.PlayerScores,
		DealerIndex:         lastEvent.DealerIndex + 1,
		GameStartPlayerSigs: gameStartSigs,
	}
	// Wrap the dealer index
	if req.DealerIndex == uint32(len(g.players)) {
		req.DealerIndex = 0
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	// Send off request async and get sigs
	ret.handStartSigs = make([][]byte, len(g.players))
	errCh := make(chan error, len(g.players))
	var wg sync.WaitGroup
	for i, p := range g.players {
		wg.Add(1)
		go func(i int, p *clientPlayer) {
			defer wg.Done()
			resp, err := p.Client.HandStart(ctx, req)
			if err == nil {
				// Go ahead and verify the sig
				if p.VerifySig(reqBytes, resp.Sig) {
					ret.handStartSigs[i] = resp.Sig
					return
				}
				err = fmt.Errorf("Signature invalid")
			}
			errCh <- game.PlayerErrorf(i, "Hand start err: %v", err)
		}(i, p)
	}
	// Wait for complete or err
	doneCh := make(chan struct{}, 1)
	go func() { wg.Wait(); doneCh <- struct{}{} }()
	select {
	case err := <-errCh:
		return nil, err
	case <-doneCh:
		return ret, nil
	}
}

func (g *Game) newDeck() (game.CardDeck, error) {
	info, err := g.doHandStart()
	if err != nil {
		return nil, err
	}
	return newDeck(g, info)
}

func (g *Game) MakePbError(err error) *pb.HostMessage_Error {
	return &pb.HostMessage_Error{
		GameId:         g.id[:],
		Message:        err.Error(),
		PlayerIndex:    int32(findErrPlayerIndex(err)),
		TerminatesGame: true,
	}
}

func findErrPlayerIndex(err error) int {
	if gameErr, ok := err.(*game.GameError); !ok {
		return -1
	} else if parentIndex := findErrPlayerIndex(gameErr.Cause); parentIndex != -1 {
		return parentIndex
	} else {
		return gameErr.PlayerIndex
	}
}
