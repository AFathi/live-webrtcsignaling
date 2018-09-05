package main

import (
	"context"
	"time"

	"encoding/json"

	"github.com/gorilla/websocket"
	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/my"
)

type connection struct {
	wsId               uint64
	dateCreation       time.Time
	ws                 *websocket.Conn
	wsMutex            my.NamedMutex
	joinMutex          my.NamedMutex
	negoSdpMutex       my.NamedMutex
	send               chan []byte
	roomId             RoomId
	userId             string
	socketId           string
	platform           string
	deviceName         string
	networkType        string
	version            string
	appVersion         string
	orientation        int
	camera             string
	sessionId          string
	when               time.Time
	maxVideoBitrate    int
	maxAudioBitrate    int
	state              string
	wsDataToRetransmit [][]byte
	ip                 string
	exit               bool
	// tempfix
	webRTCSessionListeners *WebRTCSessionMap
	/*udpConnPublisher	  *net.UDPConn
	udpConnListener			*ProtectedMap
	sdpSession *SdpSession*/
	webRTCSessionPublisher *WebRTCSession
}

func NewConnection(wsId uint64, ws *websocket.Conn) *connection {
	c := new(connection)
	c.wsId = wsId
	c.dateCreation = time.Now()
	c.send = make(chan []byte, 8192)
	c.ws = ws
	c.when = time.Now()
	c.state = `creating`
	c.exit = false
	c.webRTCSessionListeners = NewWebRTCSessionMap()
	// naming mutex
	c.wsMutex.Init("connection.ws")
	c.joinMutex.Init("connection.join")
	c.negoSdpMutex.Init("connection.negoSdp")
	return c
}

func (c *connection) processMessage(ctx context.Context, message []byte) (err error) {
	jsonStr, corrId := handleApi(ctx, c, message)
	if corrId == "" && jsonStr != nil {
		// ws server -> ws client (browser)
		if err = c.write(ctx, websocket.TextMessage, jsonStr); err != nil {
			return
		}
	}

	return
}

func (c *connection) readPump(ctx context.Context) (err error) {
	var message []byte

	log := plogger.FromContextSafe(ctx).Prefix("WSCONN").Tag("wsconn")
	defer func() {
		c.ws.Close()
	}()
	c.ws.SetReadLimit(config.Network.Ws.MaxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(config.Network.Ws.PongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(config.Network.Ws.PongWait))
		return nil
	})
	for c.exit == false {
		_, message, err = c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Infof("error: %v", err)
			}
			c.exit = true
			log.Debugf("PROUT2")
			return
		}
		err = c.processMessage(ctx, message)
		if log.OnError(err, "cannot processMessage and write on the websocket with message %s", message) {
			c.exit = true
		}
	}

	return
}

// write writes a message with the given message type and payload.
func (c *connection) write(ctx context.Context, mt int, payload []byte) (err error) {
	c.wsMutex.Lock(ctx)
	defer func() {
		c.wsMutex.Unlock(ctx)
	}()
	c.ws.SetWriteDeadline(time.Now().Add(config.Network.Ws.WriteWait))
	err = c.ws.WriteMessage(mt, payload)
	if err != nil {
		// Error on the socket during write, saving the message
		c.wsDataToRetransmit = append(c.wsDataToRetransmit, payload)
		return
	}

	return
}

// writePump pumps messages from the hub to the websocket connection.
func (c *connection) writePump(ctx context.Context) {
	ticker := time.NewTicker(config.Network.Ws.PingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	tickerPing := time.NewTicker(config.Network.Ws.WebRTCPingPeriod)
	defer func() {
		tickerPing.Stop()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(ctx, websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(ctx, websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(ctx, websocket.PingMessage, []byte{}); err != nil {
				return
			}
		case <-tickerPing.C:
			eventWebrtcPing(ctx, c.socketId)
		}
	}
}

func (c *connection) copy(cSrc *connection) {
	c.roomId = cSrc.roomId
	c.userId = cSrc.userId
	c.socketId = cSrc.socketId
	c.platform = cSrc.platform
	c.orientation = cSrc.orientation
	c.camera = cSrc.camera
	c.sessionId = cSrc.sessionId
	c.when = cSrc.when
	c.maxVideoBitrate = cSrc.maxVideoBitrate
	c.maxAudioBitrate = cSrc.maxAudioBitrate
	c.wsDataToRetransmit = cSrc.wsDataToRetransmit
}

func (c *connection) manageTimeout(ctx context.Context, ttl time.Duration) {
	if c.state == `timeout` {
		c.state = `waitingTTL`
	}
	log := plogger.FromContextSafe(ctx).Prefix("WSCONN").Tag("wsconn")
	time.Sleep(ttl)
	log.Infof("manageTimeout, duration is expired, now unregister the socket")
	if c.state == `waitingTTL` {
		log.Infof("socketId has not been reconnected, unregister %#v", c)
		c.joinMutex.Lock(ctx)
		hub.unregister <- c
		c.joinMutex.Unlock(ctx)
	} else {
		log.Infof("socketId has been reconnected, just closing the old one %#v", c)
		hub.close <- c
	}
	return
}

func (c *connection) getPublisherCodec(ctx context.Context) (codecOption CodecOptions, ok bool) {
	if c.webRTCSessionPublisher == nil {
		return -1, false
	}
	codecOption, ok = c.webRTCSessionPublisher.getCodec(ctx)
	return
}

// JSON marshaling
type jsonConnection struct {
	SocketId               string            `json:"socketId"`
	DateCreation           time.Time         `json:"dateCreation"`
	UserId                 string            `json:"userId"`
	Ip                     string            `json:"ip"`
	State                  string            `json:"state"`
	Exit                   bool              `json:"exit"`
	MaxVideoBitrate        int               `json:"maxVideoBitrate"`
	MaxAudioBitrate        int               `json:"maxAudioBitrate"`
	WebRTCSessionListeners *WebRTCSessionMap `json:"listeners"`
	WebRTCSessionPublisher *WebRTCSession    `json:"publisher"`
}

func newJsonConnection(c *connection) jsonConnection {
	return jsonConnection{
		c.socketId,
		c.dateCreation,
		c.userId,
		c.ip,
		c.state,
		c.exit,
		c.maxVideoBitrate,
		c.maxAudioBitrate,
		c.webRTCSessionListeners,
		c.webRTCSessionPublisher,
	}
}

func (c *connection) MarshalJSON() ([]byte, error) {
	return json.Marshal(newJsonConnection(c))
}
