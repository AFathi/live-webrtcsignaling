package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
@see https://tools.ietf.org/html/rfc5104#section-4.3.1.1

0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                              SSRC                             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| Seq nr.       |    Reserved                                   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

		 Figure 4 - Syntax of an FCI Entry in the FIR Message
*/
type PSFBFir struct {
	// public
	Ssrc      uint32
	SeqNumber uint8
	// private
	size int
}

func NewPSFBFir() *PSFBFir {
	return new(PSFBFir)
}

func (o *PSFBFir) Parse(data []byte) error {
	if len(data) < 8 {
		return errors.New("packetPSFBFir size")
	}
	o.Ssrc = binary.BigEndian.Uint32(data[0:4])
	o.SeqNumber = uint8(data[5])
	o.size = 8
	return nil
}

// return the byte size of the PSFBFir
func (o *PSFBFir) GetSize() int {
	return o.size
}

func (o *PSFBFir) String() string {
	return fmt.Sprintf(
		"Fir(Ssrc=%d SqN=%d)",
		o.Ssrc,
		o.SeqNumber,
	)
}

type PSFBFirs []PSFBFir

func (l *PSFBFirs) String() string {
	var firs []string

	for _, fir := range *l {
		firs = append(firs, fir.String())
	}
	return "firs=[" + strings.Join(firs, ", ") + "]"
}
