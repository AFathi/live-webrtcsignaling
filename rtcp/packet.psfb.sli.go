package rtcp

import (
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

	@see https://tools.ietf.org/html/rfc4585#section-6.3.2
	The SLI FB message is identified by PT=PSFB and FMT=2.
	The FCI field MUST contain at least one and MAY contain more than one
	SLI.

	@see https://tools.ietf.org/html/rfc4585#section-6.3.2.2

	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |            First        |        Number           | PictureID |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

					Figure 6: Syntax of the Slice Loss Indication (SLI)

*/
type PacketPSFBSli struct {
	PacketPSFB
	PSFBSlis PSFBSlis
	// private
	size int
}

func NewPacketPSFBSli() *PacketPSFBSli {
	return new(PacketPSFBSli)
}

func (p *PacketPSFBSli) ParsePacketPSFB(packet PacketPSFB) error {
	// load packet
	p.PacketPSFB = packet
	// setup offset
	offset := packet.GetOffset()
	//
	for offset < p.GetSize() {
		item := NewPSFBSli()
		err := item.Parse(p.GetData()[offset:])
		if err != nil {
			return err
		}
		p.PSFBSlis = append(p.PSFBSlis, *item)
		offset += item.GetSize()
	}
	p.size = offset
	return nil
}

func (p *PacketPSFBSli) String() string {
	return fmt.Sprintf(
		"[RTCP-PSFB-SLI %s %s]",
		p.PacketPSFB.String(),
		p.PSFBSlis.String(),
	)
}
