package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
ReportBlock: @see https://tools.ietf.org/html/rfc3550#section-6.4.1
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
|                 SSRC_1 (SSRC of first source)                 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
| fraction lost |       cumulative number of packets lost       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|           extended highest sequence number received           |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      interarrival jitter                      |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         last SR (LSR)                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                   delay since last SR (DLSR)                  |
+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
*/
type ReportBlock struct {
	SSRC         uint32
	FractionLost uint8
	TotalLost    uint32
	HighestSeq   uint32
	Jitter       uint32
	LSR          uint32
	DLSR         uint32
	// private
	size int
}

func NewReportBlock() *ReportBlock {
	r := new(ReportBlock)
	// fixed size
	r.size = 24
	return r
}

func (r *ReportBlock) Parse(data []byte) error {
	if len(data) < 24 {
		return errors.New("report block size")
	}
	//
	r.SSRC = binary.BigEndian.Uint32(data[0:4])
	r.FractionLost = data[4]
	r.TotalLost = binary.BigEndian.Uint32([]byte{0x00, data[5], data[6], data[7]})
	r.HighestSeq = binary.BigEndian.Uint32(data[8:12])
	r.Jitter = binary.BigEndian.Uint32(data[12:16])
	r.LSR = binary.BigEndian.Uint32(data[16:20])
	r.DLSR = binary.BigEndian.Uint32(data[20:24])
	return nil
}

func (r *ReportBlock) GetSize() int {
	return r.size
}

func (r *ReportBlock) Bytes() []byte {
	var result []byte

	totalLostBytes := uint32ToBytes(r.TotalLost)
	result = append(result, uint32ToBytes(r.SSRC)...)
	result = append(result,
		byte(r.FractionLost),
		totalLostBytes[1],
		totalLostBytes[2],
		totalLostBytes[3],
	)
	result = append(result, uint32ToBytes(r.HighestSeq)...)
	result = append(result, uint32ToBytes(r.Jitter)...)
	result = append(result, uint32ToBytes(r.LSR)...)
	result = append(result, uint32ToBytes(r.DLSR)...)
	return result
}

func (r *ReportBlock) String() string {
	return fmt.Sprintf(
		"RB(ssrc=%d fl=%d tl=%d hs=%d jit=%d lsr=%d dlsr=%d)",
		r.SSRC,
		r.FractionLost,
		r.TotalLost,
		r.HighestSeq,
		r.Jitter,
		r.LSR,
		r.DLSR,
	)
}

type ReportBlocks []ReportBlock

func (l *ReportBlocks) Bytes() []byte {
	var result []byte

	for i := 0; i < len(*l); i++ {
		result = append(result, (*l)[i].Bytes()...)
	}
	return result
}

func (l *ReportBlocks) String() string {
	var rbs []string

	for _, rb := range *l {
		rbs = append(rbs, rb.String())
	}
	return "RBS=[" + strings.Join(rbs, ", ") + "]"
}
