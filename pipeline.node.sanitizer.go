package main

/*
 * this node will sanitize input RTP trafic.
 * packets are fwd to output if :
 *   packet rtp timestamp is +/- 100000 ticks from prev rtp timestamp
 *
 * FIXME
 *  -  rtp timestamps are in the past or future +/- 1 day. ?
 *  - whitefilter payloads
 *  - seq numbers integrity
 *  - header integrity
 *  - ...
 *  - improve timestamp tick
 *
 * we assume first timestamp is correct ....
 */

import (
	"context"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeSanitizer struct {
	PipelineNode
	// I/O
	In  chan *srtp.PacketRTP
	Out chan *srtp.PacketRTP
}

func NewPipelineNodeSanitizer() *PipelineNodeSanitizer {
	n := new(PipelineNodeSanitizer)
	n.In = make(chan *srtp.PacketRTP, 128)
	n.Out = make(chan *srtp.PacketRTP, 128)
	return n
}

func (n *PipelineNodeSanitizer) Run(ctx context.Context) {
	var firstLoop bool = true
	//var lastRtpTimestamp uint32

	n.Running = true
	n.emitStart()
	log := plogger.FromContextSafe(ctx).Prefix("Sanitizer")
	for {
		select {
		case <-ctx.Done():
			n.onStop(ctx)
			return
		case packetRTP := <-n.In:
			//currentTimestamp := packetRTP.GetTimestamp()
			// checking timestamp integrity
			if firstLoop {
				firstLoop = false
			} else {
				// compare timestamp to old one
				// timestamp can loop & be disordered
				/*if (int64(currentTimestamp) < int64(lastRtpTimestamp)+500000 &&
					int64(currentTimestamp) > int64(lastRtpTimestamp)-500000) ||
					(currentTimestamp < 500000 && lastRtpTimestamp > 4294967295-500000) ||
					(lastRtpTimestamp < 500000 && currentTimestamp > 4294967295-500000) {
					// OK
				} else {
					log.Warnf("rtp timestamp out of bounds last=%d current=%d (PT=%d)", lastRtpTimestamp, currentTimestamp, packetRTP.GetPT())
					break
				}*/
			}
			// everything seems normal, proceed
			select {
			case n.Out <- packetRTP:
			default:
				log.Warnf("Out is full, dropping packet from In")
			}
			//lastRtpTimestamp = currentTimestamp
		}
	}
}
