package game_state

import (
	"game-backend/protos"
	"log"
	"math/rand"
)

type PlayerConn struct {
	Id    string
	Send  chan *protos.ServerEvent
	Close chan struct{}
}

type Room struct {
	Players map[string]*PlayerConn
	State   map[string]*protos.PlayerState

	Join     chan *PlayerConn
	Leave    chan *PlayerConn
	Move     chan *protos.StateIntentMessage
	Snapshot chan bool
	Done     chan struct{}
}

func NewRoom() *Room {
	return &Room{
		Players: make(map[string]*PlayerConn),
		State:   make(map[string]*protos.PlayerState),
		Join:    make(chan *PlayerConn, 2),
		Leave:   make(chan *PlayerConn, 2),
		Move:    make(chan *protos.StateIntentMessage, 2),
		Done:	make(chan struct{}),
	}
}

func (room *Room) Run() {
	var event *protos.ServerEvent
	for {
		select {
		case p := <-room.Join:
			event = room.addPlayerToGameState(p)
		case p := <-room.Leave:
			event = room.removePlayerFromGameState(p)
			if len(room.Players) == 0 {
				close(room.Done)
				log.Printf("Exiting room")
				return
			}
		case m := <-room.Move:
			event = room.movePlayer(m)
		}
		leaveEvents := room.broadcast(event)
		for _, e := range leaveEvents {
			room.broadcast(e)
		}
	}
}

func getGameState(room *Room) *protos.ServerEvent {

	players := make([]*protos.PlayerState, 0, len(room.State))
	for _, ps := range room.State {
		players = append(players, ps)
	}

	snapshot := protos.StateSnapshot{
		PlayerState: players,
	}

	serverEvent := protos.ServerEvent{
		Event: &protos.ServerEvent_Snapshot{
			Snapshot: &snapshot,
		},
	}
	return &serverEvent

}

func (room *Room) broadcast(event *protos.ServerEvent) []*protos.ServerEvent {
	var leaveEvents []*protos.ServerEvent
	for _, playerConn := range room.Players {
		select {
		case playerConn.Send <- event:
		default:
			close(playerConn.Close)
			event := room.removePlayerFromGameState(playerConn)
			leaveEvents = append(leaveEvents, event)
		}
	}
	return leaveEvents

}

func (room *Room) addPlayerToGameState(playerConn *PlayerConn) *protos.ServerEvent {
	room.Players[playerConn.Id] = playerConn
	playerState := protos.PlayerState{
		PlayerId: playerConn.Id,
		X:        int32(rand.Intn(100)),
		Y:        int32(rand.Intn(100)),
	}
	room.State[playerConn.Id] = &playerState
	snapshot := getGameState(room)
	select {
	case playerConn.Send <- snapshot:
	default:
	}

	joinEvent := protos.ServerGameJoin{
		PlayerState: &playerState,
	}
	serverEvent := protos.ServerEvent{
		Event: &protos.ServerEvent_GameJoin{
			GameJoin: &joinEvent,
		},
	}
	return &serverEvent

}

func (room *Room) removePlayerFromGameState(playerConn *PlayerConn) *protos.ServerEvent {
	delete(room.Players, playerConn.Id)
	delete(room.State, playerConn.Id)
	leaveEvent := protos.ServerGameExit{
		PlayerId: playerConn.Id,
	}
	serverEvent := protos.ServerEvent{
		Event: &protos.ServerEvent_GameExit{
			GameExit: &leaveEvent,
		},
	}
	return &serverEvent
}

func (room *Room) movePlayer(moveIntent *protos.StateIntentMessage) *protos.ServerEvent {
	playerState := room.State[moveIntent.PlayerId]
	playerState.X += moveIntent.Dx
	playerState.Y += moveIntent.Dy
	moveEvent := protos.PlayerState{
		PlayerId: moveIntent.PlayerId,
		X:        playerState.X,
		Y:        playerState.Y,
	}
	serverEvent := protos.ServerEvent{
		Event: &protos.ServerEvent_PlayerState{
			PlayerState: &moveEvent,
		},
	}
	return &serverEvent
}
