package main

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"fmt"

	"encoding/json"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/dtls"
	"github.com/heytribe/live-webrtcsignaling/gst"
	"github.com/heytribe/live-webrtcsignaling/srtp"
	"github.com/kr/pretty"
)

type WebRTCMode int

const (
	WebRTCModePublisher WebRTCMode = 0
	WebRTCModeListener  WebRTCMode = 1
)

type WebRTCSessionMap struct {
	ProtectedMap
}

func (m *WebRTCSessionMap) Get(id string) *WebRTCSession {
	value := m.ProtectedMap.Get(id)
	if value != nil {
		return value.(*WebRTCSession)
	}
	return nil
}

func NewWebRTCSessionMap() *WebRTCSessionMap {
	m := new(WebRTCSessionMap)
	m.ProtectedMap = *NewProtectedMap()
	return m
}

type WebRTCSession struct {
	mode WebRTCMode
	webRTCSessionPublisher *WebRTCSession
	// max videoBitrate authorized
	maxVideoBitrate int
	//
	udpConn    *net.UDPConn
	listenPort int
	//
	sdpCtx   *SdpContext
	stunCtx  *StunContext
	stunMode StunMode
	c        *connectionUdp
	bus      *gst.GstBus
	//loop		*gst.GMainLoop
	videoJitterBuffer *JitterBuffer
	audioJitterBuffer *JitterBuffer
	p                 *Pipeline
	// listener: last rembs received, publisher: last rembs sent.
	lastRembs []int
	// listener only: last encoding bitrate set
	lastEncodingBitrate []int
	// publisher: input bandwidth, listener: output bandwidth
	lastBandwidthEstimates []uint64
	//
	disconnected bool
	ctxCancel    context.CancelFunc
}

func (w *WebRTCSession) SetMaxVideoBitrate(bitrate int) {
	fmt.Printf("SET MAX VIDEOBITRATE ON GSTSESSION TO %d", bitrate)
	w.maxVideoBitrate = bitrate
	if w.c != nil && w.c.gstSession != nil && w.c.gstSession.GetVideoBitrate() > bitrate{
		w.c.gstSession.SetMaxVideoEncodingBitrate(bitrate)
	}
}

func (w *WebRTCSession) GetMaxVideoBitrate() int {
	return w.maxVideoBitrate
}

func (w *WebRTCSession) dtlsClientConnect(ctx context.Context) {
	var err error

	log, _ := plogger.FromContext(ctx)
	log.Debugf("dtls client connect")
	// Now DTLS connect
	w.c.dtlsSession, err = dtlsCtx.NewDTLS(w.c.send, w.stunCtx.RAddr, dtls.DtlsRoleClient)
	if log.OnError(err, "[ error ] could not DTLS link with channel %#v", w.c.send) {
		return
	}
	log.Debugf("dtls client connect state")
	w.c.dtlsState = DtlsStateCreated
	// Handshaking DTLS
	err = w.c.dtlsSession.Handshake()
	if log.OnError(err, "[ error ] could not handshake DTLS") {
		w.c.dtlsState = DtlsStateFailed
		return
	}
	w.c.dtlsState = DtlsStateConnected
	log.Infof("[ WEBRTC ] DTLS is connected with Client Mode")
	var srtpKeys *dtls.SrtpKeys
	srtpKeys, err = w.c.dtlsSession.GetSrtpKeys()
	if log.OnError(err, "[ error ] could not export keys for DTLS session") {
		w.c.dtlsState = DtlsStateFailed
		return
	}
	localSrtp := make([]byte, 30)
	remoteSrtp := make([]byte, 30)
	copy(localSrtp[0:16], srtpKeys.LocalKey)
	copy(localSrtp[16:30], srtpKeys.LocalSalt)
	copy(remoteSrtp[0:16], srtpKeys.RemoteKey)
	copy(remoteSrtp[16:30], srtpKeys.RemoteSalt)
	/*logger.Infof("[ WEBRTC ] DTLS SRTP localSrtp concat is %#v", localSrtp)
	logger.Infof("[ WEBRTC ] DTLS SRTP base64 local key is %s", base64.StdEncoding.EncodeToString(localSrtp))
	logger.Infof("[ WEBRTC ] DTLS SRTP remoteSrtp concat is %#v", remoteSrtp)
	logger.Infof("[ WEBRTC ] DTLS SRTP base64 remote key is %s", base64.StdEncoding.EncodeToString(remoteSrtp))*/

	// create SRTP session with keys
	log.Infof("[ STUN ] CREATING SRTP SESSION")
	w.c.srtpSession, err = srtp.Create(localSrtp, remoteSrtp)
	if log.OnError(err, "[ STUN ] Could not create SRTP session") {
		return
	}

	return
}

func (w *WebRTCSession) dtlsServerAccept(ctx context.Context) {
	var err error

	log, _ := plogger.FromContext(ctx)
	w.c.dtlsSession, err = dtlsCtx.NewDTLS(w.c.send, w.stunCtx.RAddr, dtls.DtlsRoleServer)
	if log.OnError(err, "[ error ] could not DTLS link with channel %#v", w.c.send) {
		return
	}
	w.c.dtlsState = DtlsStateCreated
	err = w.c.dtlsSession.Accept()
	if log.OnError(err, "[ error ] could not accept DTLS in server mode") {
		w.c.dtlsState = DtlsStateFailed
		return
	}
	w.c.dtlsState = DtlsStateConnected
	log.Infof("[ WEBRTC ] DTLS is connected with Server Mode")
	var srtpKeys *dtls.SrtpKeys
	srtpKeys, err = w.c.dtlsSession.GetSrtpKeys()
	if logOnError(err, "[ error ] could not export keys for DTLS session") {
		w.c.dtlsState = DtlsStateFailed
		return
	}
	localSrtp := make([]byte, 30)
	remoteSrtp := make([]byte, 30)
	copy(localSrtp[0:16], srtpKeys.LocalKey)
	copy(localSrtp[16:30], srtpKeys.LocalSalt)
	copy(remoteSrtp[0:16], srtpKeys.RemoteKey)
	copy(remoteSrtp[16:30], srtpKeys.RemoteSalt)
	log.Infof("[ WEBRTC ] DTLS SRTP localSrtp concat is %#v", localSrtp)
	//logger.Infof("[ WEBRTC ] DTLS SRTP base64 local key is %s", base64.StdEncoding.EncodeToString(localSrtp))
	log.Infof("[ WEBRTC ] DTLS SRTP remoteSrtp concat is %#v", remoteSrtp)
	//logger.Infof("[ WEBRTC ] DTLS SRTP base64 remote key is %s", base64.StdEncoding.EncodeToString(remoteSrtp))*/

	// create SRTP session with keys
	w.c.srtpSession, err = srtp.Create(remoteSrtp, localSrtp)
	if log.OnError(err, "[ STUN ] Could not create SRTP session") {
		return
	}

	return
}

func (w *WebRTCSession) connectListeners(ctx context.Context, ourConn *connection) {
	log := plogger.FromContextSafe(ctx)
	room := rooms.Get(ctx, ourConn.roomId)
	// foreach peer in the room excluding ourselves
	room.Range(ctx, func(i int, peerConn *connection) {
		if peerConn.socketId == ourConn.socketId {
			return // exclude ourself
		}
		// adding our connection to the listeners list of the peer (we became a listener of the peer)
		sdpCtx := NewSdpCtx()
		webRTCSession, err := NewWebRTCSession(ctx, WebRTCModeListener, sdpCtx)
		if log.OnError(err, "could not create a new WebRTC Session (1)") {
			return
		}

		peerCodec, _ := peerConn.getPublisherCodec(ctx)
		sdpCtx.createSdpOffer(ctx, peerCodec, webRTCSession.listenPort)
		log.Debugf("Setting listener with socketId %s with WebRTCSession %#v on c %s", ourConn.socketId, webRTCSession, peerConn.socketId)
		cDst := hub.socketIds.Get(ctx, peerConn.socketId)
		cDst.webRTCSessionListeners.Set(ourConn.socketId, webRTCSession)
		log.Debugf("------------------------------------")
		log.Debugf("CONNECT LISTENER SDP OFFER :\n%s", pretty.Formatter(webRTCSession.sdpCtx.offer))
		log.Debugf("------------------------------------")

		eventExchangeSdp(ctx, ourConn.socketId, ourConn.userId, peerConn.socketId, "offer", webRTCSession.sdpCtx.offer.Write(ctx))

		// adding the peer connection to our peer list (the peer became a listener of us)
		sdpCtx = NewSdpCtx()

		webRTCSession, err = NewWebRTCSession(ctx, WebRTCModeListener, sdpCtx)
		if logOnError(err, "could not create a new WebRTC Session (2)") {
			return
		}

		ourCodec, _ := ourConn.getPublisherCodec(ctx)
		sdpCtx.createSdpOffer(ctx, ourCodec, webRTCSession.listenPort)
		log.Debugf("Setting listener with socketId %s with WebRTCSession %#v on c %s", ourConn.socketId, webRTCSession, peerConn.socketId)
		ourConn.webRTCSessionListeners.Set(peerConn.socketId, webRTCSession)

		log.Debugf("------------------------------------")
		log.Debugf("CONNECT LISTENER SDP OFFER :\n%s", pretty.Formatter(webRTCSession.sdpCtx.offer))
		log.Debugf("------------------------------------")

		eventExchangeSdp(ctx, peerConn.socketId, peerConn.userId, ourConn.socketId, "offer", webRTCSession.sdpCtx.offer.Write(ctx))
	})
}

/*
   remove listener pipeline:
    - other peer connection <= our stream
    - other peer stream => our connection
*/
func (w *WebRTCSession) disconnectListener(ctx context.Context, peerConn *connection, ourConn *connection) {
	if peerConn.socketId == ourConn.socketId {
		return // cannot disconnect ourself :)
	}
	log := plogger.FromContextSafe(ctx)
	cDst := hub.socketIds.Get(ctx, peerConn.socketId)
	webRTCSession := cDst.webRTCSessionListeners.Get(ourConn.socketId)
	if webRTCSession != nil {
		cDst.webRTCSessionListeners.Del(ourConn.socketId)
		webRTCSession.Disconnect(ctx)
		log.Infof("Setting listener with socketId %s with WebRTCSession %#v on c %s", ourConn.socketId, webRTCSession, peerConn.socketId)
	}
	webRTCSession = ourConn.webRTCSessionListeners.Get(peerConn.socketId)
	if webRTCSession != nil {
		ourConn.webRTCSessionListeners.Del(peerConn.socketId)
		webRTCSession.Disconnect(ctx)
		log.Infof("Setting listener with socketId %s with WebRTCSession %#v on c %s", ourConn.socketId, webRTCSession, peerConn.socketId)
	}
}

/*
 * prod: listeUdp on random port
 * dev: listenUdp on port >= DEV_MIN_UDP_PORT && < DEV_MAX_UDP_PORT
 */
func ListenUdp(ctx context.Context) (*net.UDPConn, error) {
	if config.StaticPorts == false {
		return net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	}

	var udpConn *net.UDPConn
	var err error
	log, _ := plogger.FromContext(ctx)

	for port := DEV_MIN_UDP_PORT; port < DEV_MAX_UDP_PORT; port++ {
		log.Infof("[ WEBRTC ] try to listen on udp port %d", port)
		udpConn, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: port})
		if err == nil {
			return udpConn, nil
		}
	}
	return udpConn, err // last ones :(
}

func NewWebRTCSession(ctx context.Context, webRTCMode WebRTCMode, sdpCtx *SdpContext) (w *WebRTCSession, err error) {
	var udpConn *net.UDPConn

	udpConn, err = ListenUdp(ctx)
	if logOnError(err, "[ WEBRTC ] [ error ] could not choose a free UDP port to listen STUN/DTLS/SRTP/RTCP protocols") {
		err = errors.New("could not listen on UDP with automatic port (0)")
		return
	}
	listenPort, _ := strconv.Atoi(strings.Split(udpConn.LocalAddr().String(), ":")[1])

	var stunMode StunMode
	if webRTCMode == WebRTCModePublisher {
		stunMode = StunAnswererMode
	} else {
		stunMode = StunOffererMode
	}

	//loop := gst.MainLoopNew()

	w = &WebRTCSession{
		mode:       webRTCMode,
		udpConn:    udpConn,
		listenPort: listenPort,
		sdpCtx:     sdpCtx,
		stunCtx:    nil,
		stunMode:   stunMode,
		c:          nil,
	}

	return
}

func (w *WebRTCSession) CreateStunCtx(ctx context.Context) {
	w.stunCtx = NewStunCtx(ctx, w.udpConn.LocalAddr().String(), w.sdpCtx, w.stunMode)
}

func (w *WebRTCSession) Disconnect(ctx context.Context) {
	w.disconnected = true
	if w.ctxCancel != nil {
		w.ctxCancel()
		return
	}
}

func (w *WebRTCSession) serveWebRTC(ctx context.Context, wsConn *connection, webRTCSessionPublisher *WebRTCSession) error {
	var err error

	w.webRTCSessionPublisher = webRTCSessionPublisher
	
	log := plogger.FromContextSafe(ctx).Prefix("WebRTC").Tag("webrtc-session")
	log.Infof("START SERVE WEB RTC")
	// make webrtc session cancelable using context
	ctx, w.ctxCancel = context.WithCancel(ctx)
	defer w.ctxCancel()
	if w.disconnected == true {
		return err
	}
	// create socket udp
	connUdp := NewConnectionUdp(ctx, w.udpConn, wsConn)
	connUdp.sdpCtx = w.sdpCtx
	connUdp.state = `created`
	w.c = connUdp

	go connUdp.writePump(ctx)

	codec, _ := wsConn.getPublisherCodec(ctx)
	log.Warnf("CODEC IS %d", codec)
	if w.mode == WebRTCModePublisher {
		w.serveWebRTCPublisher(ctx, codec)
	} else {
		w.serveWebRTCListener(ctx, codec, webRTCSessionPublisher)
	}

	hUdp.unregister <- w.c

	log.Infof("STOP SERVE WEB RTC")

	return err
}

func (w *WebRTCSession) getCodec(ctx context.Context) (codecOption CodecOptions, ok bool) {
	codecOption = 0
	ok = false

	if features.IsActive(ctx, "forcecodec") {
		switch features.GetVariant(ctx, "forcecodec") {
		case "VP8":
			return CodecVP8, true
		case "H264":
			return CodecH264, true
		}
	}

	sdpAnswer := w.sdpCtx.answer
	if sdpAnswer == nil {
		return
	}
	firstMediaVideo := sdpAnswer.Data.GetFirstMediaVideo()
	if firstMediaVideo == nil {
		return
	}

	videoRtpMap := firstMediaVideo.RtpMap
	var codecStr string
	for _, rtp := range videoRtpMap {
		if rtp.Codec != "rtx" {
			codecStr = rtp.Codec
			break
		}
	}

	switch codecStr {
	case "H264":
		codecOption = CodecH264
		ok = true
	case "VP8":
		codecOption = CodecVP8
		ok = true
	default:
		log := plogger.FromContextSafe(ctx).Prefix("WebRTC").Tag("webrtc-session")
		log.Errorf("found unknown codec string '%v' in session's SDP answer %#v", codecStr, videoRtpMap)
		return
	}
	return
}

// JSON marshaling
type jsonWebRTCSession struct {
	Mode                   string       `json:"mode"`
	ListenPort             int          `json:"listenPort"`
	CodecName              string       `json:"codecName"`
	SsrcId                 uint32       `json:"ssrcId"`
	PayloadType            uint16       `json:"payloadType"`
	RtxPayloadType         uint16       `json:"rtxPayloadType"`
	ClockRate              uint32       `json:"clockRate"`
	StunCtx                *StunContext `json:"stunCtx"`
	SdpCtx                 *SdpContext  `json:"sdpCtx"`
	LastRembs              []int        `json:"lastRembs"`
	LastEncodingBitrate    []int        `json:"lastEncodingBitrate"`
	LastBandwidthEstimates []uint64     `json:"lastBandwidthEstimates"`
}

func newJsonWebRTCSession(w *WebRTCSession) jsonWebRTCSession {
	mode := "publisher"
	if w.mode == WebRTCModeListener {
		mode = "listener"
	}

	ctx := getServerStateContext()
	codec, codecOk := w.getCodec(ctx)

	codecName, ssrcId, payloadType, rtxPayloadType, clockRate := "NONE", uint32(0), uint16(0), uint16(0), uint32(0)
	if codecOk {
		switch codec {
		case CodecVP8:
			codecName = "VP8"
		case CodecH264:
			codecName = "H264"
		}
		ssrcId = w.sdpCtx.offer.GetVideoSSRC()
		if w.sdpCtx.answer != nil {
			payloadType = w.sdpCtx.answer.GetVideoPayloadType(codecName)
			rtxPayloadType = w.sdpCtx.answer.GetRtxPayloadType(codecName)
			clockRate = w.sdpCtx.answer.GetVideoClockRate(codecName)
		}
	}

	return jsonWebRTCSession{
		mode,
		w.listenPort,
		codecName,
		ssrcId,
		payloadType,
		rtxPayloadType,
		clockRate,
		w.stunCtx,
		w.sdpCtx,
		w.lastRembs,
		w.lastEncodingBitrate,
		w.lastBandwidthEstimates,
	}
}

func (w *WebRTCSession) MarshalJSON() ([]byte, error) {
	return json.Marshal(newJsonWebRTCSession(w))
}

func (m *WebRTCSessionMap) MarshalJSON() ([]byte, error) {
	m.Lock()
	defer m.Unlock()

	return json.Marshal(m.d)
}
