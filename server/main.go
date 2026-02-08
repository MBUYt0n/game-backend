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
	lis, err := net.Listen("tcp4", ":7777")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	room = game_state.NewRoom()
	go room.Run()
	log.Println("Server is running on port :7777")

	grpcServer := grpc.NewServer()
	protos.RegisterGameServiceServer(grpcServer, &GameServiceServer{})

	go func() {
		<-room.Done
		log.Println("Room ended, shutting down server")
		grpcServer.GracefulStop()
	}()

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

	playerId := join.PlayerId

	playerConn := &game_state.PlayerConn{
		Id:    playerId,
		Send:  make(chan *protos.ServerEvent, 16),
		Close: make(chan struct{}),
	}

	log.Printf("Player %s connected", playerId)

	if existing, ok := room.Players[playerId]; ok {
		close(existing.Close)
		delete(room.Players, playerId)

		room.Players[playerId] = playerConn

		snapshot := game_state.GetGameState(room)
		playerConn.Send <- snapshot
	} else {
		room.Join <- playerConn
	}

	ctx := stream.Context()
	go func() {
		<-ctx.Done()
		room.Leave <- playerConn
	}()

	go func() {
		defer func() {
			room.Leave <- playerConn
		}()

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
