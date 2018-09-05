package main

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

/*
 * This pipeline node is used to add rtp extension header orientation 3gpp.
 * FIXME: link to rfc
 */
type PipelineNodeOrient struct {
	PipelineNode
	// I/O
	In     chan *srtp.PacketRTP
	Out    chan *srtp.PacketRTP
	Orient chan struct{}
}

func NewPipelineNodeOrient() *PipelineNodeOrient {
	n := new(PipelineNodeOrient)
	n.In = make(chan *srtp.PacketRTP, 1000)
	n.Out = make(chan *srtp.PacketRTP, 1000)
	n.Orient = make(chan struct{}, 10)
	return n
}

func (n *PipelineNodeOrient) Run(ctx context.Context) {
	log := plogger.FromContextSafe(ctx)
	n.Running = true
	n.emitStart()
	for {
		select {
		case <-ctx.Done():
			n.onStop(ctx)
			return
		case packet := <-n.In:
			select {
			case <-n.Orient:
				log.Errorf("ORIENTING PACKET")
				var o = packet.GetData() // old data
				var data []byte
				// set extension
				data = append(data, o[0]|0x10)
				data = append(data, o[1:12]...)
				// insert extension
				data = append(data, 0xBE, 0xDE)
				data = append(data, 0x00, 0x01)
				data = append(data, 0x40, 0x00, 0x00, 0x00)
				// following data
				data = append(data, o[12:]...)
				packet.SetData(data)
			default:
				// skip
			}
			select {
			case n.Out <- packet:
			default:
				log.Warnf("nodeOrient.Out is full, dropping nodeOrient.In packet")
			}
		}
	}
}
