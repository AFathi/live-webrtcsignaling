package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	//"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-rabbitmqlib"
	"github.com/kr/pretty"
)

// WebRTC session
type Session struct {
	SocketId string `json:"socketId"` // de quel socket il s'agit : hexa a 16 char <= connexion web socket
	UserId   string `json:"userId"`   // côté backend ils ont d'autres types d'id
}

type ApiAction struct {
	Action string          `json:"a"`
	Data   json.RawMessage `json:"d"`
}

type WsJoin struct {
	Bearer          string `json:"bearer"`
	RoomId          RoomId `json:"roomId"`
	Platform        string `json:"platform"`
	DeviceName      string `json:"deviceName"`
	NetworkType     string `json:"networkType"`
	Version         string `json:"version"`    // OS or Browser version
	AppVersion      string `json:"appVersion"` // App Version (Tribe Version)
	Orientation     int    `json:"orientation"`
	Camera          string `json:"camera"`
	MaxVideoBitrate int    `json:"maxVideoBitrate,omitempty"`
	MaxAudioBitrate int    `json:"maxAudioBitrate,omitempty"`
}

type WsJoinR struct {
	SocketId               string          `json:"socketId"`
	UserId                 string          `json:"userId"`
	RoomSize               int             `json:"roomSize"`
	UserMediaConfiguration UMConfiguration `json:"userMediaConfiguration"`
	Sessions               []Session       `json:"sessions"`
}

type WsReconnect struct {
	SocketId string `json:"socketId"`
}

type WsResponse struct {
	Action  string          `json:"a"`
	Success bool            `json:"s"`
	Data    json.RawMessage `json:"d,omitempty"`
}

type UMCSize struct {
	Min   string `json:"min,omitempty"`
	Max   string `json:"max,omitempty"`
	Exact string `json:"exact,omitempty"`
}

type UMCVideo struct {
	Width     UMCSize `json:"width"`
	Height    UMCSize `json:"height"`
	FrameRate UMCSize `json:"frameRate"`
}

type UMConfiguration struct {
	Audio bool     `json:"audio"`
	Video UMCVideo `json:"video"`
}

type ICECandidate struct {
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMLineIndex int    `json:"sdpMLineIndex"`
	Completed     bool   `json:"completed,omitempty"`
}

// {"to":"D5C51F124A06746706EC8DBB7F04A288","candidate":{"candidate":"candidate:1321500371 2 udp 1685987070 78.201.204.97 35188 typ srflx raddr 192.168.0.22 rport 35188 generation 0 ufrag fhni network-id 2 network-cost 10","sdpMid":"audio","sdpMLineIndex":0}}
type WsExchangeCandidateTo struct {
	To        string       `json:"to"`
	Candidate ICECandidate `json:"candidate"`
}

type SdpEntry struct {
	Type string `json:"type"`
	Sdp  string `json:"sdp"`
}

type WsExchangeSdpTo struct {
	To  string   `json:"to"`
	Sdp SdpEntry `json:"sdp"`
}

type WsExchangeSdpFrom struct {
	From Session  `json:"from"`
	Sdp  SdpEntry `json:"sdp"`
}

type WsSendMessageTo struct {
	To      string          `json:"to"`
	Message json.RawMessage `json:"message"`
}

type WsSendMessageFrom struct {
	From    Session         `json:"from"`
	Message json.RawMessage `json:"message"`
}

type WsOrientationChange struct {
	Orientation int    `json:"orientation"`
	Camera      string `json:"camera"`
}

type WsEventSetAudioVideoMode struct {
	From  Session `json:"from"`
	Audio bool    `json:"audio"`
	Video bool    `json:"video"`
}

type WsEventSetBitrate struct {
	From    Session `json:"from"`
	Bitrate int     `json:"bitrate"`
}

type WsEventOrientationChange struct {
	From        Session `json:"from"`
	Orientation int     `json:"orientation"`
	Platform    string  `json:"platform"`
	Camera      string  `json:"camera"`
}

type WsEventWebrtcUp struct {
	From Session `json:"from"`
}

type WsEventCpu struct {
	CpuUsed int `json:"cpuUsed"`
}

type WsEventNetworkChange struct {
	NetworkType string `json:"networkType"`
}

type RmqResponse struct {
	Success bool            `json:"s"`
	Error   int             `json:"e,omitempty"`
	Data    json.RawMessage `json:"d"`
}

type RmqRoomJoinEvent struct {
	RoomId      RoomId `json:"roomId"`
	RoomSize    int    `json:"roomSize"`
	SocketId    string `json:"socketId"`
	UserId      string `json:"userId"`
	Platform    string `json:"platform"`
	DeviceName  string `json:"deviceName"`
	NetworkType string `json:"networkType"`
	AppVersion  string `json:"appVersion"`
	Version     string `json:"version"`
	Ip          string `json:"ip"`
	Bitrate     int    `json:"bitrate"`
}

type RmqRoomLeaveEvent struct {
	RoomId   RoomId `json:"roomId"`
	RoomSize int    `json:"roomSize"`
	SocketId string `json:"socketId"`
	UserId   string `json:"userId"`
}

type RmqWebrtcPingEvent struct {
	SocketId string `json:"socketId"`
}

type RmqBitrateChangeEvent struct {
	SocketId string `json:"socketId"`
	Type     string `json:"type"`
	Bitrate  int    `json:"bitrate"`
}

type RmqFreezeEvent struct {
	SocketId string `json:"socketId"`
}

type RmqCpuEvent struct {
	SocketId string `json:"socketId"`
	CpuUsed  int    `json:"cpuUsed"`
}

type RmqNetworkChangeEvent struct {
	SocketId    string `json:"socketId"`
	NetworkType string `json:"networkType"`
}

func generateSliceRand(n int) (b []byte) {
	for i := 0; i < n; i++ {
		var max *big.Int
		max = big.NewInt(256)
		n, _ := rand.Int(rand.Reader, max)
		b = append(b, byte(n.Int64()))
	}

	return
}

func jsonError(err error) []byte {
	return []byte(`{"error":"` + err.Error() + `"}`)
}

func buildJsonError(action string, code int) []byte {
	return []byte(fmt.Sprintf(`{"a":"%sR","s":false,"e":%d}`, action, code))
}

// eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiIxZTNvZERxUGZ1IiwiY2xpZW50SWQiOiJjb20udHJpYmVpbmMudHJpYmUiLCJpYXQiOjE0ODgyOTQ2MjAsImV4cCI6MTQ4ODI5ODIyMH0.Gd8bi4HZbnQOMY3ScfLHkzYWOHcUMQle-wIy31MLmyE
func checkAuth(ctx context.Context, roomId RoomId, tokenString string) (userId string, err error) {
	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	hmacSecret := []byte(config.JWTSecret)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			err := fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			log.Errorf(err.Error())
			return nil, err
		}
		return hmacSecret, nil
	})
	if err != nil {
		log.OnError(err, "cannot parse JWT token: %s", tokenString)
		return
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Checking if token is valid
		//expirationTime := time.Unix(claims["exp"], 0)
		//if expirationTime.Before(time.Now())
		log.Infof("Token is valid, userId is %s\n", claims["userId"])
		userId = claims["userId"].(string)
		return
	}
	log.Infof("Token %s is not valid or expired", tokenString)
	return
}

func join(ctx context.Context, c *connection, a string, wsJ WsJoin) (jsonAnswer []byte) {
	var wsR WsResponse
	var wsJR WsJoinR
	var err error

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	// Avoid multiple join on the same socketId
	c.joinMutex.Lock(ctx)
	defer func() {
		c.joinMutex.Unlock(ctx)
	}()

	// Test if user has already join
	if c.userId != "" {
		log.Errorf("[ ERROR ] Join room twice on roomId (%s) userId (%s)", c.roomId, c.userId)
		jsonAnswer = buildJsonError(a, ERROR_CODE_ROOM_ALREADY_JOINED)
		return
	}

	// Check Auth (bearer + RoomId)
	userId, err := checkAuth(ctx, wsJ.RoomId, wsJ.Bearer)
	if log.OnError(err, "Room join permission denied for bearer '%s' and roomId '%s'", wsJ.Bearer, wsJ.RoomId) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_AUTH_FAILED)
		return
	}
	c.userId = userId
	c.roomId = wsJ.RoomId
	c.platform = wsJ.Platform
	c.deviceName = wsJ.DeviceName
	c.networkType = wsJ.NetworkType
	c.version = wsJ.Version
	c.appVersion = wsJ.AppVersion
	c.orientation = wsJ.Orientation
	c.camera = wsJ.Camera

	room := rooms.Get(ctx, wsJ.RoomId)
	if room != nil {
		if len(room.connections) > 7 {
			jsonAnswer = buildJsonError(a, ERROR_CODE_ROOM_IS_FULL)
			return
		}
	}

	if wsJ.MaxVideoBitrate != 0 {
		c.maxVideoBitrate = wsJ.MaxVideoBitrate
	} else {
		c.maxVideoBitrate = config.Bitrates.Video.Max
	}
	if wsJ.MaxAudioBitrate != 0 {
		c.maxAudioBitrate = wsJ.MaxAudioBitrate
	} else {
		c.maxAudioBitrate = config.Bitrates.Audio.Max
	}
	log.OnError(err, "maxVideoBitrate is %d", c.maxVideoBitrate)
	log.OnError(err, "maxAudioBitrate is %d", c.maxAudioBitrate)

	rooms.Lock(ctx)

	room = rooms.Data[wsJ.RoomId]

	if room == nil {
		room = NewRoom()
		room.Lock(ctx)
		rooms.Data[wsJ.RoomId] = room
		rooms.Unlock(ctx)
	} else {
		room.Lock(ctx)
		rooms.Unlock(ctx)
	}

	// Count all peers connected to the room Id and
	// append this connection to rooms sent back to the browser
	var roomSize int
	for _, conn := range room.connections {
		var s Session
		s.SocketId = conn.socketId
		s.UserId = conn.userId
		wsJR.Sessions = append(wsJR.Sessions, s)
		roomSize++
	}

	room.connections = append(room.connections, c)

	room.Unlock(ctx)
	wsJR.UserId = userId
	wsJR.RoomSize = roomSize
	if wsJR.Sessions == nil {
		wsJR.Sessions = []Session{}
	}

	var rmqRE RmqRoomJoinEvent
	rmqRE.RoomId = wsJ.RoomId
	rmqRE.RoomSize = roomSize + 1
	rmqRE.SocketId = c.socketId
	rmqRE.UserId = c.userId
	rmqRE.Platform = c.platform
	rmqRE.DeviceName = c.deviceName
	rmqRE.NetworkType = c.networkType
	rmqRE.AppVersion = c.appVersion
	rmqRE.Version = c.version
	rmqRE.Ip = c.ip
	rmqRE.Bitrate = c.maxVideoBitrate
	var j []byte
	j, err = json.Marshal(&rmqRE)
	if log.OnError(err, "can't marshal interface %#v", rmqRE) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}
	err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventRoomJoinRK, j)
	log.OnError(err, "couldn't send event message '%s' to exchange %s", j, liverabbitmq.LiveEvents)

	var umConfiguration UMConfiguration
	var maxWidth int
	var maxHeight int
	if roomSize > 1 {
		if roomSize > 2 {
			maxWidth = 640 / 2
		} else {
			maxWidth = 640
		}
		maxHeight = 580 / ((roomSize + (roomSize % 2)) / 2)
	} else {
		maxWidth = 640
		maxHeight = 580
	}
	minWidthStr := strconv.Itoa(maxWidth / 3)
	minHeightStr := strconv.Itoa(maxHeight / 3)
	maxWidthStr := strconv.Itoa(maxWidth)
	maxHeightStr := strconv.Itoa(maxHeight)

	umConfiguration.Audio = true

	umConfiguration.Video.Width.Min = minWidthStr
	umConfiguration.Video.Width.Max = maxWidthStr
	//umConfiguration.Video.Width.Min = "0"
	//umConfiguration.Video.Width.Max = "1280"
	umConfiguration.Video.Height.Min = minHeightStr
	umConfiguration.Video.Height.Max = maxHeightStr
	//umConfiguration.Video.Height.Min = "0"
	//umConfiguration.Video.Height.Max = "1024"
	umConfiguration.Video.FrameRate.Min = "10"
	umConfiguration.Video.FrameRate.Max = "30"

	// Send eventUserMediaConfiguration to all peers of the roomId if there is more than 2 people
	// And update the map hash
	if roomSize > 0 {
		var apiA ApiAction
		apiA.Action = `eventUserMediaConfiguration`
		room := rooms.Get(ctx, wsJ.RoomId)
		if room == nil {
			jsonAnswer = buildJsonError(a, ERROR_CODE_SYSTEM)
			return
		}
		room.RLock(ctx)
		for _, conn := range room.connections {
			if conn.socketId != c.socketId {
				j, err = json.Marshal(&umConfiguration)
				if log.OnError(err, "can't marshal interface %#v", umConfiguration) {
					log.Infof("can't send eventUserMediaConfiguration event...")
				} else {
					apiA.Data = j
					var jsonE []byte
					jsonE, err = json.Marshal(&apiA)
					if log.OnError(err, "can't marshal interface %#v", apiA) {
						log.Infof("can't send eventUserMediaConfiguration event...")
					} else {
						if err := conn.write(ctx, websocket.TextMessage, jsonE); err != nil {
							log.Infof("can't send eventUserMediaConfiguration event %s: %s", string(jsonE), err)
						}
					}
				}
				log.Infof("calling eventOrientationChange from (%s/%s) to %s, with orientation %d, platform %s, camera %s", conn.socketId, conn.userId, c.socketId, conn.orientation, conn.platform, conn.camera)
				eventOrientationChangeSingle(ctx, conn.socketId, conn.userId, conn.roomId, c.socketId, conn.orientation, conn.platform, conn.camera)
			}
		}
		room.RUnlock(ctx)
	}

	// Send eventOrientationChange to all peers of the roomId if there is more than 2 people
	if roomSize > 0 {
		log.Infof("eventOrientationChange called")
		eventOrientationChange(ctx, c.socketId, c.userId, c.roomId, c.orientation, c.platform, c.camera)
	}

	wsJR.UserMediaConfiguration = umConfiguration
	wsJR.SocketId = c.socketId
	j, err = json.Marshal(&wsJR)
	if log.OnError(err, "can't marshal interface %#v", wsJR) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}
	wsR.Action = a + `R`
	wsR.Success = true
	wsR.Data = json.RawMessage(j)

	jsonAnswer, err = json.Marshal(&wsR)
	if log.OnError(err, "can't marshal interface %#v", wsR) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}

	return
}

func reconnect(ctx context.Context, c *connection, a string, wsRE WsReconnect) (jsonAnswer []byte) {
	var wsR WsResponse
	var err error

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	cSrc := hub.socketIds.Get(ctx, wsRE.SocketId)
	if cSrc == nil {
		log.Infof("socketId %s does not exist anymore, could not reconnect the websocket")
		jsonAnswer = buildJsonError(a, ERROR_CODE_SOCKETID_DOES_NOT_EXIST)
		return
	}
	c.copy(cSrc)
	hub.socketIds.Lock(ctx)
	delete(hub.socketIds.Data, c.socketId)
	hub.socketIds.Data[c.socketId] = c
	cSrc.state = `reconnected`
	hub.socketIds.Unlock(ctx)

	wsR.Action = a + `R`
	wsR.Success = true
	wsR.Data = nil

	jsonAnswer, err = json.Marshal(&wsR)
	if log.OnError(err, "can't marshal interface %#v", wsR) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}

	go retransmitMessages(ctx, c)

	return
}

func retransmitMessages(ctx context.Context, c *connection) {
	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	for _, d := range c.wsDataToRetransmit {
		err := c.write(ctx, websocket.TextMessage, d)
		log.OnError(err, "could not retransmit message %s to new websocket", string(d))
	}
	c.wsDataToRetransmit = [][]byte{}
}

func exchangeICECandidateDelayed(ctx context.Context, c *connection, wsECT WsExchangeCandidateTo) {
	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	time.Sleep(1 * time.Second)
	if wsECT.To == `publisher` {
		log.Infof("Received trickle candidate %#v", wsECT.Candidate)
	} else {
		log.Infof("Received trickle candidate %#v", wsECT.Candidate)
	}
}

func exchangeICECandidate(ctx context.Context, c *connection, a string, wsECT WsExchangeCandidateTo) (jsonAnswer []byte) {
	var wsR WsResponse
	var err error

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	// Need to replace by saving typ relay and send it at the end
	if wsECT.Candidate.Completed == true {
		return
	}
	/* Routed mode */
	if c.platform != `Web` && strings.Contains(wsECT.Candidate.Candidate, `typ relay`) {
		go exchangeICECandidateDelayed(ctx, c, wsECT)
	} else {
		if wsECT.To == `publisher` {
			// "candidate":"candidate:288186024 1 udp 41885951 35.185.68.52 12966 typ relay raddr 0.0.0.0 rport 0 generation 0 ufrag ouDD network-id 1 network-cost 10
			// If the candidate type is relay, buffer it, we want to send it after all other candidates
			/*err = c.j.trickle(`publisher`, wsECT.Candidate)
			  if logOnError(err, "cannot send ICE candidate to Janus with %s", wsECT.Candidate) {
			    jsonAnswer = buildJsonError(a, ERROR_CODE_JANUS_TRICKLE_ICE)
			    return
			  }*/
			log.Infof("Received ICE Candidate trickle %#v", wsECT.Candidate)
		} else {
			//cDst := hub.socketIds[wsECT.To]
			/*err = c.j.trickle(wsECT.To, wsECT.Candidate)
			  if logOnError(err, "cannot send ICE candidate to Janus with %s", wsECT.Candidate) {
			    jsonAnswer = buildJsonError(a, ERROR_CODE_JANUS_TRICKLE_ICE)
			    return
			  }*/
			log.Infof("Received ICE Candidate trickle %#v", wsECT.Candidate)
		}
	}

	wsR.Action = a + `R`
	wsR.Success = true
	wsR.Data = nil

	jsonAnswer, err = json.Marshal(&wsR)
	if log.OnError(err, "can't marshal interface %#v", wsR) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}
	return
}

func setMediaBitrateInSdp(ctx context.Context, sdp string, media string, bitrate int) (newSdp string) {
	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	str := fmt.Sprintf(`(m=%s[^\n]+\n(?:[cib]=[^\n]+\n)*)`, media)
	re, err := regexp.Compile(str)
	if log.OnError(err, "cannot compile regexp %s", str) {
		return
	}
	bitrateSdp := fmt.Sprintf("b=AS:%d\r\n", bitrate)
	newSdp = re.ReplaceAllString(sdp, "${1}"+bitrateSdp)
	if newSdp == "" {
		log.Infof("session description (SDP) is not a valid one, no m=%s found", media)
		return
	}
	return
}

func exchangeSdp(ctx context.Context, c *connection, a string, wsEST WsExchangeSdpTo) []byte {
	var wsR WsResponse
	var s *Room
	var jsonAnswer []byte
	var err error

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	s = rooms.Get(ctx, c.roomId)
	if s == nil {
		return buildJsonError(a, ERROR_CODE_SESSION)
	}
	if len(s.connections) == 0 {
		log.Infof("Race condition hit, there are no peers connected on this room, skipping message")
		return buildJsonError(a, ERROR_CODE_ROOM_EMPTY)
	}
	if wsEST.To == `publisher` {
		// we don't handle sdp renegociation yet.
		// first: we need to ensure that no webRTCSession publisher was already established
		//  if established => return an error
		c.negoSdpMutex.Lock(ctx)
		if c.webRTCSessionPublisher != nil {
			c.negoSdpMutex.Unlock(ctx)
			return buildJsonError(a, ERROR_CODE_SDP_ALREADY_NEGOCIATED)
		}
		//
		// A Publisher sends us an offer, we need to answer
		//
		sdpCtx := NewSdpCtx()
		sdpCtx.offer, err = parseSDP(ctx, wsEST.Sdp.Sdp)
		if log.OnError(err, "[ error ] SDP session decode error : %s") {
			c.negoSdpMutex.Unlock(ctx)
			return buildJsonError(a, ERROR_CODE_SDP_DECODE)
		}
		webRTCSession, err := NewWebRTCSession(ctx, WebRTCModePublisher, sdpCtx)
		if log.OnError(err, "could not create a new WebRTC Session") {
			c.negoSdpMutex.Unlock(ctx)
			return buildJsonError(a, ERROR_CODE_NETWORK)
		}

		var sdpAnswer string
    var preferredCodecOption CodecOptions
    preferredCodecOption = CodecH264
    if features.IsActive(ctx, "forcecodec") {
			switch features.GetVariant(ctx, "forcecodec") {
        case "VP8":
        	preferredCodecOption = CodecVP8
        case "H264":
          preferredCodecOption = CodecH264
        }
    }
    sdpAnswer, _ = sdpCtx.answerSDP(ctx, preferredCodecOption, webRTCSession.listenPort)
		webRTCSession.CreateStunCtx(ctx)
		c.webRTCSessionPublisher = webRTCSession
		c.negoSdpMutex.Unlock(ctx)

		go webRTCSession.serveWebRTC(ctx, c, nil)

		log.Debugf("[ DEBUG ] ------------------------------------")
		log.Debugf("[ DEBUG ] PUBLISHER SDP ANSWER :\n%s", pretty.Formatter(sdpAnswer))
		log.Debugf("[ DEBUG ] ------------------------------------")

		err = eventExchangeSdp(ctx, `publisher`, ``, c.socketId, `answer`, sdpAnswer)
		if log.OnError(err, "[ error ] could not sent eventExchangeSdp to `publisher` from socketId %s with %s", c.socketId, sdpAnswer) {
			return buildJsonError(a, ERROR_CODE_EVENT)
		}
	} else {
		log.Debugf("[ DEBUG ] ------------------------------------")
		log.Debugf("[ DEBUG ] LISTENER SDP ANSWER :\n%s", wsEST.Sdp.Sdp)
		log.Debugf("[ DEBUG ] ------------------------------------")
		cDst := hub.socketIds.Get(ctx, wsEST.To)
		webRTCSessionListener := c.webRTCSessionListeners.Get(wsEST.To)
		webRTCSessionPublisher := cDst.webRTCSessionPublisher
		webRTCSessionListener.sdpCtx.answer, err = parseSDP(ctx, wsEST.Sdp.Sdp)
		webRTCSessionListener.CreateStunCtx(ctx)
		if log.OnError(err, "[ error ] SDP session decode error : %s") {
			return buildJsonError(a, ERROR_CODE_SDP_DECODE)
		}
		//cDst.webRTCSessionListeners.Set(c.socketId, webRTCSessionListener)

		// Set max Video Bitrate for the session with current user Number
		//roomSize := rooms.GetSize()
		maxVideoBitrate := c.maxVideoBitrate /* / (roomSize + (roomSize % 2)) */
		log.Warnf("SET MAXVIDEOBITRATE DURING EXCHANGESDP TO %d", maxVideoBitrate)
		webRTCSessionListener.SetMaxVideoBitrate(maxVideoBitrate)

		// using saved udpConn & sdpSession
		go webRTCSessionListener.serveWebRTC(ctx, c, webRTCSessionPublisher)
	}

	wsR.Action = a + `R`
	wsR.Success = true
	wsR.Data = nil

	jsonAnswer, err = json.Marshal(&wsR)
	if log.OnError(err, "can't marshal interface %#v", wsR) {
		return buildJsonError(a, ERROR_CODE_JSON)
	}
	return jsonAnswer
}

func sendMessage(ctx context.Context, c *connection, a string, wsSMT WsSendMessageTo) (jsonAnswer []byte) {
	var wsSMF WsSendMessageFrom
	var wsR WsResponse

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	wsSMF.From.SocketId = c.socketId
	wsSMF.From.UserId = c.userId
	wsSMF.Message = wsSMT.Message

	j, err := json.Marshal(&wsSMF)
	if log.OnError(err, "can't marshal interface %#v", wsSMF) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}
	var apiA ApiAction
	apiA.Action = `eventMessage`
	apiA.Data = j
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}
	cDst := hub.socketIds.Get(ctx, wsSMT.To)
	if cDst != nil {
		cDst.write(ctx, websocket.TextMessage, j2)
	} else {
		log.Infof("socketId requested doesn't exist, skipping message")
		jsonAnswer = buildJsonError(a, ERROR_CODE_SOCKET_ID_DOES_NOT_EXIST)
		return
	}

	wsR.Action = a + `R`
	wsR.Success = true
	wsR.Data = nil

	jsonAnswer, err = json.Marshal(&wsR)
	if log.OnError(err, "can't marshal interface %#v", wsR) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}
	return
}

func orientationChange(ctx context.Context, c *connection, a string, wsOC WsOrientationChange) (jsonAnswer []byte) {
	var wsR WsResponse
	var err error

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	eventOrientationChange(ctx, c.socketId, c.userId, c.roomId, wsOC.Orientation, c.platform, wsOC.Camera)

	wsR.Action = a + `R`
	wsR.Success = true
	wsR.Data = nil

	jsonAnswer, err = json.Marshal(&wsR)
	if log.OnError(err, "can't marshal interface %#v", wsR) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}

	return
}

func eventExchangeSdp(ctx context.Context, socketId string, userId string, to string, sdpType string, sdp string) (err error) {
	var wsESF WsExchangeSdpFrom
	var jsonRequest []byte

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	cDst := hub.socketIds.Get(ctx, to)
	if cDst == nil {
		log.Infof("hub.socketIds[%s] is nil, could not send this eventExchangeSdp...", to)
		return
	}

	s := rooms.Get(ctx, cDst.roomId)
	if s == nil {
		log.Infof("session on roomId %s doesn't exist anymore, probably eventLeave has been called before", cDst.roomId)
		return
	}

	if len(s.connections) == 0 {
		log.Infof("Race condition hit, there is no peers connected on this room, skipping message")
		return
	}

	//newSdp := setMediaBitrateInSdp(sdp, `audio`, cDst.maxAudioBitrate / 1000)

	wsESF.From.SocketId = socketId
	wsESF.From.UserId = userId
	wsESF.Sdp.Type = sdpType
	wsESF.Sdp.Sdp = sdp

	jsonRequest, err = json.Marshal(&wsESF)
	if log.OnError(err, "can't marshal interface %#v", wsESF) {
		return
	}

	var apiA ApiAction
	apiA.Action = `eventExchangeSdp`
	apiA.Data = jsonRequest
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		return
	}
	if cDst != nil {
		log.Infof("[ DEBUG ] sending exchange SDP %s", j2)
		cDst.write(ctx, websocket.TextMessage, j2)
	} else {
		log.Infof("socketId requested doesn't exist, skipping message")
		return
	}

	return
}

func eventLeave(ctx context.Context, c *connection) {
	var apiA ApiAction
	var rmqWsRLE RmqRoomLeaveEvent
	var roomSize int

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	room := rooms.Get(ctx, c.roomId)
	if room == nil {
		log.Warnf("[ ERROR eventLeave ] rooms.Get(%s) == nil", c.roomId)
		return
	}
	// remove listeners pipelines & remove the connection from the room
	room.Lock(ctx)
	if c.webRTCSessionPublisher != nil {
		for i := 0; i < len(room.connections); i++ {
			c.webRTCSessionPublisher.disconnectListener(ctx, room.connections[i], c)
		}
	} else {
		log.Errorf("cannot disconnect listeners, missing webRTCSessionPublisher")
	}
	removedConn := room.Remove(ctx, c.socketId)
	if removedConn == nil {
		log.Errorf("websocket %s wasn't removed from room %s", c.socketId, c.roomId)
	}
	room.Unlock(ctx)

	apiA.Action = `eventLeave`
	rmqWsRLE.RoomId = c.roomId
	rmqWsRLE.RoomSize = roomSize
	rmqWsRLE.SocketId = c.socketId
	rmqWsRLE.UserId = c.userId

	j, err := json.Marshal(&rmqWsRLE)
	if log.OnError(err, "can't marshal interface %#v", rmqWsRLE) {
		return
	}
	apiA.Data = j
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		return
	}

	room = rooms.Get(ctx, c.roomId)
	if room == nil {
		log.Warnf("[ error ] session doesn't exist anymore for roomId %s, abort sending eventLeave", c.roomId)
		return
	}
	room.RLock(ctx)
	if room != nil {
		for _, c := range room.connections {
			c.write(ctx, websocket.TextMessage, j2)
			//roomSize := rooms.GetSize()
			maxVideoBitrate := c.maxVideoBitrate /* / (roomSize + (roomSize % 2)) */
			c.webRTCSessionListeners.RLock()
			for _, w := range c.webRTCSessionListeners.d {
				w.(*WebRTCSession).SetMaxVideoBitrate(maxVideoBitrate)
			}
			c.webRTCSessionListeners.RUnlock()
		}
	}
	room.RUnlock(ctx)

	err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventRoomLeaveRK, j)
	log.OnError(err, "couldn't send event message '%s' to exchange %s", j, liverabbitmq.LiveEvents)

	if len(room.connections) == 0 {
		log.Infof("room %s is empty => delete", c.roomId)
		rooms.Delete(ctx, c.roomId)
	}

	var umConfiguration UMConfiguration
	var maxWidth int
	var maxHeight int
	if roomSize > 1 {
		if roomSize > 2 {
			maxWidth = 480 / 2
		} else {
			maxWidth = 480
		}
		maxHeight = 360 / ((roomSize + (roomSize % 2)) / 2)
	} else {
		maxWidth = 480
		maxHeight = 360
	}
	minWidthStr := strconv.Itoa(maxWidth / 3)
	minHeightStr := strconv.Itoa(maxHeight / 3)
	maxWidthStr := strconv.Itoa(maxWidth)
	maxHeightStr := strconv.Itoa(maxHeight)

	umConfiguration.Audio = true
	umConfiguration.Video.Width.Min = minWidthStr
	umConfiguration.Video.Width.Max = maxWidthStr
	umConfiguration.Video.Height.Min = minHeightStr
	umConfiguration.Video.Height.Max = maxHeightStr
	umConfiguration.Video.FrameRate.Min = "10"
	umConfiguration.Video.FrameRate.Max = "25"

	// Send eventUserMediaConfiguration to all peers of the roomId if there is more than 2 people
	// And update the map hash
	if roomSize > 1 {
		var apiA ApiAction
		apiA.Action = `eventUserMediaConfiguration`
		room.RLock(ctx)
		for _, conn := range room.connections {
			j, err = json.Marshal(&umConfiguration)
			if log.OnError(err, "can't marshal interface %#v", umConfiguration) {
				log.Infof("can't send eventUserMediaConfiguration event...")
			} else {
				apiA.Data = j
				var jsonE []byte
				jsonE, err = json.Marshal(&apiA)
				if log.OnError(err, "can't marshal interface %#v", apiA) {
					log.Infof("can't send eventUserMediaConfiguration event...")
				} else {
					if err := conn.write(ctx, websocket.TextMessage, jsonE); err != nil {
						log.Infof("can't send eventUserMediaConfiguration event %s: %s", string(jsonE), err)
					}
				}
			}
		}
		room.RUnlock(ctx)
	}
	return
}

func eventSetAudioVideoMode(ctx context.Context, socketId string, userId string, to string, audio bool, video bool) (err error) {
	var wsESAVM WsEventSetAudioVideoMode
	var jsonRequest []byte

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	wsESAVM.From.SocketId = socketId
	wsESAVM.From.UserId = userId
	wsESAVM.Audio = audio
	wsESAVM.Video = video

	jsonRequest, err = json.Marshal(&wsESAVM)
	if log.OnError(err, "can't marshal interface %#v", wsESAVM) {
		return
	}
	var apiA ApiAction
	if socketId == `publisher` {
		apiA.Action = `eventSetLocalAudioVideoMode`
	} else {
		apiA.Action = `eventSetRemoteAudioVideoMode`
	}
	apiA.Data = jsonRequest
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		return
	}
	log.Infof("EVENT SET AUDIO VIDEO MODE TO %s SEND: %s", to, j2)
	cDst := hub.socketIds.Get(ctx, to)
	if cDst != nil {
		cDst.write(ctx, websocket.TextMessage, j2)
	} else {
		log.Infof("socketId requested doesn't exist, skipping message")
		return
	}

	return
}

func eventSetBitrate(ctx context.Context, socketId string, userId string, to string, bitrate int) (err error) {
	var wsESB WsEventSetBitrate
	var jsonRequest []byte

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	wsESB.From.SocketId = socketId
	wsESB.From.UserId = userId
	wsESB.Bitrate = bitrate

	jsonRequest, err = json.Marshal(&wsESB)
	if log.OnError(err, "can't marshal interface %#v", wsESB) {
		return
	}

	var apiA ApiAction
	apiA.Action = `eventSetBitrate`
	apiA.Data = jsonRequest
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		return
	}
	cDst := hub.socketIds.Get(ctx, socketId)
	if cDst != nil {
		cDst.write(ctx, websocket.TextMessage, j2)
	} else {
		log.Infof("socketId requested doesn't exist, skipping message")
		return
	}

	return
}

func eventOrientationChangeSingle(ctx context.Context, socketId string, userId string, roomId RoomId, to string, orientation int, platform string, camera string) (jsonAnswer []byte) {
	var wsEOC WsEventOrientationChange

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	wsEOC.From.SocketId = socketId
	wsEOC.From.UserId = userId
	wsEOC.Orientation = orientation
	wsEOC.Platform = platform
	wsEOC.Camera = camera

	jsonRequest, err := json.Marshal(&wsEOC)
	if log.OnError(err, "can't marshal interface %#v", wsEOC) {
		return
	}
	var apiA ApiAction
	apiA.Action = `eventOrientationChange`
	apiA.Data = jsonRequest
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		return
	}
	cDst := hub.socketIds.Get(ctx, to)
	if cDst != nil {
		log.Infof("[ WS SEND ] %s to %s", string(j2), to)
		cDst.write(ctx, websocket.TextMessage, j2)
	} else {
		log.Infof("socketId requested doesn't exist, skipping message")
	}

	return
}

func eventWebrtcUp(ctx context.Context, socketId string, userId string, to string) (err error) {
	var wsEWU WsEventWebrtcUp

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	wsEWU.From.SocketId = socketId
	wsEWU.From.UserId = userId

	jsonRequest, err := json.Marshal(&wsEWU)
	if log.OnError(err, "can't marshal interface %#v", wsEWU) {
		return
	}
	var apiA ApiAction
	apiA.Action = `eventWebrtcUp`
	apiA.Data = jsonRequest
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		return
	}
	cDst := hub.socketIds.Get(ctx, to)
	if cDst != nil {
		log.Infof("[ WS SEND ] %s to %s", string(j2), to)
		cDst.write(ctx, websocket.TextMessage, j2)
	} else {
		log.Infof("socketId requested doesn't exist, skipping message")
	}

	return
}

func eventOrientationChange(ctx context.Context, socketId string, userId string, roomId RoomId, orientation int, platform string, camera string) (jsonAnswer []byte) {
	var wsEOC WsEventOrientationChange

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	wsEOC.From.SocketId = socketId
	wsEOC.From.UserId = userId
	wsEOC.Orientation = orientation
	wsEOC.Platform = platform
	wsEOC.Camera = camera

	jsonRequest, err := json.Marshal(&wsEOC)
	if log.OnError(err, "can't marshal interface %#v", wsEOC) {
		return
	}
	var apiA ApiAction
	apiA.Action = `eventOrientationChange`
	apiA.Data = jsonRequest
	j2, err := json.Marshal(&apiA)
	if log.OnError(err, "can't marshal interface %#v", apiA) {
		return
	}

	s := rooms.Get(ctx, roomId)
	if s != nil {
		for _, c := range s.connections {
			log.Infof("c.socketId is %s and socketId is %s", c.socketId, socketId)
			if c != nil && c.socketId != socketId {
				log.Infof("[ WS SEND ] %s to %s", string(j2), c.socketId)
				c.write(ctx, websocket.TextMessage, j2)
			}
		}
	}

	return
}

func eventWebrtcPing(ctx context.Context, socketId string) (err error) {
	var rmqWPE RmqWebrtcPingEvent

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	rmqWPE.SocketId = socketId
	j, err := json.Marshal(&rmqWPE)
	if log.OnError(err, "can't marshal interface %#v", rmqWPE) {
		return
	}

	err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventWebrtcPingRK, j)
	log.OnError(err, "couldn't send event message '%s' to exchange %s", j, liverabbitmq.LiveEvents)

	return
}

func eventBitrateChange(ctx context.Context, socketId string, t string, bitrate int) (err error) {
	var rmqBC RmqBitrateChangeEvent

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	rmqBC.SocketId = socketId
	rmqBC.Type = t
	rmqBC.Bitrate = bitrate
	j, err := json.Marshal(&rmqBC)
	if log.OnError(err, "can't marshal interface %#v", rmqBC) {
		return
	}

	err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventBitrateChangeRK, j)
	log.OnError(err, "couldn't send event message '%s' to exchange %s", j, liverabbitmq.LiveEvents)

	return
}

func eventFreeze(ctx context.Context, c *connection, a string) (jsonAnswer []byte) {
	var rmqFE RmqFreezeEvent

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	rmqFE.SocketId = c.socketId
	j, err := json.Marshal(&rmqFE)
	if log.OnError(err, "can't marshal interface %#v", rmqFE) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}

	err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventWebrtcFreezeRK, j)
	if log.OnError(err, "couldn't send event message '%s' to exchange %s", j, liverabbitmq.LiveEvents) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_RMQ)
		return
	}

	return
}

func eventCpu(ctx context.Context, c *connection, a string, wsEC WsEventCpu) (jsonAnswer []byte) {
	var rmqCE RmqCpuEvent

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	rmqCE.SocketId = c.socketId
	rmqCE.CpuUsed = wsEC.CpuUsed
	j, err := json.Marshal(&rmqCE)
	if log.OnError(err, "can't marshal interface %#v", rmqCE) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}

	err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventWebrtcCpuRK, j)
	if log.OnError(err, "couldn't send event message '%s' to exchange %s", j, liverabbitmq.LiveEvents) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_RMQ)
		return
	}

	return
}

func eventNetworkChange(ctx context.Context, c *connection, a string, wsENC WsEventNetworkChange) (jsonAnswer []byte) {
	var rmqNCE RmqNetworkChangeEvent

	log := plogger.FromContextSafe(ctx).Tag("api")
	ctx = plogger.NewContext(ctx, log)
	rmqNCE.SocketId = c.socketId
	rmqNCE.NetworkType = wsENC.NetworkType
	j, err := json.Marshal(&rmqNCE)
	if log.OnError(err, "can't marshal interface %#v", rmqNCE) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_JSON)
		return
	}

	err = rmq.EventMessageSend(liverabbitmq.LiveEvents, liverabbitmq.LiveEventNetworkChangeRK, j)
	if log.OnError(err, "couldn't send event message '%s' to exchange %s", j, liverabbitmq.LiveEvents) {
		jsonAnswer = buildJsonError(a, ERROR_CODE_RMQ)
		return
	}

	return
}

func handleApi(ctx context.Context, c *connection, data []byte) (jsonAnswer []byte, corrId string) {
	var apiAA ApiAction

	log := plogger.FromContextSafe(ctx).Tag("api").Prefix("API")
	ctx = plogger.NewContext(ctx, log)
	log.Debugf("<-- %s", string(data))

	err := json.Unmarshal(data, &apiAA)
	if log.OnError(err, "Can't unmarshal data %s", string(data)) {
		jsonAnswer = buildJsonError(`general`, ERROR_CODE_JSON)
		return
	}

	if c.userId == `` && apiAA.Action != `join` && apiAA.Action != `reconnect` {
		jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_USER_NOT_AUTHENTICATED)
		return
	}
	log.Debugf("apiAA.Action is %s", apiAA.Action)
	switch apiAA.Action {
	case `join`:
		var wsJ WsJoin
		err = json.Unmarshal([]byte(apiAA.Data), &wsJ)
		if log.OnError(err, "Can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = join(ctx, c, apiAA.Action, wsJ)
	case `exchangeCandidate`:
		var wsECT WsExchangeCandidateTo
		err = json.Unmarshal([]byte(apiAA.Data), &wsECT)
		if log.OnError(err, "Can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = exchangeICECandidate(ctx, c, apiAA.Action, wsECT)
	case `exchangeSdp`:
		var wsEST WsExchangeSdpTo
		err = json.Unmarshal([]byte(apiAA.Data), &wsEST)
		if log.OnError(err, "Can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = exchangeSdp(ctx, c, apiAA.Action, wsEST)
	case `sendMessage`:
		var wsSMT WsSendMessageTo
		err = json.Unmarshal([]byte(apiAA.Data), &wsSMT)
		if log.OnError(err, "Can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = sendMessage(ctx, c, apiAA.Action, wsSMT)
	case `orientationChange`:
		var wsOC WsOrientationChange
		err = json.Unmarshal([]byte(apiAA.Data), &wsOC)
		if log.OnError(err, "Can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = orientationChange(ctx, c, apiAA.Action, wsOC)
	case `reconnect`:
		var wsRE WsReconnect
		err = json.Unmarshal([]byte(apiAA.Data), &wsRE)
		if log.OnError(err, "Can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = reconnect(ctx, c, apiAA.Action, wsRE)
	case `eventFreeze`:
		jsonAnswer = eventFreeze(ctx, c, apiAA.Action)
	case `eventCpu`:
		var wsEC WsEventCpu
		err = json.Unmarshal([]byte(apiAA.Data), &wsEC)
		if log.OnError(err, "can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = eventCpu(ctx, c, apiAA.Action, wsEC)
	case `eventNetworkChange`:
		var wsENC WsEventNetworkChange
		err = json.Unmarshal([]byte(apiAA.Action), &wsENC)
		if log.OnError(err, "can't unmarshal data %s", apiAA.Data) {
			jsonAnswer = buildJsonError(apiAA.Action, ERROR_CODE_JSON)
			return
		}
		jsonAnswer = eventNetworkChange(ctx, c, apiAA.Action, wsENC)
	default:
		jsonAnswer = []byte(fmt.Sprintf(`{"a":"`+apiAA.Action+`R","s":false,"e":%d}`, ERROR_CODE_UNKNOWN_ACTION))
	}

	log.Debugf("--> %s", string(jsonAnswer))

	return
}
