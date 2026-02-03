package main

import (
	"context"
	"fmt"
	"game-backend/protos"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GameClient struct {
	conn   *grpc.ClientConn
	client protos.GameServiceClient
	stream protos.GameService_ConnectClient
}

type PlayerPositions struct {
	Positions map[string][2]int32
}

func (pp *PlayerPositions) UpdatePosition(playerId string, x, y int32, leaveOrUpdate string) {
	if leaveOrUpdate == "leave" {
		delete(pp.Positions, playerId)
	} else {
		pp.Positions[playerId] = [2]int32{x, y}
	}
}

func NewGameClient() (*GameClient, error) {
	conn, err := grpc.NewClient("matchmaking.local:30080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to server: %v", err)
		return nil, err
	}
	client := protos.NewGameServiceClient(conn)
	return &GameClient{
		conn:   conn,
		client: client,
	}, nil
}

func (gc *GameClient) CloseStream() {
	if gc.stream != nil {
		_ = gc.stream.CloseSend()
	}
}


func (gc *GameClient) Connect(roomID string) error {
	md := metadata.New(map[string]string{
		"x-room-id": roomID,
	})

	ctx := metadata.NewOutgoingContext(context.Background(), md)

	stream, err := gc.client.Connect(ctx)
	if err != nil {
		return err
	}

	gc.stream = stream
	return nil
}


func (gc *GameClient) SendEvent(event *protos.ClientEvent) error {
	return gc.stream.Send(event)
}

func (gc *GameClient) ReceiveEvent() (*protos.ServerEvent, error) {
	return gc.stream.Recv()
}

func (gc *GameClient) Close() error {
	return gc.conn.Close()
}

func ClientGameJoin(playerId string) *protos.ClientEvent {
	return &protos.ClientEvent{
		Event: &protos.ClientEvent_GameJoin{
			GameJoin: &protos.ClientGameJoin{
				PlayerId: playerId,
			},
		},
	}
}

func ClientGameMove(playerId string, dx, dy int32) *protos.ClientEvent {
	return &protos.ClientEvent{
		Event: &protos.ClientEvent_IntentMessage{
			IntentMessage: &protos.StateIntentMessage{
				PlayerId: playerId,
				Dx:       dx,
				Dy:       dy,
			},
		},
	}
}

func ClientGameLeave() *protos.ClientEvent {
	return &protos.ClientEvent{
		Event: &protos.ClientEvent_GameExit{
			GameExit: &protos.ClientGameExit{},
		},
	}
}

func clientMatchMakingRequest() *protos.ClientQueueJoin {
	return &protos.ClientQueueJoin{}
}

func displayGameState(pp *PlayerPositions) {
	fmt.Println("Current Player Positions:")
	for playerId, pos := range pp.Positions {
		fmt.Printf("Player %s: (%d, %d)\n", playerId, pos[0], pos[1])
	}
}

func handleServerEvent(event *protos.ServerEvent, pp *PlayerPositions) {
	switch e := event.Event.(type) {
	case *protos.ServerEvent_PlayerState:
		log.Printf("Player State Update: Id - %s, X: %v, Y: %v", e.PlayerState.PlayerId, e.PlayerState.X, e.PlayerState.Y)
		pp.UpdatePosition(e.PlayerState.PlayerId, e.PlayerState.X, e.PlayerState.Y, "update")
	case *protos.ServerEvent_GameJoin:
		log.Printf("Player Joined: %v", e.GameJoin.PlayerState.PlayerId)
		pp.UpdatePosition(e.GameJoin.PlayerState.PlayerId, e.GameJoin.PlayerState.X, e.GameJoin.PlayerState.Y, "update")
	case *protos.ServerEvent_GameExit:
		log.Printf("Player Exited: %v", e.GameExit.PlayerId)
		pp.UpdatePosition(e.GameExit.PlayerId, 0, 0, "leave")
	case *protos.ServerEvent_Snapshot:
		log.Printf("Game State Snapshot Received")
		for _, p := range e.Snapshot.PlayerState {
			log.Printf("Player %v is at position (%v, %v)", p.PlayerId, p.X, p.Y)
			pp.UpdatePosition(p.PlayerId, p.X, p.Y, "update")
		}
	default:
		log.Printf("Unknown event type")
	}
	displayGameState(pp)
}

func runClient(address string, id string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := NewGameClient()
	if err != nil {
		log.Fatalf("Could not create client: %v", err)
	}
	defer client.CloseStream()
	defer client.Close()

	if err := client.Connect(address); err != nil {
		log.Fatalf("Could not connect to server: %v", err)
	}
	log.Printf("Client %s connected to %s", id, address)

	pp := &PlayerPositions{Positions: make(map[string][2]int32)}

	// ---- Send JOIN ----
	if err := client.SendEvent(ClientGameJoin(id)); err != nil {
		log.Fatalf("Join failed: %v", err)
	}

	// ---- WAIT FOR FIRST RESPONSE (IMPORTANT) ----
	for {
		event, err := client.ReceiveEvent()
		if err != nil {
			log.Fatalf("Error receiving join ack: %v", err)
		}

		// Accept either join ack or snapshot
		if join, ok := event.Event.(*protos.ServerEvent_GameJoin); ok &&
			join.GameJoin.PlayerState.PlayerId == id {
			log.Printf("Join acknowledged for %s", id)
			handleServerEvent(event, pp)
			break
		}

		if _, ok := event.Event.(*protos.ServerEvent_Snapshot); ok {
			handleServerEvent(event, pp)
			break
		}
	}

	var wg sync.WaitGroup

	// ---- Receiver ----
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				event, err := client.ReceiveEvent()
				if err != nil {
					log.Printf("Receive error: %v", err)
					cancel()
					return
				}
				handleServerEvent(event, pp)
			}
		}
	}()

	// ---- Sender ----
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 1000; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				dx := int32(rand.Intn(3) - 1)
				dy := int32(rand.Intn(3) - 1)

				if err := client.SendEvent(ClientGameMove(id, dx, dy)); err != nil {
					log.Printf("Send error: %v", err)
					cancel()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}

		_ = client.SendEvent(ClientGameLeave())
		cancel()
	}()

	wg.Wait()
	log.Printf("Client %s exited cleanly", id)
}


func Matchmaking() (string, string) {
	conn, err := grpc.NewClient("matchmaking.local:30080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Could not connect to matchmaking server: %v", err)
	}
	defer conn.Close()

	client := protos.NewMatchMakingServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resp, err := client.Connect(ctx, clientMatchMakingRequest())
	if err != nil {
		log.Fatalf("Matchmaking request failed: %v", err)
	}
	log.Printf("Matchmaking response: PlayerId=%s, ServerAddress=%s", resp.PlayerId, resp.ServerAddress)
	return resp.ServerAddress, resp.PlayerId
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	roomID, id := Matchmaking()
	log.Printf("Assigned to server %s with player ID %s", roomID, id)

	done := make(chan struct{})
	time.Sleep(5 * time.Second)
	go func() {
		runClient(roomID, id)
		close(done)
	}()

	select {
	case <-ctx.Done():
		log.Println("Shutdown signal received")
	case <-done:
		log.Println("Client finished normally")
	}
}
