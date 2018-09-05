package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
FCI:
	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |            PID                |             BLP               |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

						 Figure 4: Syntax for the Generic NACK message
*/
type RTPFBNack struct {
	// public
	PacketId          uint16
	LostPacketBitmask uint16
	// private
	size int
}

func NewRTPFBNack() *RTPFBNack {
	return new(RTPFBNack)
}

func (n *RTPFBNack) Parse(data []byte) error {
	if len(data) < 4 {
		return errors.New("packetRTPFBNack size")
	}
	n.PacketId = binary.BigEndian.Uint16(data[0:2])
	n.LostPacketBitmask = binary.BigEndian.Uint16(data[2:4])
	n.size = 4
	return nil
}

// return the byte size of the RTPFBNack
func (n *RTPFBNack) GetSize() int {
	return n.size
}

// set bit i of the bit mask to 1
// you should use this func if the receiver
//   has not received RTP packet number (PID+i) (modulo 2^16)
func (n *RTPFBNack) Lost(i uint) {
	// bitshift are operated in BigEndian (CPU)
	n.LostPacketBitmask |= (1 << (i - 1))
}

func (n *RTPFBNack) Bytes() []byte {
	bPacketId := uint16ToBytes(n.PacketId)
	bLostPacketBitmask := uint16ToBytes(n.LostPacketBitmask)
	return []byte{
		bPacketId[0],
		bPacketId[1],
		bLostPacketBitmask[0],
		bLostPacketBitmask[1],
	}
}

func (c *RTPFBNack) String() string {
	return fmt.Sprintf(
		"Nk(PId=%d LPB=%b)",
		c.PacketId,
		c.LostPacketBitmask,
	)
}

func (l *RTPFBNack) GetSequences() []uint16 {
	var b uint16

	seqs := []uint16{l.PacketId}
	for b = 0; b < 16; b++ {
		mask := l.LostPacketBitmask & (0x8000 >> b)
		if mask != 0 {
			seqs = append(seqs, l.PacketId+1+b)
		}
	}

	return seqs
}

type RTPFBNacks []RTPFBNack

func (l *RTPFBNacks) Bytes() []byte {
	var result []byte

	for _, nack := range *l {
		result = append(result, nack.Bytes()...)
	}
	return result
}

func (l *RTPFBNacks) String() string {
	var nacks []string

	for _, nack := range *l {
		nacks = append(nacks, nack.String())
	}
	return "Nks=[" + strings.Join(nacks, ", ") + "]"
}
