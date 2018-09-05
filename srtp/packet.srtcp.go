package srtp

//#include "shim.h"
import "C"

import (
	"fmt"
)

/*
	SRTCP https://tools.ietf.org/html/rfc3711#section-3.4
	 0                   1                   2                   3
	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|V=2|P|    RC   |   PT=SR or RR |               length          |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-^-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|                         SSRC of sender                        |
	+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=|=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
																	`---- assuming only 8 bytes for PT.

	/!\/!\/!\     /!\/!\/!\     /!\/!\/!\
	 W T F : https://tools.ietf.org/html/rfc3711#section-3.4
	  specify : PT = 9 bytes !? on the ASCII scheme...
	but
		https://tools.ietf.org/html/rfc3550#section-6.4.1
		https://tools.ietf.org/html/rfc3550#section-6.4.2
		specify : PT = 8 bytes
	:gun:
  /!\/!\/!\     /!\/!\/!\     /!\/!\/!\
*/
type PacketSRTCP struct {
	IPacketUDP
}

func NewPacketSRTCP(input IPacketUDP) *PacketSRTCP {
	p := new(PacketSRTCP)
	p.IPacketUDP = input
	return p
}

func (p *PacketSRTCP) GetSSRC() string {
	return fmt.Sprintf("%X", p.GetData()[4:8])
}

func (p *PacketSRTCP) Unprotect(ctx *Srtp) (*PacketRTCP, error) {
	newSize, err := UnprotectRTCP(ctx, p.GetData())
	if err != nil {
		return nil, err
	}
	p.Slice(0, newSize)
	return NewPacketRTCP(p.IPacketUDP), nil
}
