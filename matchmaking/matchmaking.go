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
	port    int
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
			ServerAddress: fmt.Sprintf("server:%d", room.port),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}

}

func matchmaker() {
	var queue []Player
	var port = 50053
	for player := range joinQueue {
		queue = append(queue, player)

		if len(queue) >= queueSize {
			roomPlayers := queue[:queueSize]
			queue = queue[queueSize:]

			room := Room{players: roomPlayers, port: port}
			port++
			log.Printf("Created room on port %d with players: %v", room.port, room.players)
			for _, p := range roomPlayers {
				select {
				case p.roomChan <- room:
				default:
				}

			}
		}
	}
}
