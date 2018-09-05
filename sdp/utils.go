package sdp

import "time"

// code from https://github.com/ernado/sdp
const (
	// ntpDelta is seconds from Jan 1, 1900 to Jan 1, 1970.
	ntpDelta = 2208988800
)

// TimeToNTP converts time.Time to NTP timestamp with special case for Zero
// time, that is interpreted as 0 timestamp.
func TimeToNTP(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return uint64(t.Unix()) + ntpDelta
}

// NTPToTime converts NTP timestamp to time.Time with special case for Zero
// time, that is interpreted as 0 timestamp.
func NTPToTime(v uint64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(int64(v-ntpDelta), 0)
}
