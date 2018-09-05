package main

import (
	"context"
	"sync/atomic"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/my"
)

var gRoomId uint64

type Room struct {
	my.NamedRWMutex

	dateCreation time.Time
	id           uint64
	connections  []*connection
}

func NewRoom() *Room {
	room := new(Room)
	room.id = atomic.AddUint64(&gRoomId, 1)
	room.dateCreation = time.Now()
	room.connections = []*connection{}
	room.NamedRWMutex.Init("room:%d", room.id)
	return room
}

// not thread safe
func (room *Room) GetConnection(socketId string) *connection {
	for i := 0; i < len(room.connections); i++ {
		c := room.connections[i]
		if c.socketId == socketId {
			return c
		}
	}
	return nil
}

// not thread safe
func (room *Room) Remove(ctx context.Context, socketId string) *connection {
	log := plogger.FromContextSafe(ctx)
	for i := 0; i < len(room.connections); i++ {
		c := room.connections[i]
		if c.socketId == socketId {
			room.connections = append(room.connections[:i], room.connections[i+1:]...)
			log.Infof("room: removing websocket %s, new room size", socketId, len(room.connections))
			return c
		}
	}
	return nil
}

func (room *Room) Range(ctx context.Context, f func(int, *connection)) {
	room.RLock(ctx)
	defer room.RUnlock(ctx)
	for i, conn := range room.connections {
		f(i, conn)
	}
}

// JSON marshaling
type jsonRoom struct {
	Id           RoomId        `json:"id"`
	DateCreation time.Time     `json:"dateCreation"`
	Connections  []*connection `json:"connections"`
}

// note: had to insert room connections directly into jsonRooms instead of embedding a room in jsonRoom (and declaring
// MarshalJSON() in the room struct) because of https://stackoverflow.com/q/38489776

func newJsonRoom(id RoomId, room *Room) jsonRoom {
	return jsonRoom{
		id,
		room.dateCreation,
		room.connections,
	}
}
