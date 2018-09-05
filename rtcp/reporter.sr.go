package rtcp

import (
	"context"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

/*
 * FIXME: use one way channels ?
 */
type ReporterSR struct {
	InRTP chan *srtp.PacketRTP
	Out   chan *PacketSR
}

func NewReporterSR() *ReporterSR {
	rsr := new(ReporterSR)
	rsr.InRTP = make(chan *srtp.PacketRTP, 1000)
	rsr.Out = make(chan *PacketSR, 1000)
	return rsr
}

func (rsr *ReporterSR) Run(ctx context.Context, ssrcId uint32, rate uint32) {
	log := plogger.FromContextSafe(ctx).Prefix("ReporterSR").Tag("ReporterSR")

	/*
		   @see https://tools.ietf.org/html/rfc3550#section-6.4.1
		   we need to set :
		       +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
		sender |              NTP timestamp, most significant word             |
		info   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		       |             NTP timestamp, least significant word             |
		       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		       |                         RTP timestamp                         |
		       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		       |                     sender's packet count                     |
		       +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		       |                      sender's octet count                     |
		       +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
	*/
	var lastRtpPacketTimestamp uint32
	var lastRtpPacketSendTime time.Time

	var totalRtpPacketCount uint32
	var totalRtpPacketOctetCount uint32

	ticker := time.NewTicker(500 * time.Millisecond)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Infof("stop")
			return
		case packet := <-rsr.InRTP:
			if packet.GetSSRCid() == ssrcId {
				lastRtpPacketTimestamp = packet.GetTimestamp()
				lastRtpPacketSendTime = time.Now()
				totalRtpPacketCount++
				totalRtpPacketOctetCount = totalRtpPacketOctetCount + packet.GetPayloadSize()
			}
		case <-ticker.C:
			log.Debugf("ticker loop start %d", time.Now().UnixNano())
			if lastRtpPacketTimestamp == 0 {
				log.Infof("no packets received from the encoder, skipping SR reports")
				break
			}
			/*
							 * Indicates the wallclock time (see Section 4) when this report was
				       * sent so that it may be used in combination with timestamps
				       * returned in reception reports from other receivers to measure
				       * round-trip propagation to those receivers
			*/
			t := time.Now()
			sec, frac := toNtpTime(t)
			packetSR := NewPacketSR()
			packetSR.SSRC = ssrcId
			packetSR.SenderInfos.NTPSec = sec
			packetSR.SenderInfos.NTPFrac = frac
			// fetch lastRtpPacketTimestamp
			packetSR.SenderInfos.RTPTimestamp = lastRtpPacketTimestamp + uint32(t.Sub(lastRtpPacketSendTime).Seconds()*float64(rate))
			packetSR.SenderInfos.PacketCount = totalRtpPacketCount
			packetSR.SenderInfos.OctetCount = totalRtpPacketOctetCount
			// force headers before dumping packet in logs..
			packetSR.ComputeHeaders()
			// push rtcp packet to output
			select {
			case rsr.Out <- packetSR:
				log.Prefix("OUT").Infof(packetSR.String())
			default:
				log.Warnf("reporterSR.Out is full, dropping report rtcp %s", packetSR.String())
			}
			log.Debugf("ticker loop end")
		}
	}
}
