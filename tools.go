package main

import (
	"fmt"
	"math"

	plogger "github.com/heytribe/go-plogger"
)

func logOnError(err error, format string, args ...interface{}) bool {
	return plogger.New().OnError(err, format, args...)
}

func panicOnError(err error, msg string) {
	if err != nil {
		s := fmt.Sprintf("%s: %s", msg, err)
		plogger.New().Fatalf(s) // FIXME
		panic(s)
	}
}

/*
func dumpPacketToString(data []byte) (s string) {
	s = ""
	for _, b := range data {
		if b >= 0x20 && b <= 0x7e {
			s += string(b)
		} else {
			s += "."
		}
	}
	return
}
*/

func Round(a float64) float64 {
	if a < 0 {
		return math.Ceil(a - 0.5)
	}
	return math.Floor(a + 0.5)
}

func uint32sToStrings(list []uint32) []string {
	var result []string
	for _, i := range list {
		result = append(result, fmt.Sprintf("%d", i))
	}
	return result
}
