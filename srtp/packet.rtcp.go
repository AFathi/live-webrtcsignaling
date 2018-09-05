package srtp

import (
	"encoding/binary"
	"fmt"
)

type PacketRTCP struct {
	IPacketUDP
}

func NewPacketRTCP(input IPacketUDP) *PacketRTCP {
	p := new(PacketRTCP)
	p.IPacketUDP = input
	return p
}

func (p *PacketRTCP) GetSSRCid() uint32 {
	return binary.BigEndian.Uint32(p.GetData()[4:8])
}

func (p *PacketRTCP) GetSSRC() string {
	return fmt.Sprintf("%X", p.GetData()[4:8])
}

func (p *PacketRTCP) GetPT() int {
	return int(p.GetData()[1] & 0x7F) // 0111 1111
}
