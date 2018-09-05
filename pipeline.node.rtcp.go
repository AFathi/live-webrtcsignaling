package main

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeRTCP struct {
	PipelineNode
	// I/O
	In chan *srtp.PacketRTCP
	// private
	rtcpCtx    *RtcpContext
	rtcpParser *rtcp.Parser
}

func NewPipelineNodeRTCP() *PipelineNodeRTCP {
	n := new(PipelineNodeRTCP)
	n.In = make(chan *srtp.PacketRTCP, 128)
	return n
}

func (n *PipelineNodeRTCP) Run(ctx context.Context) {
	n.Running = true
	n.emitStart()
	// log
	log := plogger.FromContextSafe(ctx)
	// init rtcp context & parser
	n.rtcpCtx = NewRtcpContext(ctx)
	n.rtcpParser = rtcp.NewParser(rtcp.Dependencies{Logger: log.Prefix("IN").Tag("rtcp")})
	for {
		select {
		case <-ctx.Done():
			n.onStop(ctx)
			return
		case compoundRTCP := <-n.In:
			packets, err := n.rtcpParser.Parse(compoundRTCP)
			if err != nil {
				log.Errorf(err.Error())
			} else {
				for i := 0; i < len(packets); i++ {
					n.rtcpCtx.Push(ctx, packets[i])
				}
			}
		case event := <-n.rtcpCtx.ChInfos:
			select {
			case n.Bus <- event:
			default:
				log.Warnf("Bus is full, dropping packet from rtcpCtx.ChInfos")
			}
		}
	}
}
