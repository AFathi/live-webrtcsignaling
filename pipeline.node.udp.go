package main

/*
 * PipelineNodeUDP
 * convert an UDPConn to a pipeline emitter
 *
 * IN: nothing
 * OUT: packet.UdpPacket
 *
 */

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/packet"
)

type PipelineNodeUDP struct {
	PipelineNode
	// I/O
	Out chan *packet.UDP
	// privates
	udpConn *net.UDPConn
}

func NewPipelineNodeUDP(udpConn *net.UDPConn) *PipelineNodeUDP {
	n := new(PipelineNodeUDP)
	n.udpConn = udpConn
	n.Out = make(chan *packet.UDP, 1000)
	return n
}

func (n *PipelineNodeUDP) ReadPacketUDP(ctx context.Context) (*packet.UDP, error) {
	packet := packet.NewUDP()
	size, rAddr, err := n.udpConn.ReadFromUDP(packet.GetData())
	if err != nil {
		return nil, err
	}
	packet.SetCreatedAt(time.Now())
	packet.SetRAddr(rAddr)
	packet.Slice(0, size)
	return packet, nil
}

// IPipelinePipelineNode interface functions
func (n *PipelineNodeUDP) Run(ctx context.Context) {
	var totalPacketsReceivedSize uint64
	var lastPacketsReceivedSize uint64

	n.Running = true
	n.emitStart()
	//
	log := plogger.FromContextSafe(ctx).Prefix("PipelineNodeUDP")
	// monitoring inputs
	ticker := time.NewTicker(1 * time.Second)
	defer func() {
		ticker.Stop()
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				n.onStop(ctx)
				return
			case <-ticker.C:
				msg := new(PipelineMessageInBps)
				msg.Bps = totalPacketsReceivedSize - lastPacketsReceivedSize
				lastPacketsReceivedSize = totalPacketsReceivedSize
				select {
				case n.Bus <- msg:
				default:
					plogger.Warnf("Bus is full, dropping event PipelineMessageInBps")
				}
			}
		}
	}()
	//
	for n.Running {
		packet, err := n.ReadPacketUDP(ctx)
		if err != nil {
			log.Errorf("packet read error %s", err.Error())
			return
		}
		atomic.AddUint64(&totalPacketsReceivedSize, uint64(packet.GetSize()))
		select {
		case n.Out <- packet:
		default:
			log.Warnf("Out is full, dropping udp packet")
		}
	}
}

func (n *PipelineNodeUDP) onStop(ctx context.Context) {
	log := plogger.FromContextSafe(ctx)
	if n.udpConn != nil {
		log.Infof("CLOSING UDP CONNECTION\n")
		n.udpConn.Close()
		n.udpConn = nil
	}
	n.Running = false
	n.emitStop()
}
