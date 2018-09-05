package srtp

//#include "shim.h"
import "C"

import (
	"fmt"
)

/*
	SRTP https://tools.ietf.org/html/rfc3711#section-3.1
	 0                   1                   2                   3
	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|V=2|P|X|  CC   |M|     PT      |       sequence number         |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|                           timestamp                           |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|           synchronization source (SSRC) identifier            |
	+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
*/
type PacketSRTP struct {
	IPacketUDP
}

func NewPacketSRTP(input IPacketUDP) *PacketSRTP {
	p := new(PacketSRTP)
	p.IPacketUDP = input
	return p
}

func (p *PacketSRTP) GetSSRC() string {
	return fmt.Sprintf("%X", p.GetData()[8:12])
}

func (p *PacketSRTP) GetPT() int {
	return int(p.GetData()[1] & 0x7F) // 0111 1111
}

func (p *PacketSRTP) Unprotect(ctx *Srtp) (*PacketRTP, error) {
	newSize, err := UnprotectRTP(ctx, p.GetData())
	if err != nil {
		return nil, err
	}
	p.Slice(0, newSize)
	return NewPacketRTP(p.IPacketUDP), nil
}

func (p *PacketSRTP) String() string {
	return fmt.Sprintf("PT(%d): %b \t %x \n", p.GetPT(), p.GetData()[0:12], p.GetData()[0:12])
}
