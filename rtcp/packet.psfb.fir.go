package rtcp

import (
	"fmt"
)

/*
  @see https://tools.ietf.org/html/rfc4585#section-6.4
  @see https://tools.ietf.org/html/rfc5104#section-4.3.1

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

	@see https://tools.ietf.org/html/rfc5104#section-4.3.1
	The FIR message is identified by RTCP packet type value PT=PSFB and FMT=4.
  The FCI field MUST contain one or more FIR entries.  Each entry
   applies to a different media sender, identified by its SSRC.

	@see https://tools.ietf.org/html/rfc5104#section-4.3.1.1

	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                              SSRC                             |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 | Seq nr.       |    Reserved                                   |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

			 Figure 4 - Syntax of an FCI Entry in the FIR Message

*/
type PacketPSFBFir struct {
	PacketPSFB
	PSFBFirs PSFBFirs
	// private
	size int
}

func NewPacketPSFBFir() *PacketPSFBFir {
	return new(PacketPSFBFir)
}

func (p *PacketPSFBFir) ParsePacketPSFB(packet PacketPSFB) error {
	// load packet
	p.PacketPSFB = packet
	// setup offset
	offset := packet.GetOffset()
	//
	for offset < p.GetSize() {
		item := NewPSFBFir()
		err := item.Parse(p.GetData()[offset:])
		if err != nil {
			return err
		}
		p.PSFBFirs = append(p.PSFBFirs, *item)
		offset += item.GetSize()
	}
	p.size = offset
	return nil
}

func (p *PacketPSFBFir) String() string {
	return fmt.Sprintf(
		"[RTCP-PSFB-FIR %s %s]",
		p.PacketPSFB.String(),
		p.PSFBFirs.String(),
	)
}
