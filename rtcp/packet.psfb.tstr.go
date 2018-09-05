package rtcp

import (
	"errors"
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

	@see https://tools.ietf.org/html/rfc5104#section-4.3.2
	The TSTR feedback message is identified by RTCP packet type value
	PT=PSFB and FMT=5.
	The FCI field MUST contain one or more TSTR FCI entries.

	@see https://tools.ietf.org/html/rfc5104#section-4.3.2.1

	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                              SSRC                             |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |  Seq nr.      |  Reserved                           | Index   |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

			 Figure 5 - Syntax of an FCI Entry in the TSTR Message

*/
type PacketPSFBTstr struct {
	PacketPSFB
	PSFBTstrs PSFBTstrs
	// private
	size int
}

func NewPacketPSFBTstr() *PacketPSFBTstr {
	return new(PacketPSFBTstr)
}

func (p *PacketPSFBTstr) ParsePacketPSFB(packet PacketPSFB) error {
	// load packet
	p.PacketPSFB = packet
	// setup offset
	offset := packet.GetOffset()
	// at least one entry
	itemLength := 8
	if offset+itemLength < p.GetSize() {
		return errors.New("tstr size")
	}
	//
	for offset < p.GetSize() {
		item := NewPSFBTstr()
		err := item.Parse(p.GetData()[offset:])
		if err != nil {
			return err
		}
		p.PSFBTstrs = append(p.PSFBTstrs, *item)
		offset += item.GetSize()
	}
	//
	if len(p.PSFBTstrs) == 0 {
		return errors.New("tstr should have one FCI")
	}
	p.size = offset
	return nil
}

func (p *PacketPSFBTstr) String() string {
	return fmt.Sprintf(
		"[RTCP-PSFB-TSTR %s %s]",
		p.PacketPSFB.String(),
		p.PSFBTstrs.String(),
	)
}
