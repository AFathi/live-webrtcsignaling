package rtcp

import (
	"errors"
	"fmt"
)

/*
  @see https://tools.ietf.org/html/rfc4585#section-6.1
  @see https://tools.ietf.org/html/rfc4585#section-6.2.1

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

*/
type PacketRTPFBNack struct {
	PacketRTPFB
	RTPFBNacks RTPFBNacks
	// private
	size int
}

func NewPacketRTPFBNack() *PacketRTPFBNack {
	return new(PacketRTPFBNack)
}

func (p *PacketRTPFBNack) ParsePacketRTPFB(packet PacketRTPFB) error {
	// load packet
	p.PacketRTPFB = packet
	// setup offset
	offset := packet.GetOffset()
	// The length of the FB message MUST be set to 2+n,
	// with n being the number of Generic NACKs contained in the FCI field.
	if p.PacketRTPFB.Header.Length < 3 {
		return errors.New("nack size")
	}
	for offset < packet.GetOffset()+(p.PacketRTPFB.Header.GetLength()-2)*4 {
		item := NewRTPFBNack()
		if p.GetSize() < offset {
			return errors.New("nack item size")
		}
		err := item.Parse(p.GetData()[offset:])
		if err != nil {
			return err
		}
		p.RTPFBNacks = append(p.RTPFBNacks, *item)
		offset += item.GetSize()
	}
	p.size = offset
	return nil
}

// single nack
func (p *PacketRTPFBNack) Lost(packetId uint16) {
	fciRTPFBNack := NewRTPFBNack()
	fciRTPFBNack.PacketId = packetId
	p.RTPFBNacks = append(p.RTPFBNacks, *fciRTPFBNack)
}

// set RTPFBNacks as if packet between from & to where lost
// limit to 3 * 17 packets losts
func (p *PacketRTPFBNack) LostBetween(from, to uint16) {
	if from >= to {
		return // nothing
	}
	// empty the slice
	p.RTPFBNacks = p.RTPFBNacks[:0]
	// pid starts at from + 1 & nack FCI max from + 1 + 16
	pid := from + 1
	for i := uint16(0); i < 3 && pid < to; i++ {
		fciRTPFBNack := NewRTPFBNack()
		fciRTPFBNack.PacketId = pid
		j := uint(1)
		for ; j < 17 && pid+uint16(j) < to; j++ {
			fciRTPFBNack.Lost(j)
		}
		p.RTPFBNacks = append(p.RTPFBNacks, *fciRTPFBNack)
		// bumping pid
		pid = pid + uint16(j)
	}
}

func (p *PacketRTPFBNack) Bytes() []byte {
	var result []byte

	p.PacketRTPFB.PacketRTCP.Header.Version = 2
	p.PacketRTPFB.PacketRTCP.Header.Padding = false
	// FMT is in RC field
	p.PacketRTPFB.PacketRTCP.Header.ReceptionCount = FMT_RTPFB_NACK
	// PT
	p.PacketRTPFB.PacketRTCP.Header.PacketType = PT_RTPFB
	// length
	p.PacketRTPFB.PacketRTCP.Header.Length = uint16(2 + len(p.RTPFBNacks))

	result = append(result, p.PacketRTPFB.Bytes()...)
	result = append(result, p.RTPFBNacks.Bytes()...)
	return result
}

func (p *PacketRTPFBNack) String() string {
	return fmt.Sprintf(
		"[RTCP-RTPFB-NACK %s %s]",
		p.PacketRTPFB.String(),
		p.RTPFBNacks.String(),
	)
}
