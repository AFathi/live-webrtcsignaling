package main

import (
	"fmt"
	"time"
)

/*
 * RtcpContext agregate rtcp packet info
 */
type RtcpContextInfoRemb struct {
	Remb int
	Date time.Time
}

func (s *RtcpContextInfoRemb) String() string {
	return fmt.Sprintf("{btrt=%d,d=%v}", s.Remb, s.Date)
}

type RtcpContextInfoFIR struct {
	Date time.Time
}
