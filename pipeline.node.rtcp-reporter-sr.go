package main

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeRTCPReporterSR struct {
	PipelineNode
	// I/O
	InRTP chan *srtp.PacketRTP
	Out   chan *rtcp.PacketSR
	// private
	reporter *rtcp.ReporterSR
	ssrc     uint32
	rate     uint32
	exit     chan struct{}
}

func NewPipelineNodeRTCPReporterSR(ssrc uint32, rate uint32) *PipelineNodeRTCPReporterSR {
	n := new(PipelineNodeRTCPReporterSR)
	//
	n.InRTP = make(chan *srtp.PacketRTP, 1000)
	n.Out = make(chan *rtcp.PacketSR, 1000)
	//
	n.ssrc = ssrc
	n.rate = rate
	return n
}

/*
 * @see https://github.com/versatica/mediasoup/blob/master/worker/src/RTC/RtpStream.cpp
 *
 */
func (n *PipelineNodeRTCPReporterSR) Run(ctx context.Context) {
	n.Running = true
	n.emitStart()
	//
	log := plogger.FromContextSafe(ctx).Prefix("ReporterSR")
	ctx = plogger.NewContext(ctx, log)
	// start reporter
	reporter := rtcp.NewReporterSR()
	go reporter.Run(ctx, n.ssrc, n.rate)
	//
	for {
		select {
		case <-ctx.Done():
			n.onStop(ctx)
			return
		case packet := <-n.InRTP:
			select {
			case reporter.InRTP <- packet:
			default:
				log.Warnf("InRTP is full, dropping packet from n.InRTP")
			}
		case packet := <-reporter.Out:
			select {
			case n.Out <- packet:
			default:
				log.Warnf("Out is full, dropping packet from reporter.Out")
			}
		}
	}
}
