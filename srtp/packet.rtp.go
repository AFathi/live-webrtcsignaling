package srtp

import (
	"encoding/binary"
	"fmt"

	"github.com/heytribe/live-webrtcsignaling/packet"
)

type PacketRTP struct {
	IPacketUDP
	seqCycle uint32
	tsCycle  uint32
}

func NewPacketRTP(input IPacketUDP) *PacketRTP {
	p := new(PacketRTP)
	p.IPacketUDP = input
	return p
}

func (p *PacketRTP) GetSSRCid() uint32 {
	return binary.BigEndian.Uint32(p.GetData()[8:12])
}

func (p *PacketRTP) GetSSRC() string {
	return fmt.Sprintf("%X", p.GetData()[8:12])
}

func (p *PacketRTP) SetSSRC(ssrc uint32) {
	data := p.GetData()
	binary.BigEndian.PutUint32(data[8:12], ssrc)
	p.SetData(data)
}

func (p *PacketRTP) GetPT() int {
	return int(p.GetData()[1] & 0x7F) // 0111 1111
}

func (p *PacketRTP) SetPT(pt int) {
	data := p.GetData()
	fmt.Printf("SETTING PT %d", pt)
	data[1] = data[1]&0x80 | byte(pt)
	p.SetData(data)
}

/*
    VP8:
  Marker bit (M):  MUST be set for the very last packet of each encoded
	 frame in line with the normal use of the M bit in video formats.
	 This enables a decoder to finish decoding the picture, where it
	 otherwise may need to wait for the next packet to explicitly know
	 that the frame is complete.
*/
func (p *PacketRTP) GetMarkerBit() bool {
	if (p.GetData()[1] & 0x80) != 0 {
		return true
	}
	return false
}

func (p *PacketRTP) GetTimestamp() uint32 {
	return binary.BigEndian.Uint32(p.GetData()[4:8])
}

func (p *PacketRTP) SetTimestamp(timestamp uint32) {
	data := p.GetData()
	binary.BigEndian.PutUint32(data[4:8], timestamp)
}

func (p *PacketRTP) GetSeqNumber() uint16 {
	return binary.BigEndian.Uint16(p.GetData()[2:4])
}

func (p *PacketRTP) SetSeqNumber(seq uint16) {
	data := p.GetData()
	binary.BigEndian.PutUint16(data[2:4], seq)
}

func (p *PacketRTP) RTXExtractOriginal(ssrc uint32) (pOrigin *PacketRTP, osn uint16) {
	data := p.GetData()
	cc := (data[0] & 0x0f) << 2
	headerSize := 12 + cc
	osn = binary.BigEndian.Uint16(data[headerSize : headerSize+2])
	originData := append(data[0:headerSize], data[headerSize+2:]...)
	// Restore origin Payload Type
	originData[1] = originData[1]&0x80 | (originData[1]&0x7f - 1)
	// Restore original Sequence Number
	binary.BigEndian.PutUint16(originData[2:4], osn)
	// Restore original SSRC
	binary.BigEndian.PutUint32(originData[8:12], ssrc)
	pOrigin = NewPacketRTP(packet.NewUDP())
	pOrigin.SetData(originData)

	return
}

func (p *PacketRTP) GetPayloadSize() uint32 {
	// fixme... we should take care of the padding...
	return uint32(len(p.GetData()) - 16)
}

func (p *PacketRTP) GetSize() int {
	return len(p.GetData())
}

func (p *PacketRTP) GetSeqCycle() uint32 {
	return p.seqCycle
}

func (p *PacketRTP) SetSeqCycle(cycle uint32) {
	p.seqCycle = cycle
}

func (p *PacketRTP) GetTsCycle() uint32 {
	return p.tsCycle
}

func (p *PacketRTP) SetTsCycle(cycle uint32) {
	p.tsCycle = cycle
}

func (p *PacketRTP) GetSeqNumberWithCycles() uint64 {
	return uint64(p.GetSeqNumber()) + uint64(p.GetSeqCycle())*(uint64(^uint16(0))+1)
}

func (p *PacketRTP) GetTimestampWithCycles() uint64 {
	return uint64(p.GetTimestamp()) + uint64(p.GetTsCycle())*(uint64(^uint32(0))+1)
}
