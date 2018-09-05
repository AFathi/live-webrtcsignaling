package rtcp_test

import (
	"fmt"
	"testing"

	"github.com/heytribe/live-webrtcsignaling/rtcp"
)

func TestREMBBitrateSetGet(t *testing.T) {
	p := rtcp.NewPacketALFBRemb()
	p.SetBitrate(512845.55)
	fmt.Printf("bitrate=[%f] [%s]\n", p.GetBitrate(), p.String())
}
