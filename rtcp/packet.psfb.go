package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

/*
  @see https://tools.ietf.org/html/rfc4585#section-6.1

0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|V=2|P|   FMT   |       PT      |          length               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  SSRC of packet sender                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  SSRC of media source                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
:            Feedback Control Information (FCI)                 :
:                                                               :

       Figure 3: Common Packet Format for Feedback Messages

*/
type PacketPSFB struct {
	PacketRTCP
	SenderSSRC uint32
	MediaSSRC  uint32
	// private
	size int
}

func NewPacketPSFB() *PacketPSFB {
	return new(PacketPSFB)
}

func (p *PacketPSFB) ParsePacketRTCP(packet *PacketRTCP) error {
	// load packet
	p.PacketRTCP = *packet
	// setup offset
	offset := packet.GetOffset()
	//
	if p.GetSize() < offset+8 {
		return errors.New("ssrc size")
	}
	p.SenderSSRC = binary.BigEndian.Uint32(p.GetData()[offset : offset+4])
	p.MediaSSRC = binary.BigEndian.Uint32(p.GetData()[offset+4 : offset+8])
	p.size = offset + 8
	return nil
}

func (p *PacketPSFB) GetOffset() int {
	return p.Header.GetSize() + 8
}

func (p *PacketPSFB) GetSizeFCI() int {
	size := p.Header.GetFullPacketSize() - p.GetOffset()
	if size > 0 {
		return size
	}
	return 0
}

// return the message type (const PSFB_XXX)
func (p *PacketPSFB) GetMessageType() uint8 {
	return p.Header.ReceptionCount
}

func (p *PacketPSFB) Bytes() []byte {
	var result []byte

	result = append(result, p.PacketRTCP.Bytes()...)
	result = append(result, uint32ToBytes(p.SenderSSRC)...)
	result = append(result, uint32ToBytes(p.MediaSSRC)...)
	return result
}

func (p *PacketPSFB) String() string {
	return fmt.Sprintf(
		"%s sssrc=%d msrc=%d",
		p.PacketRTCP.String(),
		p.SenderSSRC,
		p.MediaSSRC,
	)
}
