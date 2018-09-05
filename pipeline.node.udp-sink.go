package main

import (
	"context"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeUDPSink struct {
	PipelineNode
	// I/O
	InRTP  chan *srtp.PacketRTP
	InRTCP chan *RtpUdpPacket
	// privates
	cUdp *connectionUdp
}

func NewPipelineNodeUDPSink(cUdp *connectionUdp) *PipelineNodeUDPSink {
	n := new(PipelineNodeUDPSink)
	n.cUdp = cUdp
	n.InRTP = make(chan *srtp.PacketRTP, 128)
	n.InRTCP = make(chan *RtpUdpPacket, 128)
	return n
}

// IPipelinePipelineNode interface functions
func (n *PipelineNodeUDPSink) Run(ctx context.Context) {
	var totalPacketsSentSize uint64
	var lastPacketsSentSize uint64

	n.Running = true
	n.emitStart()
	//
	log := plogger.FromContextSafe(ctx).Prefix("PipelineNodeUDPSink")

	go func() {
		// monitoring inputs
		ticker := time.NewTicker(1 * time.Second)
		defer func() {
			ticker.Stop()
		}()
		for {
			select {
			case <-ctx.Done():
				n.onStop(ctx)
				return
			case packetRTP := <-n.InRTP:
				totalPacketsSentSize = totalPacketsSentSize + uint64(packetRTP.GetSize())
				n.cUdp.writeSrtpTo(ctx, packetRTP)
			case packetRTCP := <-n.InRTCP:
				// FIXME: include rtcp inside bandwidth
				n.cUdp.writeSrtpRtcpTo(ctx, packetRTCP)
			case <-ticker.C:
				msg := new(PipelineMessageOutBps)
				msg.Bps = totalPacketsSentSize - lastPacketsSentSize
				lastPacketsSentSize = totalPacketsSentSize
				select {
				case n.Bus <- msg:
				default:
					log.Warnf("Bus is full, dropping event PipelineMessageOutBps")
				}
			}
		}
	}()
}
