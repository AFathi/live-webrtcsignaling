package main

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeRTCPReporterRR struct {
	PipelineNode
	// I/O
	InRTP  chan *srtp.PacketRTP
	InRTCP chan *srtp.PacketRTCP
	Out    chan *rtcp.PacketRR
	// private
	reporter *rtcp.ReporterRR
	ssrc     uint32
	rate     uint32
}

func NewPipelineNodeRTCPReporterRR(ssrc uint32, rate uint32) *PipelineNodeRTCPReporterRR {
	n := new(PipelineNodeRTCPReporterRR)
	//
	n.InRTP = make(chan *srtp.PacketRTP, 1000)
	n.InRTCP = make(chan *srtp.PacketRTCP, 1000)
	n.Out = make(chan *rtcp.PacketRR, 1000)
	//
	n.ssrc = ssrc
	n.rate = rate
	return n
}

/*
 * @see https://github.com/versatica/mediasoup/blob/master/worker/src/RTC/RtpStream.cpp
 *
 */
func (n *PipelineNodeRTCPReporterRR) Run(ctx context.Context) {
	n.Running = true
	n.emitStart()
	//
	log := plogger.FromContextSafe(ctx)
	// start reporter
	reporter := rtcp.NewReporterRR()
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
				log.Warnf("reporter.InRTP is full, dropping packet from n.InRTP")
			}
		case packet := <-n.InRTCP:
			select {
			case reporter.InRTCP <- packet:
			default:
				log.Warnf("reporter.InRTCP is full, dropping packet from n.InRTCP")
			}
		case packet := <-reporter.Out:
			select {
			case n.Out <- packet:
			default:
				log.Warnf("n.Out is full, dropping packet from reporter.Out")
			}
		case stats := <-reporter.OutStats:
			msg := new(PipelineMessageRRStats)
			msg.InterarrivalDifference = stats.InterarrivalDifference
			msg.InterarrivalJitter = stats.InterarrivalJitter
			msg.SSRC = stats.SSRC
			select {
			case n.Bus <- msg:
			default:
				log.Warnf("Bus is full, dropping packet stats from reporter.OutStats")
			}
		}
	}
}
