package main

import (
	"context"
	"time"

	"encoding/json"

	"github.com/heytribe/live-webrtcsignaling/my"
)

type RoomId string

type Rooms struct {
	my.NamedRWMutex

	dateCreation time.Time
	Data         map[RoomId]*Room
}

func NewRooms() *Rooms {
	rooms := new(Rooms)
	rooms.dateCreation = time.Now()
	rooms.Data = make(map[RoomId]*Room)
	rooms.NamedRWMutex.Init("rooms")
	return rooms
}

func (rooms *Rooms) Set(ctx context.Context, roomId RoomId, value *Room) {
	rooms.Lock(ctx)
	rooms.Data[roomId] = value
	rooms.Unlock(ctx)
}

func (rooms *Rooms) Get(ctx context.Context, roomId RoomId) (s *Room) {
	rooms.RLock(ctx)
	s = rooms.Data[roomId]
	rooms.RUnlock(ctx)
	return
}

func (rooms *Rooms) Delete(ctx context.Context, roomId RoomId) {
	rooms.Lock(ctx)
	delete(rooms.Data, roomId)
	rooms.Unlock(ctx)
}

func (rooms *Rooms) MarshalJSON() ([]byte, error) {
	ctx := getServerStateContext()
	rooms.Lock(ctx)
	defer rooms.Unlock(ctx)

	var jsonRooms []jsonRoom
	for id, room := range rooms.Data {
		jsonRooms = append(jsonRooms, newJsonRoom(id, room))
	}
	return json.Marshal(jsonRooms)
}

func (rooms *Rooms) GetSize() int {
	rooms.RLock(ctx)
	defer rooms.RUnlock(ctx)

	return len(rooms.Data)
}
