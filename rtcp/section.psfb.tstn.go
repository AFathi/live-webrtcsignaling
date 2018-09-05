package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
@see https://tools.ietf.org/html/rfc5104#section-4.3.3.1

0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                              SSRC                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Seq nr.      |  Reserved                           | Index   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

								Figure 6 - Syntax of the TSTN
*/
type PSFBTstn struct {
	// public
	Ssrc      uint32
	SeqNumber uint8
	// private
	size int
}

func NewPSFBTstn() *PSFBTstn {
	return new(PSFBTstn)
}

func (o *PSFBTstn) Parse(data []byte) error {
	if len(data) < 8 {
		return errors.New("packetPSFBTstn size")
	}
	o.Ssrc = binary.BigEndian.Uint32(data[0:4])
	o.SeqNumber = uint8(data[5])
	o.size = 8
	return nil
}

// return the byte size of the PSFBTstn
func (o *PSFBTstn) GetSize() int {
	return o.size
}

func (o *PSFBTstn) String() string {
	return fmt.Sprintf(
		"Tstn(Ssrc=%d SqN=%d)",
		o.Ssrc,
		o.SeqNumber,
	)
}

type PSFBTstns []PSFBTstn

func (l *PSFBTstns) String() string {
	var tstns []string

	for _, tstn := range *l {
		tstns = append(tstns, tstn.String())
	}
	return "Tstns=[" + strings.Join(tstns, ", ") + "]"
}
