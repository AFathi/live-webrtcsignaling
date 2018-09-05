package main

import (
	"context"
	"strings"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeSplitRTPAV struct {
	PipelineNode
	// I/O
	In                chan *srtp.PacketRTP
	OutPacketRTPAudio chan *srtp.PacketRTP
	OutPacketRTPVideo chan *srtp.PacketRTP
	// private
	audio []uint32
	video []uint32
	exit  chan struct{}
}

func NewPipelineNodeSplitRTPAV(audio, video []uint32) *PipelineNodeSplitRTPAV {
	n := new(PipelineNodeSplitRTPAV)
	n.In = make(chan *srtp.PacketRTP, 128)
	n.OutPacketRTPAudio = make(chan *srtp.PacketRTP, 128)
	n.OutPacketRTPVideo = make(chan *srtp.PacketRTP, 128)
	//
	n.audio = audio
	n.video = video
	return n
}

func (n *PipelineNodeSplitRTPAV) IsAudio(ssrcId uint32) bool {
	for i := 0; i < len(n.audio); i++ {
		if n.audio[i] == ssrcId {
			return true
		}
	}
	return false
}

func (n *PipelineNodeSplitRTPAV) IsVideo(ssrcId uint32) bool {
	for i := 0; i < len(n.video); i++ {
		if n.video[i] == ssrcId {
			return true
		}
	}
	return false
}

func (n *PipelineNodeSplitRTPAV) Run(ctx context.Context) {
	n.Running = true
	n.emitStart()
	log := plogger.FromContextSafe(ctx).Prefix("SplitAV").Prefix("RTP")
	for {
		select {
		case <-ctx.Done():
			n.onStop(ctx)
			return
		case packetRTP := <-n.In:
			log.Debugf("ssrc=%d PT=%d", packetRTP.GetSSRCid(), packetRTP.GetPT())
			if packetRTP.GetSize() < 12 {
				log.Warnf("udp packet length should be > 12")
				continue
			}
			ssrcId := packetRTP.GetSSRCid()
			switch {
			case n.IsAudio(ssrcId):
				select {
				case n.OutPacketRTPAudio <- packetRTP:
				default:
					log.Warnf("OutPacketRTPAudio is full, dropping packet from In")
				}
			case n.IsVideo(ssrcId):
				select {
				case n.OutPacketRTPVideo <- packetRTP:
				default:
					log.Warnf("OutPacketRTPAudio is full, dropping packet from In")
				}
			default:
				audiosIds := strings.Join(uint32sToStrings(n.audio), ",")
				videosIds := strings.Join(uint32sToStrings(n.video), ",")
				log.Warnf("packet  format unknown, pt=%d ssrcId=%d VS audioList=%s & videoList=%s", packetRTP.GetPT(), ssrcId, audiosIds, videosIds)
			}
		}
	}
}
