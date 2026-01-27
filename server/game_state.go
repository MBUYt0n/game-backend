package game_state

import (
	"game-backend/protos"
	"math/rand"
)

type PlayerConn struct {
	id    string
	send  chan *protos.ServerEvent
	close chan struct{}
}

type Room struct {
	players map[string]*PlayerConn
	state   map[string]*protos.PlayerState

	join     chan *PlayerConn
	leave    chan *PlayerConn
	move     chan protos.StateIntentMessage
	snapshot chan bool
}

func newRoom() *Room {
	return &Room{
		players: make(map[string]*PlayerConn),
		state:   make(map[string]*protos.PlayerState),
		join:    make(chan *PlayerConn, 2),
		leave:   make(chan *PlayerConn, 2),
		move:    make(chan protos.StateIntentMessage, 2),
	}
}

func (room *Room) run() {
	var event protos.ServerEvent
	for {
		select {
		case p := <-room.join:
			event = room.addPlayerToGameState(p)
		case p := <-room.leave:
			event = room.removePlayerFromGameState(p)
		case m := <-room.move:
			event = room.movePlayer(&m)
		}
		leaveEvents := room.broadcast(&event)
		for _, e := range leaveEvents {
			room.broadcast(&e)
		}
	}
}

func getGameState(room *Room) protos.ServerEvent {

	players := make([]*protos.PlayerState, 0, len(room.state))
	for _, ps := range room.state {
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
	return serverEvent

}

func (room *Room) broadcast(event *protos.ServerEvent) []protos.ServerEvent {
	var leaveEvents []protos.ServerEvent
	for _, playerConn := range room.players {
		select {
		case playerConn.send <- event:
		default:
			close(playerConn.close)
			event := room.removePlayerFromGameState(playerConn)
			leaveEvents = append(leaveEvents, event)
		}
	}
	return leaveEvents

}

func (room *Room) addPlayerToGameState(playerConn *PlayerConn) protos.ServerEvent {
	room.players[playerConn.id] = playerConn
	playerState := protos.PlayerState{
		PlayerId: playerConn.id,
		X:        int32(rand.Intn(100)),
		Y:        int32(rand.Intn(100)),
	}
	room.state[playerConn.id] = &playerState
	snapshot := getGameState(room)
	select {
	case playerConn.send <- &snapshot:
	default:
		joinEvent := protos.ServerGameJoin{
			PlayerState: &playerState,
		}
		serverEvent := protos.ServerEvent{
			Event: &protos.ServerEvent_GameJoin{
				GameJoin: &joinEvent,
			},
		}
		return serverEvent
	}
}

func (room *Room) removePlayerFromGameState(playerConn *PlayerConn) protos.ServerEvent {
	delete(room.players, playerConn.id)
	delete(room.state, playerConn.id)
	leaveEvent := protos.ServerGameExit{
		PlayerId: playerConn.id,
	}
	serverEvent := protos.ServerEvent{
		Event: &protos.ServerEvent_GameExit{
			GameExit: &leaveEvent,
		},
	}
	return serverEvent
}

func (room *Room) movePlayer(moveIntent *protos.StateIntentMessage) protos.ServerEvent {
	playerState := room.state[moveIntent.PlayerId]
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
	return serverEvent
}
