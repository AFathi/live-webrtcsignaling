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
  :                        header (PT=200)                        :
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
  |                         SSRC of sender                        |
  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
  |                         sender infos                          |
  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
  |                         report block 1                        |
  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
  :                  ... other report blocks ...                  |
  +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
  |                  profile-specific extensions                  |
  +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
type PacketSR struct {
	PacketRTCP
	SSRC         uint32
	SenderInfos  SenderInfos
	ReportBlocks ReportBlocks
	// private
	size int
}

func NewPacketSR() *PacketSR {
	return new(PacketSR)
}

func (p *PacketSR) ParsePacketRTCP(packet *PacketRTCP) error {
	var err error

	// loading packet
	p.PacketRTCP = *packet
	// setup offset
	offset := packet.GetOffset()
	// ssrc
	ssrcSize := 4
	if p.GetSize() < offset+ssrcSize {
		return errors.New("PacketSR ssrc size")
	}
	p.SSRC = binary.BigEndian.Uint32(p.GetData()[offset : offset+ssrcSize])
	offset += ssrcSize
	// sender infos
	if p.GetSize() < offset {
		return errors.New("PacketSR sender infos size")
	}
	if err = p.SenderInfos.Parse(p.GetData()[offset:]); err != nil {
		return err
	}
	offset += p.SenderInfos.GetSize()
	// report blocks
	for nbRBlocks := p.Header.ReceptionCount; nbRBlocks > 0; nbRBlocks-- {
		rb := NewReportBlock()
		if p.GetSize() < offset {
			return errors.New("PacketSR rr size")
		}
		if err = rb.Parse(p.GetData()[offset:]); err != nil {
			return err
		}
		offset += rb.GetSize()
	}
	// FIXME: remaining data.
	p.size = offset
	return nil
}

func (p *PacketSR) ComputeHeaders() {
	p.PacketRTCP.Header.Version = 2
	p.PacketRTCP.Header.Padding = false
	// FMT is in RC field
	p.PacketRTCP.Header.ReceptionCount = uint8(len(p.ReportBlocks))
	// PT
	p.PacketRTCP.Header.PacketType = PT_SR
	// compute report blocks size in words of 32 bits
	rbSizeInBytes := 0
	for i := 0; i < len(p.ReportBlocks); i++ {
		rbSizeInBytes = p.ReportBlocks[i].GetSize()
	}
	rbSizeInWords := rbSizeInBytes / 4
	p.PacketRTCP.Header.Length = uint16(1 /* SSRC */ + 5 /* Sender Infos */ + rbSizeInWords)
}

func (p *PacketSR) Bytes() []byte {
	var result []byte

	p.ComputeHeaders()
	result = append(result, p.PacketRTCP.Bytes()...)
	result = append(result, uint32ToBytes(p.SSRC)...)
	result = append(result, p.SenderInfos.Bytes()...)
	result = append(result, p.ReportBlocks.Bytes()...)
	return result
}

func (p *PacketSR) String() string {
	return fmt.Sprintf(
		"[RTCP-SR %s ssrc=%d %s %s]",
		p.PacketRTCP.String(),
		p.SSRC,
		p.SenderInfos.String(),
		p.ReportBlocks.String(),
	)
}
