package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
	Application layer FB messages

	@see https://tools.ietf.org/html/draft-alvestrand-rmcat-remb-00#section-2.2

	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |V=2|P| FMT=15  |   PT=206      |             length            |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                  SSRC of packet sender                        |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                  SSRC of media source                         |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |  Unique identifier 'R' 'E' 'M' 'B'                            |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |  Num SSRC     | BR Exp    |  BR Mantissa                      |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |   SSRC feedback                                               |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |  ...                                                          |

*/
type PacketALFBRemb struct {
	PacketPSFBAfb
	BRExp      uint8
	BRMantissa uint32
	SSRCs      []uint32
	// private
	size int
}

func NewPacketALFBRemb() *PacketALFBRemb {
	return new(PacketALFBRemb)
}

func (p *PacketALFBRemb) ParsePacketPSFBAfb(packet PacketPSFBAfb) error {
	// load packet
	p.PacketPSFBAfb = packet
	// setup offset at the end of PacketPSFBAfb header
	offset := p.PacketPSFBAfb.GetOffset()
	// check min packet size
	if p.GetSize() < offset+12 {
		return errors.New("remb size")
	}
	offset += 4 // skip REMB chars
	ssrcNumber := uint8(p.GetData()[offset])
	p.BRExp = uint8(p.GetData()[offset+1] >> 2)
	p.BRMantissa = binary.BigEndian.Uint32([]byte{0x00, p.GetData()[offset+1] & 0x03, p.GetData()[offset+2], p.GetData()[offset+3]})
	offset += 4 // skip NumSSRC / BR Exp / BR Mantissa
	if p.GetSize() < offset+4*int(ssrcNumber) {
		return errors.New("remb ssrcs size")
	}
	for ; ssrcNumber > 0; ssrcNumber-- {
		ssrc := binary.BigEndian.Uint32(p.GetData()[offset : offset+4])
		p.SSRCs = append(p.SSRCs, ssrc)
		offset += 4 // skip SSRC
	}
	p.size = offset
	return nil
}

/*
 * BR Exp & BR Mantissa
 *  <=> total media bit rate in number of bits per second
 *      (ignoring all packet overhead)
 *      for the SSRCs reported in this message
 *
 * > 6 bits exponent + 18 bits mantissa
 * https://tools.ietf.org/html/rfc5104#section-4.2.1.1
 * formula is : MxTBR = mantissa * 2^exp
 *
 * @return bit rate in bits/sec
 */
func (p *PacketALFBRemb) GetBitrate() uint32 {
	return p.BRMantissa << p.BRExp
}

/*
 * save bitrate (bits/sec) in BR Exp & BR Mantissa
 * > 6 bits exponent + 18 bits mantissa
 * @see https://cs.chromium.org/chromium/src/third_party/webrtc/modules/rtp_rtcp/source/rtcp_packet/remb.cc
 */
func (p *PacketALFBRemb) SetBitrate(bitrate uint32) {
	var kMaxMantissa uint32 = 0x3ffff
	var exponenta uint8 = 0

	mantissa := bitrate
	for mantissa > kMaxMantissa {
		mantissa = mantissa >> 1
		exponenta++
	}
	p.BRMantissa = mantissa
	p.BRExp = exponenta
}

func (p *PacketALFBRemb) Bytes() []byte {
	var result []byte

	p.PacketPSFBAfb.PacketPSFB.PacketRTCP.Header.Version = 2
	p.PacketPSFBAfb.PacketPSFB.PacketRTCP.Header.Padding = false
	p.PacketPSFBAfb.PacketPSFB.PacketRTCP.Header.ReceptionCount = FMT_PSFB_AFB
	p.PacketPSFBAfb.PacketPSFB.PacketRTCP.Header.PacketType = PT_PSFB
	p.PacketPSFBAfb.PacketPSFB.PacketRTCP.Header.Length = uint16(2 + 2 + len(p.SSRCs))

	result = append(result, p.PacketPSFBAfb.Bytes()...)
	result = append(result, []byte("REMB")...)
	exp := byte(p.BRExp << 2)
	mantissa := uint32ToBytes(p.BRMantissa)
	result = append(result, byte(len(p.SSRCs)))
	result = append(result, exp|(mantissa[1]&0x03))
	result = append(result, mantissa[2], mantissa[3])
	for i := 0; i < len(p.SSRCs); i++ {
		result = append(result, uint32ToBytes(p.SSRCs[i])...)
	}
	return result
}

func (p *PacketALFBRemb) String() string {
	var ssrcs []string

	for _, ssrc := range p.SSRCs {
		ssrcs = append(ssrcs, fmt.Sprintf("%d", ssrc))
	}
	return fmt.Sprintf(
		"[RTCP-ALFB-REMB %s BRExp=%d BRMnt=%d Btrt=%d SSRCS=(%s)]",
		p.PacketPSFBAfb.String(),
		p.BRExp,
		p.BRMantissa,
		p.GetBitrate(),
		strings.Join(ssrcs, ", "),
	)
}
