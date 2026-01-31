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
			roomID := fmt.Sprintf("%d", rand.Intn(1000000))
			roomPlayers := queue[:queueSize]
			queue = queue[queueSize:]

			log.Printf("Creating game server for room %s", roomID)

			err := createGameServerPod(clientset, roomID)
			if err != nil {
				log.Printf("pod error: %v", err)
				continue
			}

			nodePort, err := createNodePortService(clientset, roomID)
			if err != nil {
				log.Printf("svc error: %v", err)
				continue
			}

			err = waitForPodReady(clientset, "game-server-"+roomID)
			if err != nil {
				log.Printf("pod not ready: %v", err)
				continue
			}

			address := fmt.Sprintf("%s:%d", nodeAddress(), nodePort)

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

