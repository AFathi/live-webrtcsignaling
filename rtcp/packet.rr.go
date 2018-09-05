package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

/*
  @see https://tools.ietf.org/html/rfc3550#section-6.4.1

   0                   1                   2                   3
   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
  :                        header (PT=201)                        :
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
  |                         SSRC of sender                        |
  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
  |                         report block 1                        |
  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
  :                  ... other report blocks ...                  |
  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
  |                  profile-specific extensions                  |
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
type PacketRR struct {
	PacketRTCP
	SSRC         uint32
	ReportBlocks ReportBlocks
	// private
	size int
}

func NewPacketRR() *PacketRR {
	return new(PacketRR)
}

func (p *PacketRR) ParsePacketRTCP(packet *PacketRTCP) error {
	var err error

	// load packet
	p.PacketRTCP = *packet
	// setup offset
	offset := packet.GetOffset()
	// ssrc
	ssrcSize := 4
	if p.GetSize() < offset+ssrcSize {
		return errors.New("PacketRR ssrc")
	}
	p.SSRC = binary.BigEndian.Uint32(p.GetData()[offset : offset+ssrcSize])
	offset += ssrcSize
	// report blocks
	for nbRBlocks := p.Header.ReceptionCount; nbRBlocks > 0; nbRBlocks-- {
		rb := NewReportBlock()
		if p.GetSize() < offset {
			return errors.New("PacketRR RBlock size")
		}
		if err = rb.Parse(p.GetData()[offset:]); err != nil {
			return err
		}
		p.ReportBlocks = append(p.ReportBlocks, *rb)
		offset += rb.GetSize()
	}
	// FIXME: remaining data.
	p.size = offset
	return nil
}

func (p *PacketRR) Bytes() []byte {
	var result []byte

	p.PacketRTCP.Header.Version = 2
	p.PacketRTCP.Header.Padding = false
	// FMT is in RC field
	p.PacketRTCP.Header.ReceptionCount = uint8(len(p.ReportBlocks))
	// PT
	p.PacketRTCP.Header.PacketType = PT_RR
	// compute report blocks size in words of 32 bits
	rbSizeInBytes := 0
	for i := 0; i < len(p.ReportBlocks); i++ {
		rbSizeInBytes = p.ReportBlocks[i].GetSize()
	}
	rbSizeInWords := rbSizeInBytes / 4
	p.PacketRTCP.Header.Length = uint16(1 + rbSizeInWords)
	result = append(result, p.PacketRTCP.Bytes()...)
	result = append(result, uint32ToBytes(p.SSRC)...)
	result = append(result, p.ReportBlocks.Bytes()...)
	return result
}

func (p *PacketRR) String() string {
	return fmt.Sprintf(
		"[RTCP-RR %s ssrc=%d %s]",
		p.PacketRTCP.String(),
		p.SSRC,
		p.ReportBlocks.String(),
	)
}
