package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

/*
SenderInfos: @see https://tools.ietf.org/html/rfc3550#section-6.4.1
+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
|              NTP timestamp, most significant word             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|             NTP timestamp, least significant word             |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         RTP timestamp                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                     sender's packet count                     |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                      sender's octet count                     |
+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
*/
type SenderInfos struct {
	NTPSec       uint32
	NTPFrac      uint32
	RTPTimestamp uint32
	PacketCount  uint32
	OctetCount   uint32
	// private
	size int
}

func NewSenderInfos() *SenderInfos {
	return new(SenderInfos)
}

func (s *SenderInfos) Parse(data []byte) error {
	if len(data) < 20 {
		return errors.New("sender infos size")
	}
	s.size = 20
	//
	s.NTPSec = binary.BigEndian.Uint32(data[0:4])
	s.NTPFrac = binary.BigEndian.Uint32(data[4:8])
	s.RTPTimestamp = binary.BigEndian.Uint32(data[8:12])
	s.PacketCount = binary.BigEndian.Uint32(data[12:16])
	s.OctetCount = binary.BigEndian.Uint32(data[16:20])
	return nil
}

// return the byte size of sender infos section
func (s *SenderInfos) GetSize() int {
	return s.size
}

/*
 The middle 32 bits out of 64 in the NTP timestamp
 @see https://tools.ietf.org/html/rfc3550#section-6.4.2

 also, @see https://tools.ietf.org/html/rfc3550#section-4
Wallclock time (absolute date and time) is represented using the
 timestamp format of the Network Time Protocol (NTP), which is in
 seconds relative to 0h UTC on 1 January 1900 [4].  The full
 resolution NTP timestamp is a 64-bit unsigned fixed-point number with
 the integer part in the first 32 bits and the fractional part in the
 last 32 bits.  In some fields where a more compact representation is
 appropriate, only the middle 32 bits are used; that is, the low 16
 bits of the integer part and the high 16 bits of the fractional part.
 The high 16 bits of the integer part must be determined
 independently.
*/
func (s *SenderInfos) GetTimestampMiddle32bits() uint32 {
	var result []byte

	result = append(result, uint32ToBytes(s.NTPSec)[2:4]...)
	result = append(result, uint32ToBytes(s.NTPFrac)[0:2]...)
	return binary.BigEndian.Uint32(result)
}

func (s *SenderInfos) Bytes() []byte {
	var result []byte

	result = append(result, uint32ToBytes(s.NTPSec)...)
	result = append(result, uint32ToBytes(s.NTPFrac)...)
	result = append(result, uint32ToBytes(s.RTPTimestamp)...)
	result = append(result, uint32ToBytes(s.PacketCount)...)
	result = append(result, uint32ToBytes(s.OctetCount)...)
	return result
}

func (s *SenderInfos) String() string {
	return fmt.Sprintf(
		"SI(ntps=%d ntpf=%d rtpt=%d pc=%d oc=%d)",
		s.NTPSec,
		s.NTPFrac,
		s.RTPTimestamp,
		s.PacketCount,
		s.OctetCount,
	)
}
