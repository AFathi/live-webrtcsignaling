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

	@see https://tools.ietf.org/html/rfc5104#section-4.3.3
	The TSTN message is identified by RTCP packet type value PT=PSFB and
	FMT=6.
	The FCI field SHALL contain one or more TSTN FCI entries.

	@see https://tools.ietf.org/html/rfc5104#section-4.3.3.1

	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                              SSRC                             |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |  Seq nr.      |  Reserved                           | Index   |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

									Figure 6 - Syntax of the TSTN

*/
type PacketPSFBTstn struct {
	PacketPSFB
	PSFBTstns PSFBTstns
	// private
	size int
}

func NewPacketPSFBTstn() *PacketPSFBTstn {
	return new(PacketPSFBTstn)
}

func (p *PacketPSFBTstn) ParsePacketPSFB(packet PacketPSFB) error {
	// load packet
	p.PacketPSFB = packet
	// setup offset
	offset := packet.GetOffset()
	// at least one entry
	itemLength := 8
	if offset+itemLength < p.GetSize() {
		return errors.New("tstn size")
	}
	//
	for offset < p.GetSize() {
		item := NewPSFBTstn()
		err := item.Parse(p.GetData()[offset:])
		if err != nil {
			return err
		}
		p.PSFBTstns = append(p.PSFBTstns, *item)
		offset += item.GetSize()
	}
	//
	if len(p.PSFBTstns) == 0 {
		return errors.New("tstn should have one FCI")
	}
	p.size = offset
	return nil
}

func (p *PacketPSFBTstn) String() string {
	return fmt.Sprintf(
		"[RTCP-PSFB-TSTN %s %s]",
		p.PacketPSFB.String(),
		p.PSFBTstns.String(),
	)
}
