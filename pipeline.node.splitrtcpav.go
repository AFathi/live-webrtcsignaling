package main

import (
	"context"
	"strings"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeSplitRTCPAV struct {
	PipelineNode
	// I/O
	In                 chan *srtp.PacketRTCP
	OutPacketRTCPAudio chan *srtp.PacketRTCP
	OutPacketRTCPVideo chan *srtp.PacketRTCP
	// private
	audio []uint32
	video []uint32
	exit  chan struct{}
}

func NewPipelineNodeSplitRTCPAV(audio, video []uint32) *PipelineNodeSplitRTCPAV {
	n := new(PipelineNodeSplitRTCPAV)
	n.In = make(chan *srtp.PacketRTCP, 128)
	n.OutPacketRTCPAudio = make(chan *srtp.PacketRTCP, 128)
	n.OutPacketRTCPVideo = make(chan *srtp.PacketRTCP, 128)
	//
	n.audio = audio
	n.video = video
	return n
}

func (n *PipelineNodeSplitRTCPAV) IsAudio(ssrcId uint32) bool {
	for i := 0; i < len(n.audio); i++ {
		if n.audio[i] == ssrcId {
			return true
		}
	}
	return false
}

func (n *PipelineNodeSplitRTCPAV) IsVideo(ssrcId uint32) bool {
	for i := 0; i < len(n.video); i++ {
		if n.video[i] == ssrcId {
			return true
		}
	}
	return false
}

func (n *PipelineNodeSplitRTCPAV) Run(ctx context.Context) {
	n.Running = true
	n.emitStart()
	log := plogger.FromContextSafe(ctx).Prefix("SplitAV").Prefix("RTCP")

	// tempfix
	rtcpParser := rtcp.NewParser(rtcp.Dependencies{Logger: log})

	for {
		select {
		case <-ctx.Done():
			n.onStop(ctx)
			return
		case packetRTCP := <-n.In:
			if packetRTCP.GetSize() < 12 {
				log.Warnf("udp packet length should be > 12")
				continue
			}
			ssrcId := packetRTCP.GetSSRCid()
			switch {
			case n.IsAudio(ssrcId):
				select {
				case n.OutPacketRTCPAudio <- packetRTCP:
				default:
					log.Warnf("OutPacketRTCPAudio is full, dropping packet from In")
				}
			case n.IsVideo(ssrcId):
				select {
				case n.OutPacketRTCPVideo <- packetRTCP:
				default:
					log.Warnf("OutPacketRTCPVideo is full, dropping packet from In")
				}
			default:
				audiosIds := strings.Join(uint32sToStrings(n.audio), ",")
				videosIds := strings.Join(uint32sToStrings(n.video), ",")
				log.Warnf("packet format unknown, pt=%d ssrcId=%d VS audioList=%s & videoList=%s", packetRTCP.GetPT(), ssrcId, audiosIds, videosIds)
				packets, err := rtcpParser.Parse(packetRTCP)
				if err != nil {
					log.Errorf(err.Error())
				} else {
					for i := 0; i < len(packets); i++ {
						log.Warnf("UNKNOWN PACKET %d: %v", i, packets[i])
					}
				}
			}
		}
	}
}
