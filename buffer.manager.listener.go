package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/packet"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

const (
	maxRTPPacketBuffer = 3000
)

type ListenerBuffer struct {
	jst                JitterStreamTypeOptions
	pt                 int
	ptRtx              int
	seqNumber          uint64
	rtxSeqNumber       uint16
	packetCounter      int
	in                 chan *srtp.PacketRTP
	buffer             *PacketRTPBuffer
	out                chan *srtp.PacketRTP
	outStarted         bool
	event              chan interface{}
	ctx                context.Context
	log                plogger.PLogger
	freq               uint32
	rAddr              *net.UDPAddr
	videoBitrate       int
	audioBitrate       int
	rtxSsrc            uint32
	ssrc               uint32
	bitrate            Bitrate
	packetReceived     int
	packetLoss         int
	packetLossRate     float64
	rtpTimestampCycles uint32
	rtpSeqCycles       uint32
	n                  *PipelineNodeJitterListener
	codecOption        CodecOptions
	outRTP             chan *srtp.PacketRTP
	outRTCP            chan *RtpUdpPacket
	lastRtpTimestamp   uint64
}

type PacketRTPBuffer struct {
	sync.RWMutex
	d             map[uint64]*srtp.PacketRTP
	lastPacketRTP *srtp.PacketRTP
	maxEntries    int
	first         uint64
	last          uint64
}

func NewPacketRTPBuffer(maxEntries int) *PacketRTPBuffer {
	pb := new(PacketRTPBuffer)
	pb.Init(maxEntries)

	return pb
}

func (pb *PacketRTPBuffer) Init(maxEntries int) {
	pb.d = make(map[uint64]*srtp.PacketRTP)
	pb.maxEntries = maxEntries
}

func (pb *PacketRTPBuffer) Push(p *srtp.PacketRTP) {
	pb.Lock()
	defer pb.Unlock()

	seq := p.GetSeqNumberWithCycles()
	pb.d[seq] = p
	if pb.first == 0 {
		pb.first = seq
	}
	if seq > pb.last {
		pb.last = seq
		pb.lastPacketRTP = p
	}
	if len(pb.d) > pb.maxEntries {
		delete(pb.d, pb.first)
		pb.first++
	}

	return
}

func (pb *PacketRTPBuffer) Get(seq uint64) *srtp.PacketRTP {
	pb.RLock()
	defer pb.RUnlock()

	return pb.d[seq]
}

func (pb *PacketRTPBuffer) GetLastPacketRTP() *srtp.PacketRTP {
	return pb.lastPacketRTP
}

func NewJitterBufferListener(ctx context.Context, codecOption CodecOptions, pt uint16, ptRtx uint16, freq uint32, ssrc uint32, rtxSsrc uint32, jst JitterStreamTypeOptions, bitrate Bitrate, rtt *int64, n *PipelineNodeJitterListener) (listenerBuffer *ListenerBuffer, err error) {
	log := plogger.FromContextSafe(ctx).Prefix(fmt.Sprintf("JitterBuffer PT %d", pt)).Tag("jitterbuffer-listener")

	listenerBuffer = &ListenerBuffer{
		jst:           jst,
		pt:            int(pt),
		ptRtx:         int(ptRtx),
		packetCounter: 0,
		in:            make(chan *srtp.PacketRTP, channelSize),
		buffer:        NewPacketRTPBuffer(maxRTPPacketBuffer),
		out:           make(chan *srtp.PacketRTP, channelSize),
		event:         make(chan interface{}, channelSize),
		ctx:           ctx,
		log:           log,
		freq:          freq,
		ssrc:          ssrc,
		videoBitrate:  config.Bitrates.Video.Start,
		audioBitrate:  config.Bitrates.Audio.Start,
		rtxSeqNumber:  randUint16(),
		rtxSsrc:       rtxSsrc,
		bitrate:       bitrate,
		n:             n,
		codecOption:   codecOption,
	}
	// emitting data packets
	listenerBuffer.outRTP = make(chan *srtp.PacketRTP, 128)

	go listenerBuffer.inPackets()

	return
}

func (lb *ListenerBuffer) cycleDetector(p *srtp.PacketRTP) {
	lastSorted := lb.buffer.GetLastPacketRTP()

	if lastSorted == nil {
		return
	}
	pSeqNumber := p.GetSeqNumberWithCycles()
	lSeqNumber := lastSorted.GetSeqNumberWithCycles()
	if (pSeqNumber >= 0 && pSeqNumber <= 1000 && lSeqNumber >= 64535 && lSeqNumber <= 65535) && pSeqNumber < lSeqNumber {
		lb.log.Debugf("CYCLE DETECTOR p.GetSeqNumberWithCycles() is %d, lastSorted is %#v, lastSorted.GetSeqNumberWithCycles() is %d", p.GetSeqNumberWithCycles(), lastSorted, lastSorted.GetSeqNumberWithCycles())
		lb.rtpSeqCycles++
		p.SetSeqCycle(lb.rtpSeqCycles)
	}
	if pSeqNumber > lSeqNumber+32768 {
		lb.log.Debugf("CYCLE DETECTOR pSeqNumber %d > lSeqNumber %d", pSeqNumber, lSeqNumber)
		p.SetSeqCycle(lb.rtpSeqCycles - 1)
	}

	pTimestamp := p.GetTimestampWithCycles()
	lTimestamp := lastSorted.GetTimestampWithCycles()
	if (pTimestamp >= 0 && pTimestamp <= 500000 && lTimestamp >= 4294467295 && lTimestamp <= 4294967295) && pTimestamp < lTimestamp {
		lb.log.Debugf("CYCLE DETECTOR p.GetTimestampWithCycles() is %d, lastSorted is %#v, lastSorted.GetTimestampWithCycles() is %d", p.GetTimestampWithCycles(), lastSorted, lastSorted.GetTimestampWithCycles())
		lb.rtpTimestampCycles++
		p.SetTsCycle(lb.rtpTimestampCycles)
	}
	if pTimestamp > lTimestamp+2147483648 {
		lb.log.Debugf("CYCLE DETECTOR pTimestamp %d > lTimestamp %d", pTimestamp, lTimestamp)
		p.SetTsCycle(lb.rtpTimestampCycles - 1)
	}

	return
}

func (lb *ListenerBuffer) isH264KeyFrame(p *srtp.PacketRTP) bool {
	if lb.jst != JitterStreamVideo {
		lb.log.Warnf("could not retreive key frame info because the stream is not configured as video")
		return false
	}
	d := p.GetData()
	RTPHeaderBytes := 12 + d[0]&0x0F

	//j.log.Warnf("BYTE IS %X", d[RTPHeaderBytes])
	f := d[RTPHeaderBytes] >> 7 & 0x1
	nri := d[RTPHeaderBytes] >> 5 & 0x03
	nalType := d[RTPHeaderBytes] & 0x1F

	lb.log.Warnf("F IS %d, NRI IS %d, TYPE IS %d", f, nri, nalType)
	if nalType == 24 || nalType == 28 {
		return true
	}

	return false
}

func (lb *ListenerBuffer) isVP8KeyFrame(p *srtp.PacketRTP) bool {
	if lb.jst != JitterStreamVideo {
		lb.log.Warnf("could not retreive key frame info because the stream is not configured as video")
		return false
	}
	d := p.GetData()
	rtpP := d[(d[0]&0x0f)*4+16] & 0x01
	if rtpP == 0 {
		return true
	}

	return false
}

func (lb *ListenerBuffer) inPackets() {
	waitingKeyFrame := true
	for {
		select {
		case <-lb.ctx.Done():
			lb.log.Infof("goroutine inPackets exit")
			return
		case p := <-lb.in:
			if lb.jst == JitterStreamVideo && waitingKeyFrame == true {
				lb.seqNumber = p.GetSeqNumberWithCycles()
				if (lb.codecOption == CodecVP8 && lb.isVP8KeyFrame(p)) || (lb.codecOption == CodecH264 && lb.isH264KeyFrame(p)) {
					lb.log.Infof("Found the keyframe, starting to send stream")
					waitingKeyFrame = false
				} else {
					lb.log.Infof("packet seq %d is not a key frame, searching...", p.GetSeqNumberWithCycles())
					continue
				}
			}
			p.SetTsCycle(lb.rtpTimestampCycles)
			p.SetSeqCycle(lb.rtpSeqCycles)
			lb.cycleDetector(p)
			seq := p.GetSeqNumberWithCycles()
			if lb.seqNumber == 0 {
				lb.seqNumber = seq
			}
			if lb.jst == JitterStreamVideo {
				lb.log.Debugf("STORING SEQ %d", p.GetSeqNumber())
				lb.buffer.Push(p)
				if seq != lb.seqNumber {
					lb.log.Warnf("RTP discontinuity or encoder problem ? waiting seq %d/%d, having seq %d/%d", GetSeqNumberWithoutCycles(lb.seqNumber), lb.seqNumber, GetSeqNumberWithoutCycles(seq), seq)
					lb.seqNumber = seq
					if seq < lb.seqNumber {
						lb.log.Warnf("the seq number of RTP packet is lower than the seq expected, something is going wrong here, discarding packet")
						return
					}
				}
			}
			lb.seqNumber++
			lb.log.Debugf("[ OUT ] Pushing on socket packet sequence number %d", p.GetSeqNumber())
			select {
			case lb.outRTP <- p:
			default:
				lb.log.Warnf("outRTP is full, dropping packet (jitterBuffer.inPacketsSender)")
			}
		}
	}
}

func (lb *ListenerBuffer) SendRTX(seqs []uint16, ssrc uint32) {
	lb.log.Warnf("SENDRTX SSRC RECEIVED IS %d", ssrc)
	for _, s := range seqs {
		currentSeq := GetSeqNumberWithoutCycles(lb.seqNumber)
		seqCycle := lb.rtpSeqCycles
		if s > currentSeq {
			seqCycle--
			if seqCycle < 0 {
				seqCycle = 0
			}
		}
		originalPacketRTP := lb.buffer.Get(GetSeqNumberWithCycles(s, seqCycle))
		if originalPacketRTP == nil {
			lb.log.Infof("could not retransmit original packet RTP seq %d, not found in list", s)
			continue
		}
		originData := originalPacketRTP.GetData()
		originCC := (originData[0] & 0x0f) << 2
		if originCC != 0 {
			lb.log.Warnf("ORIGINCC IS NOT SET TO ZERO !!! WARNING !!! RTX")
		}
		originSize := originalPacketRTP.GetSize()
		data := make([]byte, originSize+2)
		copy(data[0:12+originCC], originData[0:12+originCC])
		copy(data[12+originCC:14+originCC], originData[2:4])
		copy(data[14+originCC:], originData[12+originCC:])

		// Changing Payload Type to RTX Payload Type
		data[1] = originData[1]&0x80 | (byte(lb.ptRtx) & 0x7f)
		// Changing SequenceNumber to RTX Sequence Number
		binary.BigEndian.PutUint16(data[2:4], lb.rtxSeqNumber)
		// Changing SSRC to RTX SSRC
		binary.BigEndian.PutUint32(data[8:12], lb.rtxSsrc)

		lb.log.Infof("send RTX packet with ssrc %d / pt %d / seq %d / original seq %d / lb.seqNumber is %d", lb.rtxSsrc, byte(lb.ptRtx), lb.rtxSeqNumber, s, lb.seqNumber)
		packetRTP := srtp.NewPacketRTP(packet.NewUDP())
		packetRTP.SetData(data)
		select {
		case lb.outRTP <- packetRTP:
		default:
			lb.log.Warnf("outRTP is full, dropping packet packetRTP RTX")
		}
		lb.rtxSeqNumber++
	}
}

func (lb *ListenerBuffer) PushPacket(packet *srtp.PacketRTP) {
	lb.in <- packet
}
