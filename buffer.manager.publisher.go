package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/my"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

// percent delta difference to manage bitrate up/down
const percentDelta = 0.3

// Max latency before lowering bitrate (in ms)
const maxLatency = 800

type NackPliState struct {
	seqCycle int
	tsCycle  int
	nacked   int
	pli      int
}

type ProtectedPacketNacked struct {
	my.RWMutex
	d map[uint16]NackPliState
}

func NewProtectedPacketNacked() *ProtectedPacketNacked {
	m := new(ProtectedPacketNacked)
	m.Init()
	return m
}

func (m *ProtectedPacketNacked) Init() {
	m.d = make(map[uint16]NackPliState)
}

func (m *ProtectedPacketNacked) Set(k uint16, v NackPliState) {
	m.Lock()
	defer m.Unlock()

	m.d[k] = v
	return
}

func (m *ProtectedPacketNacked) Get(k uint16) NackPliState {
	m.RLock()
	defer m.RUnlock()

	return m.d[k]
}

func (m *ProtectedPacketNacked) Del(k uint16) {
	m.Lock()
	defer m.Unlock()

	delete(m.d, k)
}

type SlicePacketRtp []*srtp.PacketRTP

func (s SlicePacketRtp) Len() int {
	return len(s)
}

func (s SlicePacketRtp) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s SlicePacketRtp) Less(i, j int) bool {
	return s[i].GetSeqNumberWithCycles() < s[j].GetSeqNumberWithCycles()
}

type ProtectedSlicePacketRtp struct {
	my.RWMutex
	d                            SlicePacketRtp
	disordered                   map[uint64]*srtp.PacketRTP
	lastRtpPacketOrderedReceived *srtp.PacketRTP
	first                        uint64
	last                         uint64
	timeSize                     int64
	freq                         uint32
	avgTimeInterval              int64
	maxEntries                   int
}

func NewProtectedSlicePacketRtp(freq uint32, maxEntries int) *ProtectedSlicePacketRtp {
	m := new(ProtectedSlicePacketRtp)
	m.Init(freq, maxEntries)
	return m
}

func (m *ProtectedSlicePacketRtp) Init(freq uint32, maxEntries int) {
	m.freq = freq
	m.maxEntries = maxEntries
	m.timeSize = 0
	m.disordered = make(map[uint64]*srtp.PacketRTP)
}

type JitterStreamTypeOptions int

const (
	JitterStreamAudio JitterStreamTypeOptions = iota
	JitterStreamVideo
)

type BaseClock struct {
	timestamp    uint64
	rtpTimestamp uint64
}

const maxDelayBeforePLI = 100 * 1000000

type JitterBuffer struct {
	jst                JitterStreamTypeOptions
	pt                 int
	ptRtx              int
	inSeqNumber        uint64
	lastInSeqNumber    uint64
	seqNumber          uint64
	packetCounter      int
	in                 chan *srtp.PacketRTP
	inPrevRtpPacket			 *srtp.PacketRTP
	buffer             *BufferPublisher
	bufferUnsorted     *BufferPublisherUnsorted
	reorderMaxDelay    uint64
	nacked             *ProtectedPacketNacked
	out                chan *srtp.PacketRTP
	outStarted         bool
	event              chan interface{}
	ctx                context.Context
	log                plogger.PLogger
	freq               uint32
	filled             bool
	rAddr              *net.UDPAddr
	idealPacketCount   int
	videoBitrate       int
	audioBitrate       int
	rtpPackets         [][]byte
	rtxSsrc            uint32
	ssrc               uint32
	bitrate            Bitrate
	baseTime           BaseClock
	rtt                *int64
	packetReceived     int
	packetLoss         int
	packetLossRate     float64
	packetNacked       int
	outBufferedPackets []*srtp.PacketRTP
	pictureId          uint16
	lastRtpTimestamp   uint64
	lastRtpOut         *srtp.PacketRTP
	rtpTimestampCycles uint32
	rtpSeqCycles       uint32
	n                  *PipelineNodeJitterPublisher
	fps                uint8
	lastPictureId      uint16
	codecOption        CodecOptions
	discontinuity      uint16
	baseInTime         BaseClock
	cumulDelay         int64
	lastCumulDelay  int64
	cumulDelayTime   time.Time
	bufferTimeSize     uint64
	// listener
	outRTP              chan *srtp.PacketRTP
	outRTCP             chan *RtpUdpPacket
	waitingKeyFrame     bool
	lastReorderedPacket *srtp.PacketRTP
	avgRtt              float64
	exitNewKeyFrame     chan struct{}
	acceptDisorder      bool
	firstSeqFound       bool
	lastRtpOutTs        time.Time
	lastInRtp						*srtp.PacketRTP
}

// ch is a channel where jitterbuffer could push buffer packets
func NewJitterBufferPublisher(ctx context.Context, codecOption CodecOptions, pt uint16, ptRtx uint16, freq uint32, ssrc uint32, rtxSsrc uint32, jst JitterStreamTypeOptions, bitrate Bitrate, rtt *int64, n *PipelineNodeJitterPublisher) (jitterBuffer *JitterBuffer, err error) {
	var bufferTimeSize int
	if config.Mode == ModeMCU {
		bufferTimeSize = 100 * 1000000
	} else {
		bufferTimeSize = 0
	}
	log := plogger.FromContextSafe(ctx).Prefix(fmt.Sprintf("JitterBuffer PT %d", pt)).Tag("jitterbuffer-publisher")
	acceptDisorder := false
	if jst == JitterStreamAudio {
		acceptDisorder = true
	}
	bufferUnsorted := NewBufferPublisherUnsorted()
	jitterBuffer = &JitterBuffer{
		jst:             jst,
		pt:              int(pt),
		ptRtx:           int(ptRtx),
		seqNumber:       0,
		packetCounter:   0,
		in:              make(chan *srtp.PacketRTP, channelSize),
		buffer:          NewBufferPublisher(bufferUnsorted, freq, 0),
		bufferUnsorted:  bufferUnsorted,
		reorderMaxDelay: 100 * 1000000,
		nacked:          NewProtectedPacketNacked(),
		out:             make(chan *srtp.PacketRTP, channelSize),
		event:           make(chan interface{}, channelSize),
		ctx:             ctx,
		log:             log,
		freq:            freq,
		filled:          false,
		ssrc:            ssrc,
		videoBitrate:    config.Bitrates.Video.Start,
		audioBitrate:    config.Bitrates.Audio.Start,
		bitrate:         bitrate,
		rtt:             rtt,
		n:               n,
		codecOption:     codecOption,
		bufferTimeSize:  uint64(bufferTimeSize),
		acceptDisorder:  acceptDisorder,
	}
	// emitting REMB/NACKS
	jitterBuffer.outRTCP = make(chan *RtpUdpPacket, 128)

	go jitterBuffer.inPackets()

	return
}

type PipelineMessageSetJitterSize struct {
	size uint64
}

func (j *JitterBuffer) SetJitterSize(size uint64) {
	j.log.Infof("set new jitter bufferTimeSize to %d", size)
	j.bufferTimeSize = size
}

func (j *JitterBuffer) GetArrivalTime(rtpTimestamp uint64) int64 {
	rtpTimestampDiff := (float64(rtpTimestamp) / float64(j.freq) * 1000000000) - (float64(j.baseInTime.rtpTimestamp) / float64(j.freq) * 1000000000)
	return int64(float64(j.baseInTime.timestamp) + rtpTimestampDiff)
}

func (j *JitterBuffer) managePli(seq uint64) {
	j.firstSeqFound = false
	j.waitingKeyFrame = true
	j.inSeqNumber = 0
	j.buffer.Purge()
	j.log.Infof("No nack answer, Send PLI")
	if j.exitNewKeyFrame != nil {
		j.log.Warnf("an existing requestNewKeyFrame() go func is running, skipping")
		return
	}
	j.exitNewKeyFrame = make(chan struct{})
	if j.jst == JitterStreamVideo && j.codecOption == CodecVP8 {
		j.log.Warnf("VP8 packet(s) lost ? waiting seq %d, having %d, request a keyframe and continue", GetSeqNumberWithoutCycles(j.seqNumber), seq)
		j.log.Infof("Sending PLI")
		j.outBufferedPackets = []*srtp.PacketRTP{}
		j.pictureId = 0
		j.lastPictureId = 0
		go j.requestNewKeyFrame()
		j.packetLoss++
		j.log.Warnf("Should wait for a key frame to restart sending")
	} else {
		j.log.Infof("H264 packet(s) lost ? waiting seq %d, having %d, request a keyframe and continue", GetSeqNumberWithoutCycles(j.seqNumber), seq)
		j.log.Infof("Sending PLI")
		j.outBufferedPackets = []*srtp.PacketRTP{}
		go j.requestNewKeyFrame()
		j.packetLoss++
		j.log.Infof("Should wait for a key frame to restart sending")
	}
}

func (j *JitterBuffer) nackPacket(seq uint64) {
	j.log.Infof("Starting Nack sequence %d", seq)
	nackCount := 0
	for j.inSeqNumber <= seq && nackCount < 5 && j.waitingKeyFrame == false {
		j.log.Infof("NACKING packet sequence number %d/%d", GetSeqNumberWithoutCycles(seq), seq)
		j.SendNACK(GetSeqNumberWithoutCycles(seq))
		sleepTime := (j.bufferTimeSize * 2) / 5
		time.Sleep(time.Duration(sleepTime) * time.Nanosecond)
		nackCount++
		j.ChangeBitrate(-0.05)
	}
	if j.waitingKeyFrame == false && j.inSeqNumber <= seq {
		j.managePli(seq)
	}
	//j.nacked.Del(GetSeqNumberWithoutCycles(seq))

	return
}

func (j *JitterBuffer) manageTimeouts() {
	j.log.Warnf("start manageTimeouts() go routine")
	ticker := time.NewTicker(1 * time.Second)
	tickerRtt := time.NewTicker(1 * time.Second)
	rttCount := uint64(0)
	for {
		select {
		case <-j.ctx.Done():
			j.log.Infof("goroutine manageTimeouts exit")
			return
		case <-ticker.C:
			j.log.Infof("j.cumulDelay is %d", j.cumulDelay)
			cumulDelay := j.cumulDelay
			j.lastCumulDelay = cumulDelay
			if j.lastCumulDelay <= 0 && cumulDelay <= 0 && j.waitingKeyFrame == false && j.cumulDelayTime.Add(time.Duration(j.buffer.GetAvgIntervalTime()*2) * time.Nanosecond).After(time.Now()) {
				j.log.Infof("cumulDelay is under 0 (%d) and j.lastCumulDelay is under 0 (%d) up bitrate by 5 percent", cumulDelay, j.lastCumulDelay)
				j.ChangeBitrate(0.05)
			} else if j.cumulDelay > 5000000 {
				j.log.Infof("cumulDelay is over 10000000 (%d) down bitrate by 10 percent", j.cumulDelay)
				j.ChangeBitrate(-0.1)
			}
			//j.cumulDelay = 0
		case <-tickerRtt.C:
			j.log.Infof("RTT is %d", *j.rtt)
			if rttCount == 0 {
				j.avgRtt = float64(*j.rtt)
			} else {
				j.avgRtt = (j.avgRtt*float64(rttCount) + float64(*j.rtt)) / float64(rttCount+1)
			}
			rttCount++
			if config.Mode == ModeMCU {
				newBufferTimeSize := uint64(j.avgRtt) + uint64(j.reorderMaxDelay)
				delta := uint64(math.Abs(float64(int64(newBufferTimeSize) - int64(j.bufferTimeSize))))
				if delta > (j.bufferTimeSize*5)/100 {
					j.log.Infof("Setting jitter buffer size to %d", newBufferTimeSize)
					pipelineMessage := &PipelineMessageSetJitterSize{
						size: newBufferTimeSize,
					}
					j.n.Bus <- pipelineMessage
					j.bufferTimeSize = newBufferTimeSize
				}
			}
		default:
			if j.waitingKeyFrame == false {
				now := uint64(time.Now().UnixNano())
				lastRtpPacket := j.buffer.GetLastPacketRTP()
				j.log.Debugf("lastRtpPacket is %#v and sequence number", lastRtpPacket)
				if lastRtpPacket != nil {
					nextSeq := lastRtpPacket.GetSeqNumberWithCycles() + 1
					nextTs := uint64(lastRtpPacket.GetCreatedAt().UnixNano()) + j.buffer.GetAvgIntervalTime()
					for i := nextTs; i < now && j.waitingKeyFrame == false && nextSeq <= j.lastInSeqNumber; i += j.buffer.GetAvgIntervalTime() {
						packetReceived := j.bufferUnsorted.IsExist(nextSeq)
						if packetReceived == false {
							np := j.nacked.Get(GetSeqNumberWithoutCycles(nextSeq))
							if now > (nextTs + j.reorderMaxDelay + 10000000) {
								j.log.Infof("NACK sequence number %d, time is %d > @%d", nextSeq, now, nextTs+j.reorderMaxDelay+10000000)
								j.log.Infof("NACKING packet sequence number %d/%d", GetSeqNumberWithoutCycles(nextSeq), nextSeq)
								if np.nacked < 10 {
									j.SendNACK(GetSeqNumberWithoutCycles(nextSeq))
									np.nacked++
									j.nacked.Set(GetSeqNumberWithoutCycles(nextSeq), np)
								} else {
									if j.inSeqNumber <= nextSeq {
										j.managePli(nextSeq)
									}
								}
							}
						}
						nextSeq++
					}
				}
			}
			var timeToSleep uint64
			if j.buffer.GetAvgIntervalTime() != 0 {
				timeToSleep = j.buffer.GetAvgIntervalTime()
			} else {
				timeToSleep = 20 * 1000000
			}
			j.log.Debugf("managetimeout, sleeping during %d ns", timeToSleep)
			time.Sleep(time.Duration(timeToSleep) * time.Nanosecond)
		}
	}
}

func (j *JitterBuffer) checkReceivedDelay(p *srtp.PacketRTP) {
	//var isMarked bool

	//isMarked = p.GetMarkerBit()
	//if j.jst == JitterStreamVideo && isMarked == true {
		if j.inPrevRtpPacket == nil {
			j.inPrevRtpPacket = p
		}
		receivedDelay := int64(0)
		rtpTsDiff := int64(0)
		//if p.GetSeqNumberWithCycles() >= j.inPrevRtpPacket.GetSeqNumberWithCycles() {
		/*if p.GetTimestampWithCycles() >= j.inPrevRtpPacket.GetTimestampWithCycles() {
			rtpTsDiff = int64((float64(p.GetTimestampWithCycles())-float64(j.inPrevRtpPacket.GetTimestampWithCycles()))/float64(j.freq)*float64(1000000000))
			receivedDelay = (p.GetCreatedAt().UnixNano() - j.inPrevRtpPacket.GetCreatedAt().UnixNano()) - rtpTsDiff
			j.log.Warnf("p.GetSeqNumberWithCycles() = %d, j.inPrevRtpPacket.GetSeqNumberWithCycles() = %d, rtpTsDiff = ((%f - %f) / %f) * 1000000000 = %f, int64() = %d, rtpTsDiff = %d, receivedDelay = (%d - %d) - %d = %d", p.GetSeqNumberWithCycles(), j.inPrevRtpPacket.GetSeqNumberWithCycles(), float64(p.GetTimestampWithCycles()), float64(j.inPrevRtpPacket.GetTimestampWithCycles()), float64(j.freq), (float64(p.GetTimestampWithCycles())-float64(j.inPrevRtpPacket.GetTimestampWithCycles()))/float64(j.freq)*float64(1000000000), int64((float64(p.GetTimestampWithCycles())-float64(j.inPrevRtpPacket.GetTimestampWithCycles()))/float64(j.freq)*float64(1000000000)), rtpTsDiff, p.GetCreatedAt().UnixNano(), j.inPrevRtpPacket.GetCreatedAt().UnixNano(), rtpTsDiff, receivedDelay)
		} else {
			rtpTsDiff = int64((float64(j.inPrevRtpPacket.GetTimestampWithCycles())-float64(p.GetTimestampWithCycles()))/float64(j.freq)*float64(1000000000))
			receivedDelay = (j.inPrevRtpPacket.GetCreatedAt().UnixNano() - p.GetCreatedAt().UnixNano()) - rtpTsDiff
			j.log.Warnf("p.GetSeqNumberWithCycles() = %d, j.inPrevRtpPacket.GetSeqNumberWithCycles() = %d, rtpTsDiff = ((%f - %f) / %f) * 1000000000 = %f, int64() = %d, rtpTsDiff = %d, receivedDelay = (%d - %d) - %d = %d", p.GetSeqNumberWithCycles(), j.inPrevRtpPacket.GetSeqNumberWithCycles(), float64(j.inPrevRtpPacket.GetTimestampWithCycles()), float64(p.GetTimestampWithCycles()), float64(j.freq), (float64(j.inPrevRtpPacket.GetTimestampWithCycles())-float64(p.GetTimestampWithCycles()))/float64(j.freq)*float64(1000000000), int64((float64(j.inPrevRtpPacket.GetTimestampWithCycles())-float64(p.GetTimestampWithCycles()))/float64(j.freq)*float64(1000000000)), rtpTsDiff, j.inPrevRtpPacket.GetCreatedAt().UnixNano(), p.GetCreatedAt().UnixNano(), rtpTsDiff, receivedDelay)
		}
		*/
		Ri := int64((float64(j.inPrevRtpPacket.GetCreatedAt().UnixNano()) / 1000000000) * float64(j.freq))
		Rj := int64((float64(p.GetCreatedAt().UnixNano()) / 1000000000) * float64(j.freq))
		Si := int64(j.inPrevRtpPacket.GetTimestampWithCycles())
		Sj := int64(p.GetTimestampWithCycles())
		receivedDelay = (Rj - Sj) - (Ri - Si)
		j.cumulDelay += receivedDelay
		j.log.Infof("receivedDelay is %d", receivedDelay)
		j.cumulDelayTime = time.Now()

		j.log.Debugf("prevRtpPacket.GetCreatedAt().UnixNano() (%d) p.GetCreatedAt().UnixNano() (%d) diff rtp timestamp (%d) = %d", j.inPrevRtpPacket.GetCreatedAt().UnixNano(), p.GetCreatedAt().UnixNano(), rtpTsDiff, receivedDelay)
		j.inPrevRtpPacket = p

		j.lastRtpTimestamp = p.GetTimestampWithCycles()
	//}
}

const MaxUint32 = ^uint32(0)
const MaxUint16 = ^uint16(0)

func (j *JitterBuffer) cycleDetector(p *srtp.PacketRTP) {
	lastSorted := j.buffer.GetLastPacketRTP()

	if lastSorted == nil {
		return
	}
	pSeqNumber := p.GetSeqNumber()
	lSeqNumber := lastSorted.GetSeqNumber()
	if (pSeqNumber >= 0 && pSeqNumber <= 1000 && lSeqNumber >= 64535 && lSeqNumber <= 65535) && pSeqNumber < lSeqNumber {
		j.log.Debugf("CYCLE DETECTOR p.GetSeqNumberWithCycles() is %d, lastSorted is %#v, lastSorted.GetSeqNumberWithCycles() is %d", p.GetSeqNumberWithCycles(), lastSorted, lastSorted.GetSeqNumberWithCycles())
		j.rtpSeqCycles++
		p.SetSeqCycle(j.rtpSeqCycles)
	}
	if (pSeqNumber >= 64535 && pSeqNumber <= 65535) && (lSeqNumber >= 0 && lSeqNumber <= 1000) && pSeqNumber > lSeqNumber {
		j.log.Debugf("CYCLE DETECTOR pSeqNumber %d > lSeqNumber %d", pSeqNumber, lSeqNumber)
		p.SetSeqCycle(j.rtpSeqCycles - 1)
	}

	pTimestamp := p.GetTimestamp()
	lTimestamp := lastSorted.GetTimestamp()
	if (pTimestamp >= 0 && pTimestamp <= 500000 && lTimestamp >= 4294467295 && lTimestamp <= 4294967295) && pTimestamp < lTimestamp {
		j.log.Debugf("CYCLE DETECTOR p.GetTimestampWithCycles() is %d, lastSorted is %#v, lastSorted.GetTimestampWithCycles() is %d", p.GetTimestampWithCycles(), lastSorted, lastSorted.GetTimestampWithCycles())
		j.rtpTimestampCycles++
		p.SetTsCycle(j.rtpTimestampCycles)
	}
	if (pTimestamp >= 4294467295 && pTimestamp <= 4294967295) && (lTimestamp >= 0 && lTimestamp <= 1000) && pTimestamp > lTimestamp {
		j.log.Debugf("CYCLE DETECTOR pTimestamp %d > lTimestamp %d", pTimestamp, lTimestamp)
		p.SetTsCycle(j.rtpTimestampCycles - 1)
	}

	return
}

func (j *JitterBuffer) setSeqAndTsWithCycles(p *srtp.PacketRTP) {
	p.SetSeqCycle(j.rtpSeqCycles)
	p.SetTsCycle(j.rtpTimestampCycles)
}

func (j *JitterBuffer) inPackets() {
	var startFindingSeqTs *time.Time

	startWait := j.bufferTimeSize
	for {
		select {
		case <-j.ctx.Done():
			j.log.Infof("goroutine inPackets exit")
			return
		case p := <-j.in:
			switch p.GetPT() {
			case j.pt:
				j.setSeqAndTsWithCycles(p)
				j.cycleDetector(p)
				//j.correctingTimestamp(p)

				j.log.Debugf("RECEIVED PACKET RTP SEQ %d", p.GetSeqNumberWithCycles())
				if j.lastInRtp == nil || j.lastInRtp.GetSeqNumberWithCycles() < p.GetSeqNumberWithCycles() {
					j.lastInRtp = p
				}
				// If we're waiting a new key frame, don't do anything before receiving this Key frame
				if j.waitingKeyFrame == true {
					j.inSeqNumber = p.GetSeqNumberWithCycles()
					if (j.codecOption == CodecVP8 && j.isVP8KeyFrame(p)) || (j.codecOption == CodecH264 && j.isH264KeyFrame(p)) {
						j.log.Infof("Key Frame received @ seq %d/%d restart...", p.GetSeqNumber(), j.inSeqNumber)
						if j.exitNewKeyFrame != nil {
							j.exitNewKeyFrame <- struct{}{}
						}
						j.waitingKeyFrame = false
					} else {
						j.log.Warnf("packet seq %d is not a key frame... continue", p.GetSeqNumberWithCycles())
						continue
					}
				}

				// Check if we found the first RTP Sequence number because packets
				// could be unsorted from the beginning
				if j.firstSeqFound == false {
					if startFindingSeqTs == nil {
						startFindingSeqTs = new(time.Time)
						*startFindingSeqTs = time.Now()
					}
					if time.Now().After(startFindingSeqTs.Add(time.Duration(startWait) * time.Nanosecond)) {
						j.log.Infof("flushing all disordered packets in publisher buffer")
						if j.jst == JitterStreamVideo {
							j.buffer.PushAllUnsorted(false)
						} else {
							j.buffer.PushAllUnsorted(true)
						}
						j.firstSeqFound = true
						startFindingSeqTs = nil
						if j.buffer.GetLastPacketRTP() != nil {
							j.inSeqNumber = j.buffer.GetLastPacketRTP().GetSeqNumberWithCycles() + 1
						}
						j.log.Infof("j.inSeqNumber start sequence number is %d", j.inSeqNumber)
					} else {
						j.log.Infof("PUSH packet in unsorted buffer SEQ %d/%d", p.GetSeqNumber(), p.GetSeqNumberWithCycles())
						if p.GetSeqNumberWithCycles() >= j.inSeqNumber {
							j.bufferUnsorted.Put(p)
						} else {
							j.log.Warnf("dropping seq %d/%d, j.inSeqNumber is %d", p.GetSeqNumber(), p.GetSeqNumberWithCycles(), j.inSeqNumber)
						}
						continue
					}
				}
				if j.lastRtpTimestamp == 0 {
					j.baseInTime.rtpTimestamp = p.GetTimestampWithCycles()
					j.baseInTime.timestamp = uint64(time.Now().UnixNano())
				}

				if j.inSeqNumber == 0 {
					j.inSeqNumber = p.GetSeqNumberWithCycles()
				}

				if p.GetSeqNumberWithCycles() > j.lastInSeqNumber {
					j.lastInSeqNumber = p.GetSeqNumberWithCycles()
				}

				if p.GetSeqNumberWithCycles() == j.inSeqNumber {
					j.log.Debugf("PUSH IN SORTED BUFFER SEQ %d", p.GetSeqNumberWithCycles())
					err := j.buffer.Push(j.log, p, false, j.acceptDisorder)
					if err != nil {
						j.log.Warnf("Could not push packet in sorted buffer: %s", err.Error())
					}
				} else {
					if p.GetSeqNumberWithCycles() <= j.inSeqNumber {
						j.log.Warnf("RTP PACKET SEQ %d, j.inSeqNumber is %d ALREADY TREATED IN OUT QUEUE, DUPLICATE ?", p.GetSeqNumberWithCycles(), j.inSeqNumber)
						continue
					} else {
						j.log.Debugf("PUSH IN UNSORTED BUFFER SEQ %d / j.inSeqNumber is %d", p.GetSeqNumberWithCycles(), j.inSeqNumber)
						j.bufferUnsorted.Put(p)
						switch j.jst {
						case JitterStreamAudio:
							j.log.Debugf("p with seq %d is #%v, j.buffer.Get(%d) == %#v", p.GetSeqNumberWithCycles(), p, j.inSeqNumber, j.buffer.Get(j.inSeqNumber))
							// Should test if j.buffer.GetLast() != nil here
							lastPacketRTP := j.buffer.GetLastPacketRTP()
							if lastPacketRTP != nil {
								if p.GetCreatedAt().After(lastPacketRTP.GetCreatedAt().Add(time.Duration(50) * time.Millisecond)) {
									j.log.Warnf("no rtp packets received in sorted buffer since 50ms, skipping sequence %d", j.inSeqNumber)
									if j.jst == JitterStreamVideo {
										j.buffer.PushAllUnsorted(false)
									} else {
										j.buffer.PushAllUnsorted(true)
									}
									j.inSeqNumber = j.buffer.GetLastSequenceNumber() + 1
								}
							} else {
								if j.jst == JitterStreamVideo {
									j.buffer.PushAllUnsorted(false)
								} else {
									j.buffer.PushAllUnsorted(true)
								}
								j.inSeqNumber = j.buffer.GetLastSequenceNumber() + 1
							}
						}
					}
				}

				if j.outStarted == false {
					go j.outPackets()
					if j.jst == JitterStreamVideo {
						go j.manageTimeouts()
					}
					j.outStarted = true
				}
				j.checkReceivedDelay(p)
				if j.buffer.GetLastPacketRTP() != nil {
					j.inSeqNumber = j.buffer.GetLastPacketRTP().GetSeqNumberWithCycles()+1
				}
			case j.ptRtx:
				j.log.Infof("[ RTX ] RTP Packet PT %d, considered as a retransmission packet", p.GetPT())
				packetOriginRTP, originSeq := p.RTXExtractOriginal(j.ssrc)
				if j.waitingKeyFrame == false && p.GetSeqNumberWithCycles() >= j.inSeqNumber {
					// Set the correst SEQ and TS cycles
					j.setSeqAndTsWithCycles(packetOriginRTP)
					j.cycleDetector(packetOriginRTP)
					j.log.Infof("REINSERT packet data SEQ %d/%d", packetOriginRTP.GetSeqNumber(), packetOriginRTP.GetSeqNumberWithCycles())
					if packetOriginRTP.GetSeqNumberWithCycles() > j.inSeqNumber {
						j.bufferUnsorted.Put(packetOriginRTP)
					} else {
						err := j.buffer.Push(j.log, packetOriginRTP, false, j.acceptDisorder)
						if err != nil {
							j.log.Warnf("RTX packet RTP seq %d could not be reinserted on the list: %s", originSeq, err.Error())
						} else {
							j.log.Infof("RTX packet RTP seq %d reinserted on the list: %#v", originSeq, packetOriginRTP.GetData())
							j.inSeqNumber = j.buffer.GetLastPacketRTP().GetSeqNumberWithCycles() + 1
						}
					}
				}
			default:
				j.log.Warnf("unknown Payload Type (%d != %d or %d) received on this ssrc %d", p.GetPT(), j.pt, j.ptRtx, j.ssrc)
			}
		}
	}
}

func (j *JitterBuffer) isH264KeyFrame(p *srtp.PacketRTP) bool {
	if j.jst != JitterStreamVideo {
		j.log.Warnf("could not retreive key frame info because the stream is not configured as video")
		return false
	}
	d := p.GetData()
	RTPHeaderBytes := 12 + d[0]&0x0F

	//j.log.Warnf("BYTE IS %X", d[RTPHeaderBytes])
	//f := d[RTPHeaderBytes] >> 7 & 0x1
	//nri := d[RTPHeaderBytes] >> 5 & 0x03
	nalType := d[RTPHeaderBytes] & 0x1F

	//j.log.Warnf("F IS %d, NRI IS %d, TYPE IS %d", f, nri, nalType)
	if nalType == 24 {
		return true
	}

	return false
}

func (j *JitterBuffer) isVP8KeyFrame(p *srtp.PacketRTP) bool {
	if j.jst != JitterStreamVideo {
		j.log.Warnf("could not retreive key frame info because the stream is not configured as video")
		return false
	}
	d := p.GetData()
	rtpP := d[(d[0]&0x0f)*4+16] & 0x01
	if rtpP == 0 {
		return true
	}

	return false
}

func (j *JitterBuffer) getPictureId(p *srtp.PacketRTP) (pictureId uint16, err error) {
	if j.jst != JitterStreamVideo {
		err = errors.New("the stream is not configured as video")
		return
	}
	d := p.GetData()
	m := d[12+int(d[0]&0x0f)+2] & 0xf0
	if m == 0 {
		pictureId = uint16(d[12+int(d[0]&0x0f)+2] & 0x7f)
	} else {
		pictureId = binary.BigEndian.Uint16(d[12+int(d[0]&0x0f)+2:14+int(d[0]&0x0f)+2]) & 0x7fff
	}

	return
}

func (j *JitterBuffer) getMarker(p *srtp.PacketRTP) (marker bool, err error) {
	if j.jst != JitterStreamVideo {
		err = errors.New("could not get Marker Bit info because the stream is not configured as Video")
		return
	}
	markerBit := p.GetData()[1] & 0x80
	if markerBit != 0 {
		marker = true
		return
	}

	return
}

// If true -> pictureCompleted
//    false -> not completed, shoud continue
func (j *JitterBuffer) checkRtpVP8Pictures(p *srtp.PacketRTP) bool {
	markerPicture, _ := j.getMarker(p)
	pictureId, err := j.getPictureId(p)
	if err != nil {
		j.log.Warnf("could not get picture ID: %s", err.Error())
	} else {
		j.log.Debugf("SEQ %d, MARKER %t, PictureId %d", p.GetSeqNumber(), markerPicture, pictureId)
		if markerPicture == false {
			if j.pictureId == 0 {
				j.pictureId = pictureId
			}
			if j.pictureId == pictureId {
				j.log.Debugf("RTP packet is not marked, waiting frame to complete")
				j.outBufferedPackets = append(j.outBufferedPackets, p)
			}
			return false
		} else {
			if j.pictureId != 0 {
				if j.pictureId == pictureId {
					j.log.Debugf("frame is completed, send all RTP packets")
					for _, pRtp := range j.outBufferedPackets {
						j.sendVideoPacket(pRtp)
					}
					j.outBufferedPackets = []*srtp.PacketRTP{}
					j.pictureId = 0
					if pictureId != j.lastPictureId {
						j.lastPictureId = pictureId
						j.fps++
					}
					return true
				} else {
					j.log.Warnf("frame is not completed, 1 or more RTP packets has been lost, ignored")
					j.outBufferedPackets = []*srtp.PacketRTP{}
					j.pictureId = 0
					j.seqNumber = p.GetSeqNumberWithCycles() + 1
					return false
				}
			} else {
				if pictureId != j.lastPictureId {
					j.lastPictureId = pictureId
					j.fps++
				}
			}
		}
	}

	return true
}

func (j *JitterBuffer) waitTimeForSendingRTPPacket(p *srtp.PacketRTP) {
	if j.lastRtpOut == nil {
		return
	}
	//tNow := time.Now()
	/*refillBufferTime := float64(0)
	if j.buffer.GetTimeSize() <= uint64(j.buffer.GetAvgIntervalTime()) {
		refillBufferTime = float64(j.bufferTimeSize - j.buffer.GetTimeSize())
	}*/
	//t := j.lastRtpOutTs.Add(time.Duration((float64(p.GetTimestampWithCycles()-j.lastRtpOut.GetTimestampWithCycles())/float64(j.freq))*1000000000+refillBufferTime) * time.Nanosecond)
	/*t := j.lastRtpOutTs.Add(time.Duration((float64(p.GetTimestampWithCycles()-j.lastRtpOut.GetTimestampWithCycles())/float64(j.freq))*1000000000) * time.Nanosecond)
	j.log.Debugf("j.buffer.GetTimeSize() is %d", j.buffer.GetTimeSize())
	if t.After(tNow) && j.buffer.GetTimeSize() <= j.bufferTimeSize {
		timeToSleep := t.UnixNano() - tNow.UnixNano()
		timeSlept := timeToSleep
		j.log.Debugf("Waiting for sending packet of %d nanoseconds", timeToSleep)
		for timeSlept > 0 {
			time.Sleep(5 * time.Millisecond)
			if j.buffer.GetTimeSize() >= j.bufferTimeSize {
				//j.log.Infof("sorted buffer is now refilled @ 100%%, restart sending RTP packets")
				return
			}
			timeSlept -= 5000000
			if timeSlept < 0 {
				timeSlept = 0
			}
		}
	}*/
	diffTs := uint64((float64(j.lastInRtp.GetTimestampWithCycles() - p.GetTimestampWithCycles()) / float64(j.freq)) * 1000000000)
	if diffTs < j.bufferTimeSize {
		for diffTs < j.bufferTimeSize {
			time.Sleep(5 * time.Millisecond)
			diffTs = uint64((float64(j.lastInRtp.GetTimestampWithCycles() - p.GetTimestampWithCycles()) / float64(j.freq)) * 1000000000)
		}
	}

	return
}

func (j *JitterBuffer) checkAndInitializeTimestamps(packetRTP *srtp.PacketRTP) {
	if j.baseTime.timestamp == 0 {
		j.baseTime.timestamp = uint64(time.Now().UnixNano())
		j.seqNumber = packetRTP.GetSeqNumberWithCycles()
		j.baseTime.rtpTimestamp = uint64((float64(packetRTP.GetTimestamp()) / float64(j.freq)) * 1000000000)
	}
}

func (j *JitterBuffer) correctingTimestamp(p *srtp.PacketRTP) {
	now := time.Now().UnixNano()
	originalTs := p.GetCreatedAt().UnixNano()
	tsDelta := now - originalTs

	p.SetTimestamp(p.GetTimestamp() + uint32(tsDelta))
}

func (j *JitterBuffer) sendVideoPacket(packetRTP *srtp.PacketRTP) {
	j.waitTimeForSendingRTPPacket(packetRTP)
	//j.correctingTimestamp(packetRTP)

	if j.seqNumber != packetRTP.GetSeqNumberWithCycles() {
		j.log.Debugf("Stream sequence is broken, j.seqNumber is %d and packetRTP is %d", j.seqNumber, packetRTP.GetSeqNumberWithCycles())
	} else {
		j.log.Debugf("Pushing out VIDEO packet sequence number %d/%d j.seqNumber is %d", packetRTP.GetSeqNumber(), packetRTP.GetSeqNumberWithCycles(), j.seqNumber)
	}
	j.lastRtpOut = packetRTP
	j.lastRtpOutTs = time.Now()
	j.out <- packetRTP
	j.seqNumber = packetRTP.GetSeqNumberWithCycles() + 1
	j.packetReceived++
	j.packetLossRate = (float64(100) / float64(j.packetReceived)) * float64(j.packetLoss)
}

func (j *JitterBuffer) manageVideoPacket(packetRTP *srtp.PacketRTP) {
	var completed bool

	switch j.codecOption {
	case CodecVP8:
		completed = j.checkRtpVP8Pictures(packetRTP)
	default:
		completed = true
	}

	if completed == true {
		j.sendVideoPacket(packetRTP)
	}

	return
}

func (j *JitterBuffer) manageAudioPacket(packetRTP *srtp.PacketRTP) {
	j.checkAndInitializeTimestamps(packetRTP)
	j.waitTimeForSendingRTPPacket(packetRTP)
	//j.correctingTimestamp(packetRTP)

	j.lastRtpOut = packetRTP
	j.lastRtpOutTs = time.Now()
	j.log.Debugf("Pushing out AUDIO packet sequence number %d", GetSeqNumberWithoutCycles(j.seqNumber))
	j.out <- packetRTP
	j.seqNumber++
}

var test int

func (j *JitterBuffer) requestNewKeyFrame() {
	test++
	j.ChangeBitrate(-0.3)
	j.log.Warnf("starting requestNewKeyFrame -- %d", test)
	ticker := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-j.ctx.Done():
			j.log.Infof("goroutine requestNewKeyFrame() exit")
			return
		case <-ticker.C:
			j.log.Infof("key frame not received, sending PLI again and wait 1000 ms...")
			j.ChangeBitrate(-0.3)
			j.SendPLI()
		case <-j.exitNewKeyFrame:
			j.exitNewKeyFrame = nil
			test--
			return
		}
		j.log.Warnf("LOOP PLI")
	}
	test--
}

func (j *JitterBuffer) outPackets() {
	var fpsTicker *time.Ticker
	j.waitingKeyFrame = false

	j.log.Warnf("starting outPackets")
	fpsTicker = time.NewTicker(10 * time.Second)
	for {
		select {
		case <-j.ctx.Done():
			j.log.Infof("goroutine outPackets exit")
			return
		case <-fpsTicker.C:
			if j.fps != 0 {
				j.log.Infof("FPS is %d", j.fps)
				j.fps = 0
			}
		default:
			if j.buffer.GetAvgIntervalTime() == 0 || *j.rtt == 0 {
				j.log.Infof("Waiting for Average Interval Time and RTT infos...")
				time.Sleep(5 * time.Millisecond)
				continue
			}
			packetRTP := j.buffer.Pop()
			if packetRTP != nil {
				switch j.jst {
				case JitterStreamVideo:
					j.manageVideoPacket(packetRTP)
				case JitterStreamAudio:
					j.manageAudioPacket(packetRTP)
				}
			} else {
				timeToSleep := j.buffer.GetAvgIntervalTime()
				if timeToSleep == 0 {
					timeToSleep = 20000000
				}
				time.Sleep(time.Duration(timeToSleep) * time.Nanosecond)
			}
		}
	}
}

// if this is the first packet received (seqNumber == 0), we should wait for another packet before considering
// reforwarding to the pushCh channel
func (j *JitterBuffer) PushPacket(packet *srtp.PacketRTP) {
	j.in <- packet
}

func (j *JitterBuffer) PullPacket() *srtp.PacketRTP {
	return <-j.out
}

func (j *JitterBuffer) SetRaddr(rAddr *net.UDPAddr) {
	j.rAddr = rAddr
}

func (j *JitterBuffer) GetInCh() chan *srtp.PacketRTP {
	return j.in
}

func (j *JitterBuffer) GetOutCh() chan *srtp.PacketRTP {
	return j.out
}

func (j *JitterBuffer) GetSSRC() uint32 {
	return j.ssrc
}

func (j *JitterBuffer) IsSSRC(ssrc uint32) bool {
	if ssrc == j.ssrc {
		return true
	}

	return false
}

func (j *JitterBuffer) SendNACK(seq uint16) {
	if j.rAddr == nil {
		j.log.Warnf("could not send any packets j.rAddr is nil")
		return
	}
	pNack := rtcp.NewPacketRTPFBNack()
	pNack.PacketRTPFB.SenderSSRC = 0
	pNack.PacketRTPFB.MediaSSRC = j.ssrc
	pNack.Lost(seq)
	dataNack := pNack.Bytes()
	rtcpPacketNack := &RtpUdpPacket{
		RAddr: j.rAddr,
		Data:  dataNack,
	}
	j.log.Infof("send RTCP NACK %b, %s", dataNack, pNack.String())
	select {
	case j.outRTCP <- rtcpPacketNack:
	default:
		j.log.Warnf("outRTCP is full, dropping packet rtcpPacketNack")
	}
}

func (j *JitterBuffer) SendREMB(bitrate int) {
	if j.rAddr == nil {
		j.log.Warnf("could not send any packets j.rAddr is nil")
		return
	}
	remb := rtcp.NewPacketALFBRemb()
	remb.SetBitrate(uint32(bitrate))
	remb.SSRCs = append(remb.SSRCs, j.ssrc)
	dataRemb := remb.Bytes()
	rtcpPacketRemb := &RtpUdpPacket{
		RAddr: j.rAddr,
		Data:  dataRemb,
	}
	j.log.Infof("send RTCP REMB with bitrate %d , %s", bitrate, remb.String())
	select {
	case j.outRTCP <- rtcpPacketRemb:
	default:
		j.log.Warnf("outRTCP is full, dropping packet rtcpPacketRemb")
	}
	j.n.Bus <- remb
}

func (j *JitterBuffer) SendPLI() {
	if j.rAddr == nil {
		j.log.Warnf("could not send any packets j.rAddr is nil")
		return
	}
	pli := rtcp.NewPacketPSFBPli()
	pli.PacketPSFB.SenderSSRC = 0
	pli.PacketPSFB.MediaSSRC = j.ssrc
	dataPli := pli.Bytes()
	rtcpPacketPli := &RtpUdpPacket{
		RAddr: j.rAddr,
		Data:  dataPli,
	}
	j.log.Infof("send RTCP PLI to %s:%d", j.rAddr.IP.String(), j.rAddr.Port)
	select {
	case j.outRTCP <- rtcpPacketPli:
	default:
		j.log.Warnf("outRTCP is full, dropping packet rtcpPacketPli")
	}
}

func (j *JitterBuffer) SendFIR() {
	if j.rAddr == nil {
		j.log.Warnf("could not send any packets j.rAddr is nil")
		return
	}
	fir := rtcp.NewPacketPSFBFir()
	fir.PacketPSFB.SenderSSRC = 0
	fir.PacketPSFB.MediaSSRC = j.ssrc
	dataFir := fir.Bytes()
	rtcpPacketFir := &RtpUdpPacket{
		RAddr: j.rAddr,
		Data:  dataFir,
	}
	j.log.Infof("send RTCP FIR to %s:%d", j.rAddr.IP.String(), j.rAddr.Port)
	select {
	case j.outRTCP <- rtcpPacketFir:
	default:
		j.log.Warnf("outRTCP is full, dropping packet rtcpPacketFir")
	}
}

func (j *JitterBuffer) ChangeBitrate(percentDelta float64) {
	var newBitrate int
	dontRemb := false
	newBitrate = int(float64(j.videoBitrate) + (float64(j.videoBitrate) * percentDelta))

	if newBitrate < j.bitrate.Min {
		j.log.Infof("bitrate is now at the minimum (%d bits/s)", j.bitrate.Min)
		newBitrate = j.bitrate.Min
	} else {
		if newBitrate > j.bitrate.Max {
			j.log.Infof("bitrate is now at the maximum (%d bits/s)", j.bitrate.Max)
			dontRemb = true
		} else {
			if newBitrate == j.bitrate.Min {
				dontRemb = true
			}
		}
	}
	if dontRemb == false {
		j.log.Infof("bitrate change with REMB to %d kbits/s", newBitrate/1000)
		j.SendREMB(newBitrate)
		j.videoBitrate = newBitrate
	}

	return
}
