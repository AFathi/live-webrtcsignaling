package main

import (
	"context"
	"errors"
	"net"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/dtls"
	"github.com/heytribe/live-webrtcsignaling/my"
	"github.com/heytribe/live-webrtcsignaling/packet"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type DtlsState int

const (
	DtlsStateNone      DtlsState = 0
	DtlsStateCreating  DtlsState = 1
	DtlsStateCreated   DtlsState = 2
	DtlsStateTrying    DtlsState = 3
	DtlsStateConnected DtlsState = 4
	DtlsStateFailed    DtlsState = 5
)

type connectionUdp struct {
	wsConn      *connection
	conn        *net.UDPConn
	dtlsSession *dtls.DTLSSession
	gstSession  *GstSession
	exit        bool
	connMutex   my.NamedMutex
	send        chan *packet.UDP
	when        time.Time
	state       string
	dtlsState   DtlsState
	tieBreaker  []byte
	srtpSession *srtp.SrtpSession
	sdpCtx      *SdpContext
}

// FIXME: refactor.
func NewConnectionUdp(ctx context.Context, udpConn *net.UDPConn, wsConn *connection) *connectionUdp {
	c := new(connectionUdp)
	c.send = make(chan *packet.UDP, 1000)
	c.connMutex.Init("conn")
	c.conn = udpConn
	c.wsConn = wsConn
	c.when = time.Now()
	c.state = `creating`
	c.exit = false
	c.dtlsState = DtlsStateNone
	c.tieBreaker = generateSliceRand(8)
	return c
}

// write writes a message with the given message type and payload.
func (c *connectionUdp) writeTo(ctx context.Context, udpPacket *packet.UDP) (err error) {
	if udpPacket.GetRAddr() == nil {
		err = errors.New("could not send packet remote address is not set")
		return
	}
	c.connMutex.Lock(ctx)
	defer c.connMutex.Unlock(ctx)
	_, err = c.conn.WriteTo(udpPacket.GetData(), udpPacket.GetRAddr())
	return
}

type RtpUdpPacket struct {
	RAddr *net.UDPAddr
	Data  []byte
}

// write writes a message with the given message type and payload.
func (c *connectionUdp) writeSrtpTo(ctx context.Context, rtpPacket *srtp.PacketRTP) (err error) {
	log, _ := plogger.FromContext(ctx)
	if rtpPacket == nil {
		err = errors.New("could not push a nil RTP packet !!!")
		return
	}
	if rtpPacket.GetRAddr() == nil {
		err = errors.New("could not send packet remote address is not set")
		return
	}
	l := len(rtpPacket.GetData())
	d := make([]byte, l, l+256)
	copy(d, rtpPacket.GetData())
	//log.Debugf("[ CONNUDP ] LEN OF DATA UNENCRYPTED IS %d", len(d))
	/*padding := 4 - (l % 4)
	if padding == 0 {
		padding = 4
	}
	endPadding := l + padding
	d = d[:endPadding]
	d[endPadding-1] = byte(padding)
	// Set padding bit to the RTP packet
	d[0] |= (1 << 5)*/
	newSize, err := srtp.Protect(c.srtpSession.SrtpOut, d)
	if err != nil {
		log.Warnf("[ warning ] Could not encrypt SRTP packet data %#v, len=%d, %s", d, len(d), err.Error())
		return nil // not really an error ?
	}
	c.connMutex.Lock(ctx)
	defer c.connMutex.Unlock(ctx)
	_, err = c.conn.WriteTo(d[:newSize], rtpPacket.GetRAddr())
	if err != nil {
		return
	}

	return
}

// write writes a message with the given message type and payload.
func (c *connectionUdp) writeSrtpRtcpTo(ctx context.Context, rtcpPacket *RtpUdpPacket) (err error) {
	log, _ := plogger.FromContext(ctx)
	if rtcpPacket.RAddr == nil {
		err = errors.New("could not send packet remote address is not set")
		return
	}
	c.connMutex.Lock(ctx)
	defer c.connMutex.Unlock(ctx)
	l := len(rtcpPacket.Data)
	d := make([]byte, l, l+256)
	copy(d, rtcpPacket.Data)
	if c.srtpSession == nil {
		err = errors.New("could not send packet, srtpSession is nul")
		return
	}
	newSize, err := srtp.ProtectRtcp(c.srtpSession.SrtpOut, d)
	if err != nil {
		log.Warnf("[ warning ] Could not encrypt SRTP RTCP packet data %#v, len=%d, %s", d, len(d), err.Error())
		return nil // not really an error ?
	}
	//logger.Debugf("rtpPacket.data was len = %d, new Size is %d", l, newSize)
	_, err = c.conn.WriteTo(d[:newSize], rtcpPacket.RAddr)
	//_, err = c.conn.WriteTo(rtpPacket.Data, rtpPacket.RAddr)
	if err != nil {
		return
	}

	return
}

// writePump pumps messages from the hub to the UDP connection.
func (c *connectionUdp) writePump(ctx context.Context) {
	log, _ := plogger.FromContext(ctx)
	defer func() {
		c.conn.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case udpPacket, ok := <-c.send:
			if !ok {
				log.Infof("[ error ] could not receive packet correctly from c.send channel")
				return
			}
			//logger.Infof("[ CONNUDP ] send packet len %d to raddr %#v", len(udpPacket.Data), udpPacket.RAddr)
			//logger.Infof("[ CONNUDP ] sending %s", c.dumpPacketToString(udpPacket.Data))
			//logger.Infof("[ CONNUDP ] sending %#v", udpPacket.Data)
			err := c.writeTo(ctx, udpPacket)
			if log.OnError(err, "[ error ] could not write message on the UDP socket") {
				return
			}
		}
	}
}
