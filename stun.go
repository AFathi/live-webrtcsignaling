package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"net"
	"strings"
	"time"
	"sync"

	"encoding/json"
	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/my"
	"github.com/heytribe/live-webrtcsignaling/packet"
)

const (
	STUN_REQ_TTL_SECS                 = 5
	STUN_DELETE_EXPIRED_INTERVAL_SECS = 10
)

type StunTransactionsMap struct {
	my.NamedRWMutex
	Data map[string]*StunRequest
}

func NewStunTransactionsMap(ctx context.Context) *StunTransactionsMap {
	m := new(StunTransactionsMap)
	m.Data = make(map[string]*StunRequest)
	m.NamedRWMutex.Init("StunTMap")

	// clean expired entries periodically
	ticker := time.NewTicker(time.Second * STUN_DELETE_EXPIRED_INTERVAL_SECS)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.DeleteExpired(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	return m
}

func (stm *StunTransactionsMap) Set(ctx context.Context, key string, value *StunRequest) {
	stm.Lock(ctx)
	defer stm.Unlock(ctx)
	value.ttl = time.Now().Add(time.Second * STUN_REQ_TTL_SECS)
	stm.Data[key] = value
}

func (stm *StunTransactionsMap) Get(ctx context.Context, key string) (s *StunRequest) {
	stm.RLock(ctx)
	defer stm.RUnlock(ctx)
	s = stm.Data[key]
	return
}

func (stm *StunTransactionsMap) Delete(ctx context.Context, key string) {
	stm.Lock(ctx)
	defer stm.Unlock(ctx)
	delete(stm.Data, key)
}

func (stm *StunTransactionsMap) DeleteExpired(ctx context.Context) {
	stm.Lock(ctx)
	defer stm.Unlock(ctx)
	now := time.Now()
	for key, stunRequest := range stm.Data {
		if stunRequest.ttl.Before(now) {
			delete(stm.Data, key)
		}
	}
	return
}

type StunHeader struct {
	messageType   uint16
	messageLength uint16
	magicCookie   uint32
	transactionId [12]byte
}

type StunAttributeHeader struct {
	typ    uint16
	length uint16
	value  []byte
}

type StunMessage struct {
	b          []byte
	tieBreaker []byte
}

type StunRequest struct {
	username      *string
	password      *string
	transactionId *string
	useCandidate  bool
	priority      uint32
	mappedAddress string
	ttl           time.Time
}

var bin = binary.BigEndian

func (m *StunMessage) Init(tieBreaker []byte) {
	m.tieBreaker = tieBreaker
}

func (m *StunMessage) AddPadding(length uint16) {
	padding := 4 - length%4
	if padding == 0 {
		return
	}
	b := make([]byte, padding)
	m.b = append(m.b, b...)

	return
}

func (m *StunMessage) AddXorMappedAddress(rAddr *net.UDPAddr) (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a XOR-MAPPED-ADDRESS attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}
	if len(rAddr.IP) != 4 {
		err = errors.New(fmt.Sprintf("remote ip address format %#v is not supported (IPv4 only)", rAddr.IP))
		return
	}

	rIpUint := bin.Uint32(rAddr.IP[:])
	b = make([]byte, 12)
	sAttrHeader.typ = 0x0020
	sAttrHeader.length = 8
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	b[4] = 0x00 // Reserved
	b[5] = 0x01 // IPv4
	bin.PutUint16(b[6:8], uint16(rAddr.Port)^0x2112)
	bin.PutUint32(b[8:12], rIpUint^0x2112a442)

	m.b = append(m.b, b...)

	return
}

func (m *StunMessage) AddUsername(username string, password string) (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a USERNAME attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}
	str := fmt.Sprintf("%s:%s", username, password)
	b = make([]byte, 4+len(str))
	sAttrHeader.typ = 0x0006
	sAttrHeader.length = uint16(len(str))
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	copy(b[4:], []byte(str))
	m.b = append(m.b, b...)
	m.AddPadding(sAttrHeader.length)

	return
}

func (m *StunMessage) AddMessageIntegrity(key string) (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a MESSAGE-INTEGRITY attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}

	b = make([]byte, 4)
	sAttrHeader.typ = 0x0008
	sAttrHeader.length = 20
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	mac := hmac.New(sha1.New, []byte(key))
	// Update STUN header length with the size of MESSAGE-INTEGRITY attribute before computing HMAC-SHA1
	newLength := uint16(len(m.b) + 4)
	bin.PutUint16(m.b[2:4], newLength)
	mac.Write(m.b)
	hmacSha1 := mac.Sum(nil)
	m.b = append(m.b, b...)
	m.b = append(m.b, hmacSha1...)

	return
}

func (m *StunMessage) AddFingerprint() (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a FINGERPRINT attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}
	b = make([]byte, 8)
	sAttrHeader.typ = 0x8028
	sAttrHeader.length = 4
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	// Update STUN header length with the size of FINGERPRINT attribute before computing CRC32
	newLength := uint16(len(m.b) - 12)
	bin.PutUint16(m.b[2:4], newLength)
	crc := crc32.ChecksumIEEE(m.b) ^ 0x5354554e
	bin.PutUint32(b[4:8], crc)
	m.b = append(m.b, b...)

	return
}

func (m *StunMessage) AddPriority(prio uint32) (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a PRIORITY attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}
	b = make([]byte, 8)
	sAttrHeader.typ = 0x0024
	sAttrHeader.length = 4
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	bin.PutUint32(b[4:8], prio)
	m.b = append(m.b, b...)

	return
}

func (m *StunMessage) AddIceControlled() (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a ICE-CONTROLLED attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}
	if len(m.tieBreaker) != 8 {
		err = errors.New("tieBreaker should heave a length of exactly 8 bytes")
		return
	}
	b = make([]byte, 12)
	sAttrHeader.typ = 0x8029
	sAttrHeader.length = 8
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	copy(b[4:12], m.tieBreaker[0:8])
	m.b = append(m.b, b...)

	return
}

func (m *StunMessage) AddIceControlling() (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a ICE-CONTROLLING attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}
	if len(m.tieBreaker) != 8 {
		err = errors.New("tieBreaker should heave a length of exactly 8 bytes")
		return
	}
	b = make([]byte, 12)
	sAttrHeader.typ = 0x802a
	sAttrHeader.length = 8
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	copy(b[4:12], m.tieBreaker[0:8])
	m.b = append(m.b, b...)

	return
}

func (m *StunMessage) AddUseCandidate() (err error) {
	var b []byte
	var sAttrHeader StunAttributeHeader

	if m.b == nil || len(m.b) < 20 {
		err = errors.New("could not add a USE-CANDIDATE attribute if the packet is not initialized correctly (missing STUN header)")
		return
	}
	b = make([]byte, 4)
	sAttrHeader.typ = 0x0025
	sAttrHeader.length = 0
	bin.PutUint16(b[0:2], sAttrHeader.typ)
	bin.PutUint16(b[2:4], sAttrHeader.length)
	m.b = append(m.b, b...)

	return
}

func (m *StunMessage) UpdateLength() (err error) {
	if m.b == nil {
		err = errors.New("b is nil, could not update STUN packet that is uninitialized")
		return
	}
	newLength := uint16(len(m.b) - 20)
	bin.PutUint16(m.b[2:4], newLength)

	return
}

func (m *StunMessage) BuildBindingResponse(ctx context.Context, rAddr *net.UDPAddr, stunRequest StunRequest, transactionId [12]byte, icePwd string) (err error) {
	log, _ := plogger.FromContext(ctx)
	if stunRequest.username == nil || stunRequest.password == nil {
		err = errors.New(fmt.Sprintf("username and/or password are nil on stunRequest, could not build STUN response"))
		return
	}

	// Set STUN header
	m.b = make([]byte, 20)
	bin.PutUint16(m.b[0:2], 0x0101)
	bin.PutUint32(m.b[4:8], 0x2112a442)
	copy(m.b[8:20], transactionId[:])

	// Add XOR-MAPPED-ADDRESS attribute
	err = m.AddXorMappedAddress(rAddr)
	if log.OnError(err, "[ error ] could not build attribute XOR-MAPPED-ADDRESS") {
		return
	}
	// Add USERNAME attribute
	m.AddUsername(*stunRequest.username, *stunRequest.password)
	// Add MESSAGE-INTEGRITY attribute
	/*sessionKey := *stunRequest.username + ":" + *stunRequest.password
	sdpData := sdpSessions.Get(sessionKey)
	if sdpData == nil {
		err = errors.New(fmt.Sprintf("SDP session did not exist -- sdpSessions.Get(%s) == nil", sessionKey))
		return
	}*/
	m.AddMessageIntegrity(icePwd)
	m.AddFingerprint()
	err = m.UpdateLength()
	if err != nil {
		return
	}

	return
}

func (m *StunMessage) BuildBindingRequest(ctx context.Context, rAddr *net.UDPAddr, stunRequest StunRequest, icePwd string, stunMode StunMode) (err error) {
	// Set STUN header
	m.b = make([]byte, 20)
	bin.PutUint16(m.b[0:2], 0x0001)
	bin.PutUint32(m.b[4:8], 0x2112a442)
	transactionId := generateSliceRand(12)
	copy(m.b[8:20], transactionId[:])

	// Add PRIORITY attribute
	m.AddPriority(1845494271)
	// Add ICE-CONTROLLED attribute
	switch stunMode {
	case StunAnswererMode:
		m.AddIceControlled()
	case StunOffererMode:
		m.AddUseCandidate()
		m.AddIceControlling()
	default:
		err = errors.New(fmt.Sprintf("unknown stunMode of type %d", stunMode))
		return
	}
	// Add USERNAME attribute
	m.AddUsername(*stunRequest.username, *stunRequest.password)
	// Add MESSAGE-INTEGRITY
	/*sessionKey := *stunRequest.username + ":" + *stunRequest.password
	sdpData := sdpSessions.Get(sessionKey)
	if sdpData == nil {
		err = errors.New(fmt.Sprintf("SDP sessions did not exist -- sdpSessions.Get(%s) == nil", sessionKey))
		return
	}*/
	m.AddMessageIntegrity(icePwd)
	m.AddFingerprint()
	// Update the length of the STUN message (STUN Header)
	err = m.UpdateLength()
	if err != nil {
		return
	}

	// Add stun request transaction Id / stunRequest infos to stunTransactions
	key := fmt.Sprintf("%X", transactionId)
	stunTransactions.Set(ctx, key, &stunRequest)

	return
}

func (m *StunMessage) BuildBindingIndication() (err error) {
	// Set STUN header
	m.b = make([]byte, 20)
	bin.PutUint16(m.b[0:2], 0x0011)
	bin.PutUint32(m.b[4:8], 0x2112a442)
	transactionId := generateSliceRand(12)
	copy(m.b[8:20], transactionId[:])

	// Add FINGEPRINT
	m.AddFingerprint()
	// Update the length of the STUN message (STUN Header)
	err = m.UpdateLength()
	if err != nil {
		return
	}

	return
}

type StunMode int

const (
	StunOffererMode  StunMode = 0
	StunAnswererMode StunMode = 1
)

type StunState int

const (
	StunStateInit      StunState = 0
	StunStateCompleted StunState = 1
)

type StunContext struct {
	sync.RWMutex
	State              StunState
	ChState            chan StunState
	RAddr              *net.UDPAddr
	sdpCtx             *SdpContext
	icePwdLocal        string
	icePwdRemote       string
	mode               StunMode
	requestTs          time.Time
	rtt                *int64
	monitorStarted     bool
	monitorStunRequest StunRequest
	monitorRAddr       *net.UDPAddr
}

func NewStunCtx(ctx context.Context, key string, sdpCtx *SdpContext, mode StunMode) (stunCtx *StunContext) {
	log := plogger.FromContextSafe(ctx).Prefix("STUN").Tag("stun")
	ctx = plogger.NewContext(ctx, log)
	stunCtx = new(StunContext)
	stunCtx.State = StunStateInit
	stunCtx.RAddr = nil
	stunCtx.ChState = make(chan StunState)
	stunCtx.sdpCtx = sdpCtx
	stunCtx.mode = mode
	if stunCtx.mode == StunAnswererMode {
		stunCtx.icePwdLocal = sdpCtx.answer.Data.Medias[0].IcePwd
		stunCtx.icePwdRemote = sdpCtx.offer.Data.Medias[0].IcePwd
	} else {
		stunCtx.icePwdLocal = sdpCtx.offer.Data.Medias[0].IcePwd
		stunCtx.icePwdRemote = sdpCtx.answer.Data.Medias[0].IcePwd
	}
	stunCtx.rtt = new(int64)
	log.Debugf("Local icePwd is %s, Remote icePwd is %s", stunCtx.icePwdLocal, stunCtx.icePwdRemote)
	return
}

func (stunCtx *StunContext) monitorRTT(ctx context.Context, c *connectionUdp) {
	var err error
	var udpPacket *packet.UDP

	log := plogger.FromContextSafe(ctx).Prefix("STUN").Tag("stun")
	ctx = plogger.NewContext(ctx, log)
	for {
		select {
		case <-ctx.Done():
			log.Infof("go func monitorRTT exiting")
			return
		default:
			if stunCtx.RAddr != nil {
				var m2 StunMessage
				m2.Init(c.tieBreaker)
				err = m2.BuildBindingRequest(ctx, stunCtx.monitorRAddr, stunCtx.monitorStunRequest, stunCtx.icePwdRemote, stunCtx.mode)
				if err != nil {
					log.Warnf("could not create a STUN binding request: %s", err.Error())
				} else {
					udpPacket = packet.NewUDPFromData(m2.b, stunCtx.monitorRAddr)
					stunCtx.requestTs = time.Now()
					log.Debugf("Sending a binding request @ %d", time.Now().UnixNano())
					c.send <- udpPacket
				}
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (stunCtx *StunContext) handleStunMessage(ctx context.Context, c *connectionUdp, ipacket *packet.UDP) (err error) {
	var sHeader StunHeader
	var udpPacket *packet.UDP

	if stunCtx.monitorStarted == false {
		go stunCtx.monitorRTT(ctx, c)
		stunCtx.monitorStarted = true
	}
	buf := ipacket.GetData()
	rAddr := ipacket.GetRAddr()
	log := plogger.FromContextSafe(ctx).Prefix("STUN").Tag("stun")
	ctx = plogger.NewContext(ctx, log)
	// Checking size of buf, if it's less than 20 bytes, it's not a STUN package (RFC violation)
	if len(buf) < 20 {
		err = errors.New("invalid STUN packet received, the length of packet is less than 20 bytes (RFC violation)")
		return
	}

	sHeader.messageType = bin.Uint16(buf[0:2])
	sHeader.messageLength = bin.Uint16(buf[2:4])
	sHeader.magicCookie = bin.Uint32(buf[4:8])
	copy(sHeader.transactionId[:], buf[8:20])

	// Checking if two first bits are 00
	if buf[0]&0xc0 != 0 {
		err = errors.New("invalid STUN packet received, the two first bits of messageType should be set to 00")
		return
	}

	// Checking if the magic cookie is good
	if sHeader.magicCookie != 0x2112a442 {
		err = errors.New(fmt.Sprintf("invalid STUN packet received, the magic cookie 0x%x is wrong, expected 0x2112a442", sHeader.magicCookie))
		return
	}

	// Checking if the message Length is a multiple of 4 bytes
	// If not, it's not a valid STUN packet, reject it
	if sHeader.messageLength&0x03 != 0 {
		err = errors.New("invalid STUN packet received, the message length is not a multiple of 4 bytes (RFC violation)")
		return
	}

	// If there is no attributes, discard the packet silently
	if sHeader.messageLength == 0x14 {
		err = errors.New("no STUN attributes, discard it")
		return
	}

	// Checking if the packet is a request or a response
	switch sHeader.messageType {
	case 0x0001:
		// Binding Request
		var stunRequest StunRequest
		stunRequest, err = stunCtx.decodeStun(ctx, buf, true)
		if log.OnError(err, "[ error ] could not decode stun message") {
			return
		}

		// Stun packet is OK - changing state in sdpSessions
		/*sessionKey := *stunRequest.username + ":" + *stunRequest.password
		sdpData := sdpSessions.Get(sessionKey)
		sdpData.iceState = `completed`*/
		stunCtx.sdpCtx.iceState = `completed`

		var m StunMessage
		m.Init(c.tieBreaker)
		err = m.BuildBindingResponse(ctx, rAddr, stunRequest, sHeader.transactionId, stunCtx.icePwdLocal)
		if log.OnError(err, "[ error ] could not build STUN binding response") {
			// XXX Should build a STUN ERROR MESSAGE here
			err = nil
			return
		}
		udpPacket = packet.NewUDPFromData(m.b, rAddr)
		c.send <- udpPacket
		if log.OnError(err, "[ error ] could not write response packet %#v to %#v", m.b, rAddr) {
			return
		}

		stunCtx.sdpCtx.iceCandidatePort = rAddr.Port
		//sdpSessions.Set(sessionKey, sdpData)

		// Now build the STUN request to the server
		if c.dtlsState == DtlsStateNone {
			username := stunRequest.username
			stunCtx.monitorRAddr = rAddr
			stunRequest.username = stunRequest.password
			stunRequest.password = username
			stunCtx.monitorStunRequest = stunRequest
			var m2 StunMessage
			m2.Init(c.tieBreaker)
			err = m2.BuildBindingRequest(ctx, rAddr, stunRequest, stunCtx.icePwdRemote, stunCtx.mode)
			udpPacket = packet.NewUDPFromData(m2.b, rAddr)
			stunCtx.requestTs = time.Now()
			log.Debugf("Sending a binding request @ %d", time.Now().UnixNano())
			c.send <- udpPacket
		}
	case 0x0101:
		// Binding Response
		responseTs := time.Now()
		var stunRequest StunRequest
		stunRequest, err = stunCtx.decodeStun(ctx, buf, false)
		if log.OnError(err, "[ error ] could not decode stun message") {
			return
		}

		if stunRequest.transactionId == nil {
			log.Infof("[ error ] transactionId is nil, could not know the original request infos")
			return
		}

		// XXX Should check IP:Port corresponding to the table here - SANITY CHECKS

		// Get stunRequest origin infos
		key := *stunRequest.transactionId
		stunOriginRequest := stunTransactions.Get(ctx, key)
		if stunOriginRequest == nil {
			log.Infof("[ error ] could not know the origin infos associated to transaction ID, stunTransactions.Get(%s) == nil", stunRequest.transactionId)
			return
		}
		stunTransactions.Delete(ctx, key)

		// Stun packet is OK - changing state in sdpSessions
		/*sessionKey := *stunOriginRequest.username + ":" + *stunOriginRequest.password
		sdpData := sdpSessions.Get(sessionKey)
		if sdpData == nil {
			logger.Infof("[ error ] could not found the origin request for this reponses sdpSessions.Get(%s) == nil", sessionKey)
			return
		}*/

		*stunCtx.rtt = responseTs.UnixNano() - stunCtx.requestTs.UnixNano()
		log.Debugf("RTT is %f ms", float64(*stunCtx.rtt)/1000000)

		// It's OK we could send the binding indication
		if c.dtlsState == DtlsStateNone {
			var m3 StunMessage
			m3.Init(c.tieBreaker)
			err = m3.BuildBindingIndication()
			if log.OnError(err, "[ error ] could not build the binding indication STUN message") {
				return
			}
			udpPacket = packet.NewUDPFromData(m3.b, rAddr)
			c.send <- udpPacket

			log.Debugf("[ STUN ] Probably STUN is about to complete and create DTLS session")
			log.Debugf("[ STUN ] rAddr.Port is %d and stunCtx.sdpCtx.iceCandidatePort is %d", rAddr.Port, stunCtx.sdpCtx.iceCandidatePort)

			stunCtx.RLock()
			defer stunCtx.RUnlock()
			if stunCtx.State != StunStateCompleted && ((stunCtx.sdpCtx.iceCandidatePort == 0) || (rAddr.Port == stunCtx.sdpCtx.iceCandidatePort)) {
				stunCtx.sdpCtx.iceState = `completed`
				stunCtx.RAddr = rAddr
				stunCtx.State = StunStateCompleted
				log.Debugf("posting to stunCtx.ChState -> state completed stunCtx.ChState is %p", stunCtx.ChState)
				go func(ch chan StunState) {
					ch <- StunStateCompleted
				}(stunCtx.ChState)
			}
		}
	}

	return
}

func (stunCtx *StunContext) checkMessageIntegrity(buf []byte, sAttrHeader *StunAttributeHeader, stunRequest *StunRequest, icePwd string) (err error) {
	// Get ice key from sdpSessions
	/*sessionKey := *stunRequest.username + ":" + *stunRequest.password
	sdpData := sdpSessions.Get(sessionKey)
	if sdpData == nil {
		err = errors.New(fmt.Sprintf("could not check MESSAGE-INTEGRITY, no SDP session attached to key %s", sessionKey))
		return
	}*/

	// newLength is len(buffer) - 20 bytes (Stun Header) + 24 bytes (STUN attribute MESSAGE-INTEGRITY length) -> +4
	newLength := uint16(len(buf) + 4)
	// Setting the size to len(buf) - 20 bytes STUN header
	bin.PutUint16(buf[2:4], newLength)

	mac := hmac.New(sha1.New, []byte(icePwd))
	mac.Write(buf)
	hmacSha1 := mac.Sum(nil)
	if hmac.Equal(hmacSha1, sAttrHeader.value) == false {
		err = errors.New(fmt.Sprintf("invalid STUN packet, MESSAGE-INTEGRITY error on HMAC-SHA1, packet value = %X, expected = %X", sAttrHeader.value, hmacSha1))
		return
	}

	return
}

func (stunCtx *StunContext) decodeStun(ctx context.Context, buf []byte, request bool) (stunRequest StunRequest, err error) {
	var (
		bufPos            uint16
		stunOriginRequest *StunRequest
	)

	log, _ := plogger.FromContext(ctx)
	if len(buf) < 20 {
		err = errors.New("invalid STUN packet, the packet is less than 20 bytes")
		return
	}

	stunRequest.useCandidate = false
	stunRequest.transactionId = new(string)
	*stunRequest.transactionId = fmt.Sprintf("%X", buf[8:20])
	if request == false {
		key := *stunRequest.transactionId
		log.Infof("Searching transactionId %s", key)
		stunOriginRequest = stunTransactions.Get(ctx, key)
		if stunOriginRequest == nil {
			err = errors.New("invalid STUN response, no origin request found for this one")
			return
		}
	}

	bufPos = 20
	bufLen := uint16(len(buf))
	// buf length should be 32bits minimum (then 32 bit boundary)
	if bufLen < 4 {
		err = errors.New(fmt.Sprintf("invalid STUN packet, total attributes length is less than 4 bytes (%d)", bufLen))
		return
	}
	for bufPos < bufLen {
		// Decoding attribute header
		var sAttrHeader StunAttributeHeader

		sAttrHeader.typ = bin.Uint16(buf[bufPos : bufPos+2])
		sAttrHeader.length = bin.Uint16(buf[bufPos+2 : bufPos+4])
		padding := sAttrHeader.length % 4
		if padding != 0 {
			padding = 4 - padding
		}

		if bufPos+sAttrHeader.length+padding > bufLen {
			err = errors.New(fmt.Sprintf("invalid STUN attribute packet, attribute length (%d bytes) + padding (%d bytes) is bigger than the rest of UDP packet (%d bytes)", bufPos+4+sAttrHeader.length, padding, bufLen))
			return
		}
		sAttrHeader.value = buf[bufPos+4 : bufPos+4+sAttrHeader.length]

		switch sAttrHeader.typ {
		// Comprehension required attributes
		case 0x0001:
			// MAPPED-ADDRESS
		case 0x0006:
			// USERNAME
			str := string(sAttrHeader.value)
			s := strings.Split(str, `:`)
			if len(s) < 2 {
				err = errors.New("invalid STUN attribute packet USERNAME, the value should be of type user:password")
				return
			}
			stunRequest.username = &s[0]
			stunRequest.password = &s[1]
		case 0x0008:
			// MESSAGE-INTEGRITY
			if request == true {
				if stunRequest.password == nil {
					err = errors.New("recevied MESSAGE-INTEGRITY before USERNAME attribute")
					return
				}
				err = stunCtx.checkMessageIntegrity(buf[:bufPos], &sAttrHeader, &stunRequest, stunCtx.icePwdLocal)
			} else {
				err = stunCtx.checkMessageIntegrity(buf[:bufPos], &sAttrHeader, stunOriginRequest, stunCtx.icePwdRemote)
			}
			if err != nil {
				return
			}
		case 0x000A:
			// UNKNOWN-ATTRIBUTES
		case 0x0014:
			// REALM
		case 0x0015:
			// NONCE
		case 0x0020:
			// XOR-MAPPED-ADDRESS
			if sAttrHeader.length != 8 {
				err = errors.New("invalid STUN attribute packet XOR-MAPPED-ADDRESS, length should be 8 bytes / IPv4 only")
				return
			}
			ipv4 := bin.Uint32(sAttrHeader.value[4:8]) ^ 0x2112a442
			port := bin.Uint16(sAttrHeader.value[2:4]) ^ 0x2112
			stunRequest.mappedAddress = fmt.Sprintf("%d.%d.%d.%d:%d", (ipv4&0xff000000)>>24, (ipv4&0x00ff0000)>>16, (ipv4&0x0000ff00)>>8, ipv4&0x000000ff, port)
		case 0x0024:
			// PRIORITY
			if len(sAttrHeader.value) != 4 {
				err = errors.New(fmt.Sprintf("invalid STUN packet, PRIORITY have a value of %d bytes, expected 4 bytes", len(sAttrHeader.value)))
				return
			}
			stunRequest.priority = bin.Uint32(sAttrHeader.value)
		case 0x0025:
			// USE-CANDIDATE
			stunRequest.useCandidate = true
		// Comprehension optional attributes
		case 0x8022:
			// SOFTWARE
		case 0x8023:
			// ALTERNATE-SERVER
		case 0x8028:
			// FINGERPRINT
		case 0x8029:
			// ICE-ICECONTROLLED
		case 0x802a:
			// ICE-CONTROLLING
		default:
			log.Infof("[ STUN ] Unknown attribute type %.2X", sAttrHeader.typ)
		}

		// bufPos should skip padding 32-bit boundary
		bufPos += 4 + sAttrHeader.length + padding
	}

	return
}

// JSON marshaling
type jsonStunContext struct {
	State string  `json:"state"`
	Mode  string  `json:"mode"`
	RTT   float64 `json:"rtt"`
}

func newJsonStunContext(stunCtx *StunContext) jsonStunContext {
	state := "init"
	if stunCtx.State == StunStateCompleted {
		state = "completed"
	}

	mode := "answered"
	if stunCtx.mode == StunOffererMode {
		mode = "offered"
	}

	return jsonStunContext{
		state,
		mode,
		float64(*stunCtx.rtt) / 1000000,
	}
}

func (stunCtx *StunContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(newJsonStunContext(stunCtx))
}
