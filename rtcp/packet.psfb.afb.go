package rtcp

import (
	"fmt"
)

/*
	Application layer FB messages

	From a protocol
   point of view, an application layer FB message is treated as a
   special case of a payload-specific FB message.

	@see https://tools.ietf.org/html/rfc4585#section-6.1
  @see https://tools.ietf.org/html/rfc4585#section-6.4

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

	@see https://tools.ietf.org/html/rfc4585#section-6.4
	Application layer FB messages are a special case of payload-specific
   messages and are identified by PT=PSFB and FMT=15.

	Application Message (FCI): variable length
*/
type PacketPSFBAfb struct {
	PacketPSFB
	// private
	size int
}

func NewPacketPSFBAfb() *PacketPSFBAfb {
	return new(PacketPSFBAfb)
}

func (p *PacketPSFBAfb) ParsePacketPSFB(packet PacketPSFB) error {
	// load packet
	p.PacketPSFB = packet
	// setup offset
	p.size = p.PacketPSFB.GetOffset()
	//
	return nil
}

/*
 * @see https://tools.ietf.org/html/draft-alvestrand-rmcat-remb-00#section-2.2
 * Remb PSFBAfb packet contains at least 12 bytes in FCI section
 * and contains a unique identifier 'R' 'E' 'M' 'B' (4 ASCII char)
 */
func (p *PacketPSFBAfb) IsREMB() bool {
	sizeFci := p.PacketPSFB.GetSizeFCI()
	offset := p.PacketPSFB.GetOffset()
	return sizeFci >= 12 &&
		string(p.GetData()[offset:offset+4]) == "REMB"
}

func (p *PacketPSFBAfb) Bytes() []byte {
	return p.PacketPSFB.Bytes()
}

func (p *PacketPSFBAfb) String() string {
	return fmt.Sprintf(
		"[RTCP-PSFB-AFB %s]",
		p.PacketPSFB.String(),
	)
}
