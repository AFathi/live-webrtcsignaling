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

 @see https://tools.ietf.org/html/rfc5104#section-4.2

 FMT Assigned in AVPF [RFC4585]:

		1:    Generic NACK
		31:   reserved for future expansion of the identifier number space

 Assigned in this memo:

		2:    reserved (see note below)
		3:    Temporary Maximum Media Stream Bit Rate Request (TMMBR)
		4:    Temporary Maximum Media Stream Bit Rate Notification (TMMBN)

	The FCI field of the TMMBN feedback message may contain zero, one, or
 more TMMBN FCI entries.
	The length of the TMMBN message SHALL be set to 2+2*N where N is the
	   number of TMMBN FCI entries.

*/
type packetRTPFBTmmbn struct {
	PacketRTPFB
	RTPFBTmmbns RTPFBTmmbs
	// private
	size int
}

func NewPacketRTPFBTmmbn() *packetRTPFBTmmbn {
	return new(packetRTPFBTmmbn)
}

func (p *packetRTPFBTmmbn) ParsePacketRTPFB(packet PacketRTPFB) error {
	// load packet
	p.PacketRTPFB = packet
	// setup offset on FCI field
	offset := packet.GetOffset()
	//
	for offset < packet.GetOffset()+p.PacketRTPFB.Header.GetLength()*4 {
		item := NewRTPFBTmmb()
		if p.GetSize() < offset {
			return errors.New("tmmbn item size")
		}
		err := item.Parse(p.GetData()[offset:])
		if err != nil {
			return err
		}
		p.RTPFBTmmbns = append(p.RTPFBTmmbns, *item)
		offset += item.GetSize()
	}
	p.size = offset
	return nil
}

func (p *packetRTPFBTmmbn) String() string {
	return fmt.Sprintf(
		"[RTCP-RTPFB-TMMBN %s %s]",
		p.PacketRTPFB.String(),
		p.RTPFBTmmbns.String(),
	)
}
