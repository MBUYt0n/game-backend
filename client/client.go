package main

import (
	"bufio"
	"context"
	"fmt"
	"game-backend/protos"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func NewGameClient(address string) (*GameClient, error) {
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
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

func (gc *GameClient) Connect() error {
	stream, err := gc.client.Connect(context.Background())
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

func runClientCli(address string, id string) {
	client, err := NewGameClient(address)
	if err != nil {
		log.Fatalf("Could not create client: %v", err)
	}
	defer client.Close()

	err = client.Connect()
	if err != nil {
		log.Fatalf("Could not connect to server: %v", err)
	}
	pp := &PlayerPositions{}
	pp.Positions = make(map[string][2]int32)

	go func() {
		for {
			event, err := client.ReceiveEvent()
			if err != nil {
				log.Printf("Error receiving event: %v", err)
				return
			}
			// log.Printf("Received event: %v", event)
			handleServerEvent(event, pp)

		}
	}()
	event := ClientGameJoin(id)
	err = client.SendEvent(event)
	if err != nil {
		log.Printf("Error sending event: %v", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter commands: 'move <dx> <dy>' to move, 'leave' to exit")
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		parts := strings.Split(input, " ")
		if len(parts) == 0 {
			continue
		}
		command := parts[0]
		if command == "move" && len(parts) == 3 {
			dx, errX := strconv.Atoi(parts[1])
			dy, errY := strconv.Atoi(parts[2])
			if errX != nil || errY != nil {
				fmt.Println("Invalid move command. Use 'move <dx> <dy>'")
				continue
			}
			event = ClientGameMove(id, int32(dx), int32(dy))
			err = client.SendEvent(event)
			if err != nil {
				log.Printf("Error sending move event: %v", err)
			}
		} else if command == "leave" {
			event = ClientGameLeave()
			err = client.SendEvent(event)
			if err != nil {
				log.Printf("Error sending leave event: %v", err)
			}
			break
		} else {
			fmt.Println("Unknown command. Use 'move <dx> <dy>' or 'leave'")
		}
	}

	time.Sleep(2 * time.Second)
}

func runClient(address string, id string) {
	client, err := NewGameClient(address)
	if err != nil {
		log.Fatalf("Could not create client: %v", err)
	}
	defer client.Close()

	err = client.Connect()
	if err != nil {
		log.Fatalf("Could not connect to server: %v", err)
	} else {
		log.Printf("Client %s connected to server at %s", id, address)
	}

	pp := &PlayerPositions{}
	pp.Positions = make(map[string][2]int32)

	event := ClientGameJoin(id)
	err = client.SendEvent(event)
	if err != nil {
		log.Printf("Error sending event: %v", err)
	}

	// Wait for join acknowledgement
	ackReceived := false
	for !ackReceived {
		event, err := client.ReceiveEvent()
		if err != nil {
			log.Fatalf("Error receiving join ack: %v", err)
		}
		if joinEvent, ok := event.Event.(*protos.ServerEvent_GameJoin); ok && joinEvent.GameJoin.PlayerState.PlayerId == id {
			log.Printf("Join acknowledged for player %s", id)
			handleServerEvent(event, pp) // Process the ack
			ackReceived = true
		} else if _, ok := event.Event.(*protos.ServerEvent_Snapshot); ok {
			handleServerEvent(event, pp)
		}
	}

	go func() {
		for {
			event, err := client.ReceiveEvent()
			if err != nil {
				log.Printf("Error receiving event: %v", err)
				return
			}
			handleServerEvent(event, pp)
		}
	}()

	go func() {
		for range 1000 { // Send 1000 moves, adjust as needed
			dx := int32(rand.Intn(3) - 1) // Random -1, 0, 1
			dy := int32(rand.Intn(3) - 1)
			event := ClientGameMove(id, dx, dy)
			err := client.SendEvent(event)
			if err != nil {
				log.Printf("Error sending move event: %v", err)
			}
			time.Sleep(10 * time.Millisecond)
		}
		event := ClientGameLeave()
		client.SendEvent(event)
	}()

	time.Sleep(15 * time.Second) // Wait for simulation to complete
}

func Matchmaking() (string, string) {
	conn, err := grpc.NewClient("matchmaking:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	address, id := Matchmaking()
	log.Printf("Assigned to server %s with player ID %s", address, id)
	runClient(address, id)
}
