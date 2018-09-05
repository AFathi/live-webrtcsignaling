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

	@see https://tools.ietf.org/html/rfc4585#section-6.3.1
	The PLI FB message is identified by PT=PSFB and FMT=1.
	There MUST be exactly one PLI contained in the FCI field.

	but https://tools.ietf.org/html/rfc4585#section-6.3.1.2

	PLI does not require parameters.  Therefore, the length field MUST be
   2, and there MUST NOT be any Feedback Control Information.
*/
type PacketPSFBPli struct {
	PacketPSFB
	// private
	size int
}

func NewPacketPSFBPli() *PacketPSFBPli {
	return new(PacketPSFBPli)
}

func (p *PacketPSFBPli) ParsePacketPSFB(packet PacketPSFB) error {
	// load packet
	p.PacketPSFB = packet
	// setup offset
	p.size = packet.GetOffset()
	return nil
}

func (p *PacketPSFBPli) Bytes() []byte {
	p.PacketPSFB.PacketRTCP.Header.Version = 2
	p.PacketPSFB.PacketRTCP.Header.Padding = false
	// FMT is in RC field
	p.PacketPSFB.PacketRTCP.Header.ReceptionCount = FMT_PSFB_PLI
	// PT
	p.PacketPSFB.PacketRTCP.Header.PacketType = PT_PSFB
	// length
	p.PacketPSFB.PacketRTCP.Header.Length = 2
	return p.PacketPSFB.Bytes()
}

func (p *PacketPSFBPli) String() string {
	return fmt.Sprintf(
		"[RTCP-PSFB-PLI %s]",
		p.PacketPSFB.String(),
	)
}
