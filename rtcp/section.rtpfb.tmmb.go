package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
mutualising TMMBR & TMMBN FCI structure.

@see https://tools.ietf.org/html/rfc5104#section-4.2.1.1

FCI:
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                              SSRC                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| MxTBR Exp |  MxTBR Mantissa                 |Measured Overhead|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

        Figure 2 - Syntax of an FCI Entry in the TMMBR Message

@see https://tools.ietf.org/html/rfc5104#section-4.2.2.1

0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                              SSRC                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| MxTBR Exp |  MxTBR Mantissa                 |Measured Overhead|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

		Figure 3 - Syntax of an FCI Entry in the TMMBN Message

*/
type RTPFBTmmb struct {
	// public
	SSRC          uint32
	MxTBRExp      uint8
	MxTBRMantissa uint32
	Overhead      uint16
	// private
	size int
}

func NewRTPFBTmmb() *RTPFBTmmb {
	return new(RTPFBTmmb)
}

func (n *RTPFBTmmb) Parse(data []byte) error {
	if len(data) < 8 {
		return errors.New("tmmb{r,n} size")
	}
	n.SSRC = binary.BigEndian.Uint32(data[0:4])
	n.MxTBRExp = uint8(data[5] >> 2) // 6 bit
	/*
		 0                   1                   2                   3
		 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
		+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		| MxTBR Exp |  MxTBR Mantissa                 |Measured Overhead|
		+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		 5 5 5 5 5 5 5 5 6 6 6 6 6 6 6 6 7 7 7 7 7 7 7 7 8 8 8 8 8 8 8 8
		            [-|-+-+-+-+-+-+-+-|-+-+-+-+-+-+-+-]
	*/
	n.MxTBRMantissa = binary.BigEndian.Uint32([]byte{
		0x00,
		0x01 & (data[5] >> 1),
		0xFF & ((data[5]&0x01)<<7 | (data[6] >> 1)),
		0xFF & ((data[6]&0x01)<<7 | (data[7] >> 1)),
	}) // 17 bits
	n.Overhead = binary.BigEndian.Uint16([]byte{
		0x01 & data[7],
		0xFF & data[8],
	})
	n.size = 8
	return nil
}

// return the byte size of the RTPFBTmmb
func (c *RTPFBTmmb) GetSize() int {
	return c.size
}

func (c *RTPFBTmmb) String() string {
	return fmt.Sprintf(
		"T(ssrc=%d exp=%d mant=%d ovhd=%d)",
		c.SSRC,
		c.MxTBRExp,
		c.MxTBRMantissa,
		c.Overhead,
	)
}

type RTPFBTmmbs []RTPFBTmmb

func (l *RTPFBTmmbs) String() string {
	var tmmbs []string

	for _, tmmb := range *l {
		tmmbs = append(tmmbs, tmmb.String())
	}
	return "Tmmbs=[" + strings.Join(tmmbs, ", ") + "]"
}
