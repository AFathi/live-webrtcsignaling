package rtcp_test

import (
	"fmt"
	"testing"

	"github.com/heytribe/live-webrtcsignaling/rtcp"
)

func TestSDP(t *testing.T) {
	p := rtcp.NewPacketPSFBPli()
	p.PacketPSFB.SenderSSRC = 4242
	p.PacketPSFB.MediaSSRC = 4343
	bytes := p.Bytes()
	fmt.Printf("%b", bytes)
}
