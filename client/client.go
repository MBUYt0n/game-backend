package main

import (
	"context"
	"fmt"
	"game-backend/protos"
	"log"
	"math/rand"
	"time"
	"sync"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GameClient struct {
	conn   *grpc.ClientConn
	client protos.GameServiceClient
	stream protos.GameService_ConnectClient
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


func main() {
    var wg sync.WaitGroup
    numClients := 5

    for i := 0; i < numClients; i++ {
        wg.Add(1)
        go func(clientId int) {
            defer wg.Done()
            runClient(clientId)
        }(i)
    }

    wg.Wait()
}
// Example usage
func runClient(id int) {
	client, err := NewGameClient("localhost:50051")
	if err != nil {
		log.Fatalf("Could not create client: %v", err)
	}
	defer client.Close()

	err = client.Connect()
	if err != nil {
		log.Fatalf("Could not connect to server: %v", err)
	}

	// Example of sending and receiving events
	go func() {
		for {
			event, err := client.ReceiveEvent()
			if err != nil {
				log.Printf("Error receiving event: %v", err)
				return
			}
			log.Printf("Received event: %v", event)
		}
	}()
	event := ClientGameJoin(fmt.Sprintf("%d", id))
	err = client.SendEvent(event)
	if err != nil {
		log.Printf("Error sending event: %v", err)
	}

	var x int
	var y int

	for i := 0; i < 5; i++ {
		x = rand.Intn(3)
		y = rand.Intn(3)
		event := ClientGameMove(fmt.Sprintf("%d", id), int32(x), int32(y))
		err = client.SendEvent(event)
		if err != nil {
			log.Printf("Error sending event: %v", err)
		}
		time.Sleep(1 * time.Second)
	}
	event = ClientGameLeave()
	err = client.SendEvent(event)
	if err != nil {
		log.Printf("Error sending event: %v", err)
	}

	time.Sleep(2 * time.Second) // Wait to receive any final events
}
