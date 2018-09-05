package rtcp

import (
	"encoding/binary"
	"time"
)

func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func AbsInt64(i int64) int64 {
	if i < 0 {
		return -i
	}
	return i
}

func uint16ToBytes(i uint16) []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, i)
	return bytes
}

func uint32ToBytes(i uint32) []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, i)
	return bytes
}

func uint64ToBytes(i uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, i)
	return bytes
}

// toNtpTime converts the time.Time value t
// into its 64-bit fixed-point ntpTime representation.
//
// The Network Time Protocol (NTP) is in
//  seconds relative to 0h UTC on 1 January 1900
// The full resolution NTP timestamp is a 64-bit unsigned
//  fixed-point number with the integer part in the first 32 bits and the
//  fractional part in the last 32 bits.
//
// @see https://github.com/beevik/ntp/
// @see https://tools.ietf.org/html/rfc3550#section-4
func toNtpTime(t time.Time) (uint32, uint32) {
	var nanoPerSec uint64

	ntpEpoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	nsec := uint64(t.Sub(ntpEpoch))
	nanoPerSec = 1000000000
	sec := nsec / nanoPerSec
	// Round up the fractional component so that repeated conversions
	// between time.Time and ntpTime do not yield continually decreasing
	// results.
	frac := (((nsec - sec*nanoPerSec) << 32) + nanoPerSec - 1) / nanoPerSec
	return uint32(sec & 0x00000000FFFFFFFF), uint32(frac & 0x00000000FFFFFFFF)
}
