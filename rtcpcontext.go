package main

import (
	"context"
	"fmt"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
)

type RtcpContext struct {
	Rembs   *RtcpContextRembs
	ChInfos chan interface{}
}

func NewRtcpContext(ctx context.Context) *RtcpContext {
	c := new(RtcpContext)
	// export info
	c.ChInfos = make(chan interface{}, 128)
	// dependency with config.
	c.Rembs = NewRtcpContextRembs(ctx, c.ChInfos, RTCP_REMB_ALGORITHM_SIMPLE)
	return c
}

func (c *RtcpContext) Push(ctx context.Context, untypedPacket interface{}) {
	log := plogger.FromContextSafe(ctx)
	switch packet := untypedPacket.(type) {
	case *rtcp.PacketALFBRemb:
		c.Rembs.Push(packet)
	case *rtcp.PacketPSFBFir:
		fir := &RtcpContextInfoFIR{
			Date: time.Now(),
		}
		select {
		case c.ChInfos <- fir:
		default:
			log.Warnf("c.ChInfos is full, dropping FIR packet")
		}
	case *rtcp.PacketPSFBPli:
		log.Infof("received a PLI packet !")
		select {
		case c.ChInfos <- packet:
		default:
			log.Warnf("c.ChInfos is full, dropping PLI packet")
		}
	case *rtcp.PacketRTPFBNack:
		log.Infof("received a NACK packet !")
		select {
		case c.ChInfos <- packet:
		default:
			log.Warnf("c.ChInfos is full, dropping NACK packet")
		}
	case *rtcp.PacketRR:
		 log.Infof("received a RR packet")
		 select {
		 case c.ChInfos <- packet:
		 default:
				 log.Warnf("c.ChInfos is full, dropping RR packet")
		 }
	default:
		// nothing
		// fmt.Printf("RTCP : unknown type\n")
	}
}

func (c *RtcpContext) Destroy() {
	c.Rembs.Destroy()
	close(c.ChInfos)
}

func (c *RtcpContext) String() string {
	return fmt.Sprintf(
		"[RTCP-CTX: %s]",
		c.Rembs,
	)
}
