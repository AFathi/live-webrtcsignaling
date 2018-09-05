package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
@see https://tools.ietf.org/html/rfc5104#section-4.3.2.1

0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                              SSRC                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Seq nr.      |  Reserved                           | Index   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

		 Figure 5 - Syntax of an FCI Entry in the TSTR Message
*/
type PSFBTstr struct {
	// public
	Ssrc      uint32
	SeqNumber uint8
	// private
	size int
}

func NewPSFBTstr() *PSFBTstr {
	return new(PSFBTstr)
}

func (o *PSFBTstr) Parse(data []byte) error {
	if len(data) < 8 {
		return errors.New("packetPSFBTstr size")
	}
	o.Ssrc = binary.BigEndian.Uint32(data[0:4])
	o.SeqNumber = uint8(data[5])
	o.size = 8
	return nil
}

// return the byte size of the PSFBTstr
func (o *PSFBTstr) GetSize() int {
	return o.size
}

func (o *PSFBTstr) String() string {
	return fmt.Sprintf(
		"Tstr(Ssrc=%d SqN=%d)",
		o.Ssrc,
		o.SeqNumber,
	)
}

type PSFBTstrs []PSFBTstr

func (l *PSFBTstrs) String() string {
	var tstrs []string

	for _, tstr := range *l {
		tstrs = append(tstrs, tstr.String())
	}
	return "Tstrs=[" + strings.Join(tstrs, ", ") + "]"
}
