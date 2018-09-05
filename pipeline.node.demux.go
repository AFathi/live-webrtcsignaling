package main

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/packet"
)

type PipelineNodeDemux struct {
	PipelineNode
	// I/O
	In            chan *packet.UDP
	OutPacketErr  chan *packet.UDP
	OutPacketSTUN chan *packet.UDP
	OutPacketDTLS chan *packet.UDP
	OutPacketSRTP chan *packet.UDP
}

func NewPipelineNodeDemux() *PipelineNodeDemux {
	n := new(PipelineNodeDemux)
	n.In = make(chan *packet.UDP, 128)
	n.OutPacketErr = make(chan *packet.UDP, 128)
	n.OutPacketSTUN = make(chan *packet.UDP, 128)
	n.OutPacketDTLS = make(chan *packet.UDP, 128)
	n.OutPacketSRTP = make(chan *packet.UDP, 128)
	return n
}

func (n *PipelineNodeDemux) Run(ctx context.Context) {
	log := plogger.FromContextSafe(ctx).Prefix("DEMUX")
	n.Running = true
	n.emitStart()
	for {
		select {
		case <-ctx.Done():
			n.onStop(ctx)
			return
		case packet := <-n.In:
			switch {
			case packet.IsEmpty():
				select {
				case n.OutPacketErr <- packet:
				default:
					log.Warnf("OutPacketErr is full, dropping from In")
				}
			case packet.IsSTUN():
				select {
				case n.OutPacketSTUN <- packet:
				default:
					log.Warnf("OutPacketSTUN is full, dropping from In")
				}
			case packet.IsDTLS():
				select {
				case n.OutPacketDTLS <- packet:
				default:
					log.Warnf("OutPacketDTLS is full, dropping from In")
				}
			case packet.IsSRTPorSRTCP():
				select {
				case n.OutPacketSRTP <- packet:
				default:
					log.Warnf("OutPacketSRTP is full, dropping from In")
				}
			default:
				if packet.GetSize() > 0 {
					log.Warnf("unknown packet type %b, wtf ? DROP", packet.GetData()[0])
				} else {
					log.Warnf("unknown packet type unknown, wtf ? DROP")
				}
			}
		}
	}
}
