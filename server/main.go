package main

import (
	"game-backend/protos"
	"game-backend/server/game_state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"log"
	"net"
)

var room *game_state.Room

type GameServiceServer struct {
	protos.UnimplementedGameServiceServer
}

func main() {
	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	room = game_state.NewRoom()
	go room.Run()
	log.Println("Server is running on port :50053")

	grpcServer := grpc.NewServer()
	protos.RegisterGameServiceServer(grpcServer, &GameServiceServer{})
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("grpc serve error: %v", err)
	}
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
		Send:  make(chan *protos.ServerEvent, 16),
		Close: make(chan struct{}),
	}

	room.Join <- playerConn

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
			room.Move <- e.IntentMessage

		case *protos.ClientEvent_GameExit:
			room.Leave <- playerConn
			return nil
		}
	}
}
