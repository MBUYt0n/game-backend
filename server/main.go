package main

import (
	"google.golang.org/grpc"
	"game-backend/protos"
	"net"
	"log"
	"game-backend/server/game_state"

)

var room *game_state.Room

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
	room = game_state.NewRoom()
	go room.Run()
	log.Println("Server is running on port :50051")
}

func (s *GameServiceServer) Connect(
	stream protos.GameService_ConnectServer,
) error {

	firstEvent, err := stream.Recv()
	if err != nil {
		return err
	}

	join := firstEvent.GetGameJoin()
	if join == nil {
		return status.Error(codes.InvalidArgument, "first event must be GameJoin")
	}

	playerConn := &game_state.PlayerConn{
		Id:    join.PlayerId,
		Send: make(chan *protos.ServerEvent, 16),
		Close: make(chan struct{}),
	}

	room.join <- playerConn

	go func() {
		for {
			select {
			case event := <-playerConn.Send:
				if err := stream.Send(event); err != nil {
					return
				}
			case <-playerConn.Close:
				return
			}
		}
	}()

	for {
		clientEvent, err := stream.Recv()
		if err == io.EOF {
			room.Leave <- playerConn
			return nil
		}
		if err != nil {
			room.Leave <- playerConn
			return err
		}

		switch e := clientEvent.Event.(type) {

		case *protos.ClientEvent_IntentMessage:
			room.Move <- *e.IntentMessage

		case *protos.ClientEvent_GameExit:
			room.Leave <- playerConn
			return nil
		}
	}
}
