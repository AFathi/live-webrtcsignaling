package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
@see https://tools.ietf.org/html/rfc4585#section-6.3.2.2

0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|            First        |        Number           | PictureID |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

				Figure 6: Syntax of the Slice Loss Indication (SLI)
*/
type PSFBSli struct {
	// public
	FirstLostMacroblock  uint16
	NumberLostMacroblock uint16
	PictureId            uint8
	// private
	size int
}

func NewPSFBSli() *PSFBSli {
	return new(PSFBSli)
}

func (n *PSFBSli) Parse(data []byte) error {
	if len(data) < 4 {
		return errors.New("packetPSFBSli size")
	}
	n.FirstLostMacroblock = binary.BigEndian.Uint16(data[0:2]) >> 3
	n.NumberLostMacroblock = uint16((binary.BigEndian.Uint32(data[0:4]) >> 6) & 0x00001FFF)
	n.PictureId = uint8(data[3] & 0x3F)
	n.size = 4
	return nil
}

// return the byte size of the PSFBSli
func (c *PSFBSli) GetSize() int {
	return c.size
}

func (c *PSFBSli) String() string {
	return fmt.Sprintf(
		"Sli(FLM=%d LMB=%b PId=%d)",
		c.FirstLostMacroblock,
		c.NumberLostMacroblock,
		c.PictureId,
	)
}

type PSFBSlis []PSFBSli

func (l *PSFBSlis) String() string {
	var slis []string

	for _, sli := range *l {
		slis = append(slis, sli.String())
	}
	return "Slis=[" + strings.Join(slis, ", ") + "]"
}
