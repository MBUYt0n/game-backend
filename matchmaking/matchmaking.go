package main

import (
	"context"
	"fmt"
	"game-backend/protos"
	"log"
	"math/rand"
	"net"
	"google.golang.org/grpc"
)

type MatchMakingServiceServer struct {
	protos.UnimplementedMatchMakingServiceServer
}

type Player struct {
	id       string
	roomChan chan Room
}

type Room struct {
	players []Player
	address    string
}

const queueSize = 4

var joinQueue = make(chan Player, 100)

func main() {
	go matchmaker()

	lis, err := net.Listen("tcp4", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	protos.RegisterMatchMakingServiceServer(grpcServer, &MatchMakingServiceServer{})
	log.Println("Matchmaking server is running on port :50052")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve error: %v", err)
	}
}

func (s *MatchMakingServiceServer) Connect(
	ctx context.Context,
	req *protos.ClientQueueJoin,
) (*protos.MatchMakingResponse, error) {

	playerId := rand.Intn(1000)

	roomChan := make(chan Room, 1)
	player := Player{
		id:       fmt.Sprint(playerId),
		roomChan: roomChan,
	}

	joinQueue <- player
	log.Printf("Player %s joined the queue", player.id)

	select {
	case room := <-roomChan:
		log.Printf("Player %s assigned to room: ", player.id)
		return &protos.MatchMakingResponse{
			PlayerId:      player.id,
			ServerAddress: room.address,
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}

}

func matchmaker() {
	clientset, err := kubeClient()
	if err != nil {
		log.Fatalf("kube client error: %v", err)
	}

	var queue []Player

	for player := range joinQueue {
		queue = append(queue, player)

		if len(queue) >= queueSize {
			roomPlayers := queue[:queueSize]
			queue = queue[queueSize:]

			log.Printf("Creating game server for room %d", 1)

			err := createGameServerPod(clientset)
			if err != nil {
				log.Printf("pod error: %v", err)
				continue
			}

			address, err := createClusterIPService(clientset)
			if err != nil {
				log.Printf("svc error: %v", err)
				continue
			}

			err = waitForPodReady(clientset, "game-server-1")
			if err != nil {
				log.Printf("pod not ready: %v", err)
				continue
			}

			room := Room{
				address: address,
			}

			for _, p := range roomPlayers {
				p.roomChan <- room
				log.Printf("Assigned player %s to %s", p.id, address)
			}
		}
	}
}

