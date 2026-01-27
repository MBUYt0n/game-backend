package server

import (
	"google.golang.org/grpc"
	"game-backend/protos"
	"net"
	"log"
	"game-backend/server/game_state"

)


type GameServiceServer struct {
	protos.UnimplementedGameServiceServer
}
func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	protos.RegisterGameServiceServer(grpcServer, &GameServiceServer{})
	grpcServer.Serve(lis)
}

func (s* GameServiceServer) Connect(req *protos.ClientEvent, stream grpc.ServerStreamingServer[protos.ServerEvent]) error {
	for {
		event, err := stream.Recv()
		if err == stream.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch event.Event.(type) {
		case *protos.ClientEvent_GameJoin:

		}
	}
}