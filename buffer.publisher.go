package main

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

type JitterModeOptions int

const (
	JitterModeSender JitterModeOptions = iota
	JitterModeReceiver
)

type BufferPublisherUnsorted struct {
	sync.RWMutex
	d map[uint64]*srtp.PacketRTP
}

func NewBufferPublisherUnsorted() (b *BufferPublisherUnsorted) {
	b = new(BufferPublisherUnsorted)
	b.d = make(map[uint64]*srtp.PacketRTP)

	return
}

func (b *BufferPublisherUnsorted) Put(p *srtp.PacketRTP) {
	b.Lock()
	defer b.Unlock()

	b.d[p.GetSeqNumberWithCycles()] = p
}

func (b *BufferPublisherUnsorted) Get(seq uint64) (p *srtp.PacketRTP) {
	b.Lock()
	defer b.Unlock()

	p = b.d[seq]
	if p != nil {
		delete(b.d, seq)
	}

	return
}

func (b *BufferPublisherUnsorted) IsExist(seq uint64) bool {
	b.Lock()
	defer b.Unlock()

	if b.d[seq] != nil {
		return true
	}

	return false
}

func (b *BufferPublisher) GetLast() *srtp.PacketRTP {
	b.RLock()
	defer b.RUnlock()

	return b.d[b.last]
}

func (b *BufferPublisherUnsorted) GetAllSlice() (sp []*srtp.PacketRTP) {
	b.RLock()
	defer b.RUnlock()

	for _, v := range b.d {
		sp = append(sp, v)
	}
	sort.Slice(sp, func(i, j int) bool { return sp[i].GetSeqNumberWithCycles() < sp[j].GetSeqNumberWithCycles() })
	b.d = make(map[uint64]*srtp.PacketRTP)

	return
}

func (b *BufferPublisherUnsorted) Purge() {
	b.d = make(map[uint64]*srtp.PacketRTP)

	return
}

type BufferPublisher struct {
	sync.RWMutex
	d               map[uint64]*srtp.PacketRTP
	lastPacketRTP   *srtp.PacketRTP
	bufferUnsorted  *BufferPublisherUnsorted
	first           uint64
	last            uint64
	timeSize        uint64
	freq            uint32
	packetCount			uint64
	avgTimeInterval uint64
	maxEntries      int
}

func NewBufferPublisher(bufferUnsorted *BufferPublisherUnsorted, freq uint32, maxEntries int) (b *BufferPublisher) {
	b = new(BufferPublisher)
	b.d = make(map[uint64]*srtp.PacketRTP)
	b.bufferUnsorted = bufferUnsorted
	b.freq = freq
	b.maxEntries = maxEntries

	return
}

func (b *BufferPublisher) Get(seq uint64) *srtp.PacketRTP {
	b.RLock()
	defer b.RUnlock()

	return b.d[seq]
}

func (b *BufferPublisher) Purge() {
	b.Lock()
	defer b.Unlock()

	b.first = 0
	b.last = 0
	b.d = make(map[uint64]*srtp.PacketRTP)
	b.lastPacketRTP = nil
	b.bufferUnsorted.Purge()

	return
}

func (b *BufferPublisher) GetLastPacketRTP() *srtp.PacketRTP {
	return b.lastPacketRTP
}

func (b *BufferPublisher) Push(log plogger.PLogger, p *srtp.PacketRTP, force bool, acceptDisorder bool) (err error) {
	b.Lock()
	defer b.Unlock()

	log.Debugf("(%d) b.first = %d, b.last = %d, PUSH PACKET SEQ %d", time.Now().UnixNano(), b.first, b.last, p.GetSeqNumberWithCycles())
	seq := p.GetSeqNumberWithCycles()
	if b.first != 0 {
		if seq == b.last+1 {
			b.last = seq
		} else {
			if acceptDisorder == false {
				// Disordered Packet
				err = fmt.Errorf("Packet is unsorted and could not be pushed in BufferPublisher. Sequence number expected is %d and packet sequence number is %d", b.last+1, seq)
				return
			} else {
				b.last = seq
			}
		}
	}
	if seq < b.first {
		if force == false {
			err = fmt.Errorf("RTP packet seqNumber %d/%d is in the past, b.first is %d/%d: DROP", p.GetSeqNumber(), p.GetSeqNumberWithCycles(), GetSeqNumberWithoutCycles(b.first), b.first)
			return
		} else {
			b.first = seq
		}
	}
	if b.first == 0 {
		if b.lastPacketRTP != nil && b.lastPacketRTP.GetSeqNumberWithCycles() >= seq {
			err = fmt.Errorf("Packet has arrived too late or duplicated last RTP packet seq is %d, and pushed RTP packet seq is %d", b.lastPacketRTP.GetSeqNumberWithCycles(), seq)
			return
		}
		b.first = seq
		b.last = seq
	} else {
		if b.d[seq] != nil {
			err = fmt.Errorf("duplicate packet seqNumber %d", seq)
			return
		}
	}

	b.d[seq] = p
	b.lastPacketRTP = p

	// Check if we need to push unsorted packet in buffer
	stop := false
	for i := seq + 1; stop == false; i++ {
		p := b.bufferUnsorted.Get(i)
		if p != nil {
			b.d[i] = p
			b.lastPacketRTP = p
			b.last = i
		} else {
			stop = true
		}
	}

	if b.maxEntries != 0 && int(b.last-b.first) > b.maxEntries {
		delete(b.d, b.first)
		b.first++
	}
	b.adjustTimeSize()
	b.adjustAvgIntervalTime()

	b.packetCount++

	return
}

func (b *BufferPublisher) PushAllUnsorted(pushWithHoles bool) {
	b.Lock()
	defer b.Unlock()

	b.first = 0
	for _, p := range b.bufferUnsorted.GetAllSlice() {
		if pushWithHoles == false && p.GetSeqNumberWithCycles() != b.last+1 && b.last != 0 {
			//fmt.Printf("RTP Packet is unsorted and could not be pushed in sorted buffer. Seq %d, waiting %d\n", p.GetSeqNumberWithCycles(), b.last+1)
			//fmt.Printf("Repush in unsorted buffer seq %d\n", p.GetSeqNumberWithCycles())
			b.bufferUnsorted.Put(p)
		} else {
			//fmt.Printf("PushAllUnsorted PUSH PACKET SEQ %d in sorted buffer\n", p.GetSeqNumberWithCycles())
			b.d[p.GetSeqNumberWithCycles()] = p
			b.lastPacketRTP = p
			if b.first == 0 {
				b.first = p.GetSeqNumberWithCycles()
			}
			b.last = p.GetSeqNumberWithCycles()
		}
	}
}

func (b *BufferPublisher) Dump() string {
	b.RLock()
	defer b.RUnlock()

	str := ""
	for _, v := range b.d {
		str += fmt.Sprintf("( %d - %d ) ", v.GetSeqNumber(), v.GetTimestampWithCycles())
	}
	return fmt.Sprintf("%#v\n", str)
}

func (b *BufferPublisher) Pop() (p *srtp.PacketRTP) {
	b.Lock()
	defer b.Unlock()

	p = b.d[b.first]
	if p != nil {
		delete(b.d, b.first)
		if b.first == b.last {
			b.first = 0
			b.last = 0
		} else {
			b.first++
		}
		b.adjustTimeSize()
	}

	return
}

func (b *BufferPublisher) adjustAvgIntervalTime() (err error) {
	if b.last-b.first < 2 || b.d[b.last] == nil || b.d[b.last-1] == nil {
		return
	}
	lastInterval := uint64((float64(b.d[b.last].GetTimestampWithCycles())/float64(b.freq) - float64(b.d[b.last-1].GetTimestampWithCycles())/float64(b.freq)) * 1000000000)
	if lastInterval <= 0 {
		if lastInterval < 0 {
			err = errors.New("average time is negative, it's impossible... a wrong RTP packet timestamp has been inserted on the list")
		}
		return
	}
	if b.avgTimeInterval == 0 {
		b.avgTimeInterval = lastInterval
	} else {
		b.avgTimeInterval = ((b.avgTimeInterval * b.packetCount) + lastInterval) / (b.packetCount+1)
	}

	return
}

func (b *BufferPublisher) GetAvgIntervalTime() uint64 {
	return b.avgTimeInterval
}

func (b *BufferPublisher) adjustTimeSize() {
	if b.d[b.first] == nil {
		//b.timeSize = 0
		return
	}
	b.timeSize = uint64((float64(b.lastPacketRTP.GetTimestampWithCycles())/float64(b.freq) - float64(b.d[b.first].GetTimestampWithCycles())/float64(b.freq)) * 1000000000)
}

func (b *BufferPublisher) GetTimeSize() uint64 {
	return b.timeSize
}

func (b *BufferPublisher) GetTimeSizeBetween(sSeq uint64, eSeq uint64) (size int64, err error) {
	b.RLock()
	defer b.RUnlock()

	packetRTP1 := b.d[eSeq]
	packetRTP2 := b.d[sSeq]
	if packetRTP1 == nil || packetRTP2 == nil {
		err = errors.New("one of the packets are missing/lost")
		return
	}
	size = int64((float64(packetRTP1.GetTimestampWithCycles())/float64(b.freq) - float64(packetRTP2.GetTimestampWithCycles())/float64(b.freq)) * 1000000000)
	return
}

func (b *BufferPublisher) GetSize() uint64 {
	b.RLock()
	defer b.RUnlock()

	return b.last - b.first
}

func (b *BufferPublisher) GetFirstSequenceNumber() uint64 {
	b.RLock()
	defer b.RUnlock()

	return b.first
}

func (b *BufferPublisher) GetLastSequenceNumber() uint64 {
	b.RLock()
	defer b.RUnlock()

	return b.last
}
