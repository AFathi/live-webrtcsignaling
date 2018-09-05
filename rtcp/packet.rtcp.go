package rtcp

import (
	"errors"
	"fmt"
)

/*
	PacketRTCP
  It's an abstract minimalist common ground for all RTCP packets.

			 0                   1                   2                   3
			 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
			 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
header |V=2|P|    RC   |   PT=SR=200   |             length            |
			 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
			 :                                                               :
			 :                           data                                :
			 :                                                               :
*/
type PacketRTCP struct {
	Packet
	Header Header
}

func NewPacketRTCP() *PacketRTCP {
	return new(PacketRTCP)
}

func (p *PacketRTCP) Parse(packet *Packet) error {
	p.Packet = *packet
	// header analysis
	err := p.Header.Parse(p.GetData())
	if err != nil {
		return err
	}
	// loading data
	if p.Header.GetFullPacketSize() > p.GetSize() {
		return errors.New(fmt.Sprintf("packet length %d > data %d", p.Header.GetFullPacketSize(), len(p.GetData())))
	}
	p.Slice(0, p.Header.GetFullPacketSize())
	return nil
}

func (p *PacketRTCP) GetOffset() int {
	return p.Header.GetSize()
}

func (p *PacketRTCP) IsRTCP() bool {
	return p.Header.IsRTCP()
}

func (p *PacketRTCP) GetSize() int {
	return p.Header.GetFullPacketSize()
}

func (p *PacketRTCP) Bytes() []byte {
	return p.Header.Bytes()
}

func (p *PacketRTCP) String() string {
	return p.Header.String()
}
