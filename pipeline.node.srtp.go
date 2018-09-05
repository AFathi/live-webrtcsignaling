package main

import (
	"context"
	"errors"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/packet"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type PipelineNodeSRTP struct {
	PipelineNode
	// I/O
	In            chan *packet.UDP
	OutPacketRTP  chan *srtp.PacketRTP
	OutPacketRTCP chan *srtp.PacketRTCP
	// private
	session *srtp.SrtpSession
}

func NewPipelineNodeSRTP() *PipelineNodeSRTP {
	n := new(PipelineNodeSRTP)
	n.In = make(chan *packet.UDP, 128)
	n.OutPacketRTP = make(chan *srtp.PacketRTP, 128)
	n.OutPacketRTCP = make(chan *srtp.PacketRTCP, 128)
	return n
}

func (n *PipelineNodeSRTP) SetSession(ctx context.Context, session *srtp.SrtpSession) {
	log := plogger.FromContextSafe(ctx).Prefix("PipelineNodeSRTP")

	if n.session == nil {
		n.session = session
	} else {
		log.Warnf("trying to overwrite existing pipeline node srtp session => forbidden => skipping")
	}
}

func (n *PipelineNodeSRTP) Run(ctx context.Context) {
	log := plogger.FromContextSafe(ctx).Prefix("PipelineNodeSRTP")
	n.PipelineNode.Run(ctx)
	for {
		if n.session == nil {
			select {
			case <-ctx.Done():
				n.onStop(ctx)
				return
			default:
			}
			time.Sleep(5 * time.Millisecond)
		} else {
			select {
			case <-ctx.Done():
				n.onStop(ctx)
				return
			case packet := <-n.In:
				err := n.Unprotect(ctx, packet)
				if err != nil {
					log.Errorf("error parsing srtp %s", err.Error())
				}
			}
		}
	}
}

func (n *PipelineNodeSRTP) Unprotect(ctx context.Context, inPacket *packet.UDP) error {
	log := plogger.FromContextSafe(ctx)
	if n.session == nil {
		log.Warnf("received packet but session is nil")
		return nil
	}
	p, err := srtp.Unprotect(n.session.SrtpIn, inPacket)
	if err != nil {
		if err.Error() == "srtp_err_status_replay_fail" {
			// Ignoring this repeated packet
			log.Warnf("srtp error=srtp_err_status_replay_fail => skip packet")
			return nil
		}
		log.Errorf("SRTP %s", err.Error())
		return errors.New("could not decrypt SRTP packet")
	}
	switch outPacket := p.(type) {
	case *srtp.PacketRTP:
		select {
		case n.OutPacketRTP <- outPacket:
		default:
			log.Warnf("OutPacketRTP is full, dropping packet unprotect")
		}
	case *srtp.PacketRTCP:
		select {
		case n.OutPacketRTCP <- outPacket:
		default:
			log.Warnf("OutPacketRTP is full, dropping packet unprotect")
		}
	default:
		return errors.New("rtp or rtcp")
	}
	return nil
}
