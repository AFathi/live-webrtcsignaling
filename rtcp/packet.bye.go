package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
  @see https://tools.ietf.org/html/rfc3550#section-6.4.1

				0                   1                   2                   3
				0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
			 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
			 |V=2|P|    SC   |   PT=BYE=203  |             length            |
			 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
			 |                           SSRC/CSRC                           |
			 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
			 :                              ...                              :
			 +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
 (opt) |     length    |               reason for leaving            ...
		   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
type PacketBYE struct {
	PacketRTCP
	SSRCs  SSRCs
	Length int
	Reason string
	// private
	size int
}

func NewPacketBYE() *PacketBYE {
	return new(PacketBYE)
}

func (p *PacketBYE) ParsePacketRTCP(packet *PacketRTCP) error {
	// load packet
	p.PacketRTCP = *packet
	// setup offset
	offset := packet.GetOffset()
	//
	ssrcSize := 4
	for nbSsrc := p.Header.ReceptionCount; nbSsrc > 0; nbSsrc-- {
		if p.GetSize() < offset+ssrcSize {
			return errors.New("PacketBYE ssrc size")
		}
		ssrc := binary.BigEndian.Uint32(p.GetData()[offset : offset+ssrcSize])
		p.SSRCs = append(p.SSRCs, ssrc)
		offset += ssrcSize
	}
	if p.Header.GetFullPacketSize() > offset {
		if p.GetSize() < offset+1 {
			return errors.New("PacketBYE length size")
		}
		p.Length = int(p.GetData()[offset])
		offset += 1
		if p.GetSize() < offset+p.Length {
			return errors.New("PacketBYE text size")
		}
		p.Reason = string(p.GetData()[offset : offset+p.Length])
		p.size = offset + p.Length
	} else {
		p.size = offset
	}
	return nil
}

type SSRCs []uint32

func (l *SSRCs) String() string {
	var ssrcs []string

	for _, ssrc := range *l {
		ssrcs = append(ssrcs, fmt.Sprintf("%d", ssrc))
	}
	return "ssrc=[" + strings.Join(ssrcs, ", ") + "]"
}

func (p *PacketBYE) String() string {
	reason := ""
	if p.Length > 0 {
		reason = fmt.Sprintf(" l=%d r=%s", p.Length, p.Reason)
	}
	return fmt.Sprintf(
		"[RTCP-BYE %s %s%s]",
		p.PacketRTCP.String(),
		p.SSRCs.String(),
		reason,
	)
}
