package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

/*
 @see https://tools.ietf.org/html/rfc3550#section-6.4.1

	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |V=2|P|    RC   |   PT=SR=200   |             length            |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

version (V): 2 bits
	 Identifies the version of RTP, which is the same in RTCP packets
	 as in RTP data packets.  The version defined by this specification
	 is two (2).

 padding (P): 1 bit
   If the padding bit is set, this individual RTCP packet contains
   some additional padding octets at the end which are not part of
   the control information but are included in the length field.  The
   last octet of the padding is a count of how many padding octets
   should be ignored, including itself (it will be a multiple of
   four).  Padding may be needed by some encryption algorithms with
   fixed block sizes.  In a compound RTCP packet, padding is only
   required on one individual packet because the compound packet is
   encrypted as a whole for the method in Section 9.1.  Thus, padding
   MUST only be added to the last individual packet, and if padding
   is added to that packet, the padding bit MUST be set only on that
   packet.  This convention aids the header validity checks described
   in Appendix A.2 and allows detection of packets from some early
   implementations that incorrectly set the padding bit on the first
   individual packet and add padding to the last individual packet.

	reception report count (RC): 5 bits
		 The number of reception report blocks contained in this packet.  A
		 value of zero is valid.

	packet type (PT): 8 bits
		 Contains the constant 200 to identify this as an RTCP SR packet.

	length: 16 bits
		 The length of this RTCP packet in 32-bit words minus one,
		 including the header and any padding.  (The offset of one makes
		 zero a valid length and avoids a possible infinite loop in
		 scanning a compound RTCP packet, while counting 32-bit words
		 avoids a validity check for a multiple of 4.)
*/
type Header struct {
	// public
	Version        uint8
	Padding        bool
	ReceptionCount uint8
	PacketType     uint8
	Length         uint16 // length RTCP packet in 32-bit words minus one
	// private
	data [4]byte
	size int
}

func NewHeader() *Header {
	return new(Header)
}

func (h *Header) Parse(data []byte) error {
	if len(data) < 4 {
		return errors.New("header size")
	}
	h.data = [4]byte{data[0], data[1], data[2], data[3]}
	h.size = 4
	// pre-compute
	h.Version = uint8(data[0] & 0xC0 /*1100 0000*/ >> 6)
	h.Padding = bool((data[0] & 0x20 /*0010 0000*/ >> 5) != 0)
	h.ReceptionCount = uint8(data[0] & 0x1F /*0001 1111*/)
	h.PacketType = uint8(data[1])
	h.Length = binary.BigEndian.Uint16([]byte{data[2], data[3]})
	return nil
}

func (h *Header) GetLength() int {
	return int(h.Length)
}

// return the byte size of the header
func (h *Header) GetSize() int {
	return h.size
}

// return the size in bytes, including the header.
func (h *Header) GetFullPacketSize() int {
	return h.GetLength()*4 + 4
}

/*
  @see https://tools.ietf.org/html/draft-ietf-avtcore-rfc5764-mux-fixes-11
	+----------------+
	|        [0..3] -+--> forward to STUN
	|                |
	|      [16..19] -+--> forward to ZRTP
	|                |
  |      [20..63] -+--> forward to DTLS
	|                |
	|      [64..79] -+--> forward to TURN Channel
	|                |
	|    [128..191] -+--> forward to RTP/RTCP
	+----------------+
*/
func (h *Header) IsRTCP() bool {
	return h.Version == 2 &&
		// [128..191] -+--> forward to RTP/RTCP
		h.data[0] >= 128 && h.data[0] <= 191 &&
		// https://www.iana.org/assignments/rtp-parameters/rtp-parameters.xhtml#rtp-parameters-4
		h.PacketType >= 192 && h.PacketType <= 213
}

func (h *Header) Bytes() []byte {
	bLength := uint16ToBytes(h.Length)
	return []byte{
		((0xC0 /*1100 0000*/ & (h.Version << 6)) |
			(0x20 /*0010 0000*/ & (boolToUint8(h.Padding) << 5)) |
			(0x1F /*0001 1111*/ & h.ReceptionCount)),
		h.PacketType,
		bLength[0],
		bLength[1],
	}
}

func (h *Header) String() string {
	return fmt.Sprintf(
		"H(v=%d p=%t rc=%d pt=%d l=%d pksz=%d)",
		h.Version,
		h.Padding,
		h.ReceptionCount,
		h.PacketType,
		h.Length,
		h.GetFullPacketSize(),
	)
}
