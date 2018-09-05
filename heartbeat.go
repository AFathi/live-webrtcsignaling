package main

import (
	"context"
	"encoding/json"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-rabbitmqlib"
)

type RmqHeartbeatRoomsOnline struct {
	RoomId   RoomId    `json:"roomId"`
	RoomSize int       `json:"roomSize"`
	Sessions []Session `json:"sessions"`
}

func sendHeartbeatRoomsOnline(ctx context.Context, every time.Duration) {
	var jsonEvent []byte
	var err error

	log, _ := plogger.FromContext(ctx)
	for {
		rooms.Lock(ctx)
		for roomId, session := range rooms.Data {
			var rmqHRO RmqHeartbeatRoomsOnline
			rmqHRO.RoomId = roomId
			rmqHRO.Sessions = []Session{}
			for _, c := range session.connections {
				var s Session
				s.SocketId = c.socketId
				s.UserId = c.userId
				rmqHRO.Sessions = append(rmqHRO.Sessions, s)
			}
			rmqHRO.RoomSize = len(rmqHRO.Sessions)
			jsonEvent, err = json.Marshal(&rmqHRO)
			if log.OnError(err, "cannot marshal interface %#v", rmqHRO) == false {
				err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventRoomOnlineRK, jsonEvent)
				log.OnError(err, "cannot send event %s to exchange %s routing key %s", jsonEvent, liverabbitmq.LiveEvents, liverabbitmq.LiveEventRoomOnlineRK)
			}
		}
		rooms.Unlock(ctx)
		time.Sleep(every * time.Second)
	}
}
