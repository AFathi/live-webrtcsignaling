package main

import (
	"context"
	"fmt"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/my"
)

type SocketIdMap struct {
	my.NamedRWMutex
	Data map[string]*connection
}

func NewSocketIdMap() *SocketIdMap {
	sm := new(SocketIdMap)
	sm.Data = make(map[string]*connection)
	sm.NamedRWMutex.Init("SocketIdMap")
	return sm
}

func (sm *SocketIdMap) Set(ctx context.Context, key string, value *connection) {
	sm.Lock(ctx)
	sm.Data[key] = value
	sm.Unlock(ctx)
}

func (sm *SocketIdMap) Get(ctx context.Context, key string) *connection {
	sm.RLock(ctx)
	c := sm.Data[key]
	sm.RUnlock(ctx)
	return c
}

func (sm *SocketIdMap) Delete(ctx context.Context, key string) {
	sm.Lock(ctx)
	delete(sm.Data, key)
	sm.Unlock(ctx)
}

// Hub maintains the set of active connections and broadcasts messages to the
// connections.
type Hub struct {
	register   chan *connection // Register requests from the connections.
	unregister chan *connection // Unregister requests from connections.
	close      chan *connection // Close requests from connections.

	socketIds *SocketIdMap // Registered socketIds
}

func NewHub() *Hub {
	hub := new(Hub)
	hub.register = make(chan *connection)
	hub.unregister = make(chan *connection)
	hub.close = make(chan *connection)
	hub.socketIds = NewSocketIdMap()
	return hub
}

func (hub *Hub) destroyConnection(ctx context.Context, c *connection) {
	log, _ := plogger.FromContext(ctx)
	rooms.Lock(ctx)
	usersConnectedOnRoomId := 0
	if rooms.Data[c.roomId] != nil {
		usersConnectedOnRoomId = len(rooms.Data[c.roomId].connections)
	}
	rooms.Unlock(ctx)
	// Send stats call duration for this user
	log.Infof("userConnectedOnRoomId is %d", usersConnectedOnRoomId)
	if usersConnectedOnRoomId > 1 {
		var stat string
		duration := int(time.Now().Sub(c.when).Seconds()) * 1000
		if c.roomId[0:3] == `w__` {
			stat = fmt.Sprintf("live.duration:%d|ms\nlive.web.duration:%d|ms", duration, duration)
		} else {
			stat = fmt.Sprintf("live.duration:%d|ms\nlive.mobile.duration:%d|ms", duration, duration)
		}
		log.Infof("[ STATS ] %s", stat)
		fmt.Fprintf(udpStats, stat)
	}
	if c.userId != "" {
		log.Infof("CALLING EVENTLEAVE")
		eventLeave(ctx, c)
	}
	log.Infof("socketIds.Delete(%s)", c.socketId)
	hub.socketIds.Delete(ctx, c.socketId)
	log.Infof("close socket connections %#v", c)
	close(c.send)

	return
}

func (hub *Hub) run() {
	// the hub has his own logger context
	ctx := plogger.NewContext(context.Background(), plogger.New().Prefix("HUB"))
	for {
		select {
		case c := <-hub.register:
			socketId := fmt.Sprintf("%X", generateSliceRand(16))
			c.socketId = socketId
			hub.socketIds.Set(ctx, socketId, c)
		case c := <-hub.unregister:
			go hub.destroyConnection(ctx, c)
		case c := <-hub.close:
			close(c.send)
		}
	}
}
