package rtcp

import (
	"context"
	"math"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

/*
 * FIXME: use one way channels ?
 */
type ReporterRR struct {
	InRTP    chan *srtp.PacketRTP
	InRTCP   chan *srtp.PacketRTCP
	Out      chan *PacketRR
	OutStats chan *ReporterRRStats
}

type ReporterRRStats struct {
	InterarrivalDifference int64
	InterarrivalJitter     uint32
	SSRC                   uint32
}

// fixme: remove dependency to plogger ...
func NewReporterRR() *ReporterRR {
	rrr := new(ReporterRR)
	rrr.InRTP = make(chan *srtp.PacketRTP, 1000)
	rrr.InRTCP = make(chan *srtp.PacketRTCP, 1000)
	rrr.Out = make(chan *PacketRR, 1000)
	rrr.OutStats = make(chan *ReporterRRStats, 1000)
	return rrr
}

/*
 * @see https://github.com/versatica/mediasoup/blob/master/worker/src/RTC/RtpStream.cpp
 * @param ctx
 * @param ssrcId ssrcId of the rtp ssrc identifier
 * @param rate is the clock rate associated with the payload associated with this ssrc.
 */
func (rrr *ReporterRR) Run(ctx context.Context, ssrcId uint32, rate uint32) {
cumulDelay := int64(0)

	log := plogger.FromContextSafe(ctx).Prefix("ReporterRR").Tag("ReporterRR")

	// using our own parser... maybe we should
	// depend on main instantiated parser to avoid double parsing.
	parser := NewParser(Dependencies{Logger: log.Prefix("IN")})

	lastPacket := struct {
		SeqNumber         uint16
		ExtendedSeqNumber uint32
		RtpTimestamp      int64
		ArrivalTimestamp  int64 // in clock cycles since 1st january 1970
	}{}

	lastFramePacket := struct {
		SeqNumber         uint16
		ExtendedSeqNumber uint32
		RtpTimestamp      int64
		ArrivalTimestamp  int64 // in clock cycles since 1st january 1970
	}{}

	currentPacket := struct {
		SeqNumber         uint16
		ExtendedSeqNumber uint32
		RtpTimestamp      int64
		ArrivalTimestamp  int64 // in clock cycles since 1st january 1970
	}{}

	firstPacket := struct {
		SeqNumber         uint16
		ExtendedSeqNumber uint32
		Received          bool
	}{}

	firstFramePacket := struct {
		Received bool
	}{}

	/*
	 * @see https://tools.ietf.org/html/rfc3550#appendix-A.3
	 * The number of packets received is simply the count of packets
	 * as they arrive, including any late or duplicate packets
	 */
	var numberOfPacketsReceived uint32

	/*
	 * @see https://tools.ietf.org/html/rfc3550#appendix-A.3
	 * The number of packets expected can be
	 * computed by the receiver as the difference between the highest
	 * sequence number received (s->max_seq) and the first sequence number
	 * received (s->base_seq)
	 */
	var numberOfPacketsExpected uint32
	var highestSeqNumber uint32 // highest

	/*
	 * @see http://www.freesoft.org/CIE/RFC/1889/53.htm
	 * The seq number is only 16 bits (65536), it wrap arounds
	 */
	var cycle uint32

	/*
	 * @see https://tools.ietf.org/html/rfc3550#appendix-A.3
	 * The number of packets lost is defined to be the number of packets
	 * expected less the number of packets actually received:
	 */
	var numberOfPacketsLosts uint32

	/*
	 * we process computation at interval
	 * we save the previous numberOfPacketsExpected
	 */
	var previousNumberOfPacketsExpected uint32
	var previousNumberOfPacketsReceived uint32
	var firstInterval bool = true

	/*
	 * interarrival jitter
	 * @see https://tools.ietf.org/html/rfc3550#section-6.4.1
	 * mean deviation (smoothed absolute value) of the difference D in packet spacing
	 *
	 * If Si is the RTP timestamp from packet i, and Ri is the time of
	 *  arrival in RTP timestamp units for packet i, then for two packets
	 *    i and j, D may be expressed as :
	 *
	 *     D(i,j) = (Rj - Ri) - (Sj - Si) = (Rj - Sj) - (Ri - Si)
	 *
	 *
	 * WARNING: it seems that multiple RTP packets can have the same RTP Timestamp !
	 *   we must compute D(i,j) & J(i) per Frame to improve bitrate computation.
	 *
	 * following https://slideblast.com/performance-analysis-of-receive-side-real-time-semantic-scholar_5940ace91723ddbf079431a8.html
	 * it's due to MTU
	 * ( The receiver estimates the overuse or underuse of the bottleneck
	 *   link based on the timestamps of incoming frames relative
	 *   to the generation timestamps. At high bit rates, the video
	 *   frames exceed the MTU size and are fragmented over multiple
	 *   RTP packets, in which case the received timestamp of the last
	 *   packet is used )
	 *
	 * And the computation should be based on a per "frame" and not "per packet"
	 *
	 * When a video frame is fragmented all the RTP
	 * packets have the same generation timestamp [3]. Formally, the
	 * jitter is calculated as follows: Ji = (ti − ti−1) − (Ti − Ti−1),
	 * where t is receive timestamp, T is the RTP timestamp, and the
	 * i and i-1 are successive frames. Typically, if Ji is positive, it
	 * corresponds to congestion. Further, [8] proposes inter-arrival
	 * jitter as a function serialization delay, queuing delay and
	 * network jitter
	 *
	 * Frame seems to be marked: http://www.networksorcery.com/enp/protocol/rtp.htm#M, Marker
	 */
	var interarrivalDifference int64
	var interarrivalJitter uint32
	var previousInterarrivalJitter uint32
	var frameInterarrivalDifference int64
	var frameInterarrivalJitter uint32
	var framePreviousInterarrivalJitter uint32

	/*
	 *
	 */
	var lastSRTimestamp uint32
	var lastSRArrival time.Time

	log.Infof("start")

	/*
	 https://chromium.googlesource.com/external/webrtc/stable/webrtc/+/master/modules/rtp_rtcp/source/rtp_rtcp_config.h
	 enum { RTCP_INTERVAL_VIDEO_MS       = 1000 };
	 enum { RTCP_INTERVAL_AUDIO_MS       = 5000 };
	*/
	ticker := time.NewTicker(300 * time.Millisecond)
	defer func() {
		ticker.Stop()
	}()

	var cumulative []int64

	for {
		select {
		case <-ctx.Done():
			log.Infof("stop")
			return
		case compoundRTCP := <-rrr.InRTCP:
			// input RTCP
			packets, err := parser.Parse(compoundRTCP)
			if err != nil {
				log.Errorf(err.Error())
			} else {
				for i := 0; i < len(packets); i++ {
					switch packet := packets[i].(type) {
					case *PacketSR:
						if packet.SSRC == ssrcId {
							log.Debugf("SR ssrc=%d RECEIVED, lastSRTimestamp = %d", packet.SSRC, packet.SenderInfos.GetTimestampMiddle32bits())
							lastSRTimestamp = packet.SenderInfos.GetTimestampMiddle32bits()
							lastSRArrival = time.Now()
						} else {
							log.Debugf("SR ssrc=%d RECEIVED, mismatch %d => skip", packet.SSRC, ssrcId)
						}
					}
				}
			}
		case packet := <-rrr.InRTP:
			// WE NEED TO COMPUTE D() & J() on every packet for RR Reports
			// we can also compute D() & J() on every Frame for bitrate evaluation
			currentPacket.SeqNumber = packet.GetSeqNumber()
			currentPacket.RtpTimestamp = int64(packet.GetTimestamp())
			currentPacket.ArrivalTimestamp = int64(float64(packet.GetCreatedAt().UnixNano()) / float64(1000000000) * float64(rate))

			if firstPacket.Received == false {
				currentPacket.ExtendedSeqNumber = uint32(currentPacket.SeqNumber)
				firstPacket.Received = true
				firstPacket.SeqNumber = currentPacket.SeqNumber
				firstPacket.ExtendedSeqNumber = currentPacket.ExtendedSeqNumber
			} else {
				// input should be ordered.
				if lastPacket.SeqNumber > currentPacket.SeqNumber {
					// wrapping around
					cycle++
				}
				currentPacket.ExtendedSeqNumber = cycle*65536 + uint32(currentPacket.SeqNumber)

				// empiric filter: abnormal rtp timestamp values => break.
				if AbsInt64(currentPacket.ArrivalTimestamp-lastPacket.ArrivalTimestamp) > 1000000 {
					log.Warnf("corrupted arrivalTimestamp %d (previousArrivalTimestamp %d)", currentPacket.ArrivalTimestamp, lastPacket.ArrivalTimestamp)
					// break // <=> packet never arrived, will be added to packet loss.
				}

				// Interarrival Difference & Jitter between packets.
				// computing J(i) = J(i-1) + (|D(i-1,i)| - J(i-1))/16
				//  knowing that D(i-1,i) = (Ri - Si) - (Ri-1 - Si-1)
				//  Ri=time of arrival of current packet
				//  Ji=timestamp of current packet
				interarrivalDifference = (currentPacket.ArrivalTimestamp - currentPacket.RtpTimestamp) -
					(lastPacket.ArrivalTimestamp - lastPacket.RtpTimestamp)
					cumulDelay += interarrivalDifference
					log.Infof("REPORT RR cumulDelay is %d", cumulDelay)
				interarrivalJitter = uint32(float64(previousInterarrivalJitter) +
					(math.Abs(float64(interarrivalDifference))-
						float64(previousInterarrivalJitter))/16.)

				log.Debugf("PacketInterDiff [Seq=%d,PT=%d] D(%d,%d)=(%d - %d) - (%d - %d)=%d J(%d)=%d+%f=%d",
					currentPacket.SeqNumber, packet.GetPT(),
					lastPacket.ExtendedSeqNumber, currentPacket.ExtendedSeqNumber,
					currentPacket.ArrivalTimestamp, currentPacket.RtpTimestamp,
					lastPacket.ArrivalTimestamp, lastPacket.RtpTimestamp,
					interarrivalDifference,
					currentPacket.ExtendedSeqNumber,
					previousInterarrivalJitter, (math.Abs(float64(interarrivalDifference))-float64(previousInterarrivalJitter))/16.,
					interarrivalJitter,
				)

				if packet.GetMarkerBit() == true {
					if firstFramePacket.Received == false {
						firstFramePacket.Received = true
					} else {
						// Interarrival Difference & Jitter between frames.
						frameInterarrivalDifference = (currentPacket.ArrivalTimestamp - currentPacket.RtpTimestamp) -
							(lastFramePacket.ArrivalTimestamp - lastFramePacket.RtpTimestamp)
						frameInterarrivalJitter = uint32(float64(framePreviousInterarrivalJitter) +
							(math.Abs(float64(frameInterarrivalDifference))-
								float64(framePreviousInterarrivalJitter))/16.)
						stats := new(ReporterRRStats)
						stats.InterarrivalDifference = frameInterarrivalDifference
						stats.InterarrivalJitter = frameInterarrivalJitter
						stats.SSRC = ssrcId
						select {
						case rrr.OutStats <- stats:
						default:
							log.Prefix("OUTStats").Warnf("reporterRR.OutStats is full, dropping stat report %v", stats)
						}

						log.Debugf("FrameInterDiff [Seq=%d,PT=%d] D(%d,%d)=(%d - %d) - (%d - %d)=%d J(%d)=%d+%f=%d",
							currentPacket.SeqNumber, packet.GetPT(),
							lastFramePacket.SeqNumber, currentPacket.ExtendedSeqNumber,
							currentPacket.ArrivalTimestamp, currentPacket.RtpTimestamp,
							lastFramePacket.ArrivalTimestamp, lastFramePacket.RtpTimestamp,
							frameInterarrivalDifference,
							currentPacket.ExtendedSeqNumber,
							framePreviousInterarrivalJitter, (math.Abs(float64(frameInterarrivalDifference))-float64(framePreviousInterarrivalJitter))/16.,
							frameInterarrivalJitter,
						)

						cumulative = append(cumulative, frameInterarrivalDifference)
						if len(cumulative) > 50 {
							cumulative = cumulative[1:51]
							var c int64
							for i := 0; i < 50; i++ {
								c += cumulative[i]
							}
							log.Debugf("FrameInterDiff cumulative 50 values = %d cumulative = %v", c, cumulative)
						}
						framePreviousInterarrivalJitter = frameInterarrivalJitter
					}
					lastFramePacket.ArrivalTimestamp = currentPacket.ArrivalTimestamp
					lastFramePacket.RtpTimestamp = currentPacket.RtpTimestamp
					lastFramePacket.SeqNumber = currentPacket.SeqNumber
					lastFramePacket.ExtendedSeqNumber = currentPacket.ExtendedSeqNumber
				}
			}
			numberOfPacketsReceived++
			if currentPacket.ExtendedSeqNumber > highestSeqNumber {
				highestSeqNumber = currentPacket.ExtendedSeqNumber
			}
			numberOfPacketsExpected = highestSeqNumber - uint32(firstPacket.SeqNumber) + 1
			numberOfPacketsLosts = numberOfPacketsExpected - numberOfPacketsReceived
			//
			lastPacket.ArrivalTimestamp = currentPacket.ArrivalTimestamp
			lastPacket.RtpTimestamp = currentPacket.RtpTimestamp
			lastPacket.SeqNumber = currentPacket.SeqNumber
			lastPacket.ExtendedSeqNumber = currentPacket.ExtendedSeqNumber
			previousInterarrivalJitter = interarrivalJitter
		case <-ticker.C:
			// create & send compound RTCP packet
			if numberOfPacketsReceived == 0 {
				break
			}
			if firstInterval {
				log.Debugf("ticker loop: first => skip")
				firstInterval = false
			} else {
				log.Debugf("ticker loop start")
				/*
				 * @see https://tools.ietf.org/html/rfc3550#appendix-A.3
				 */
				expectedInterval := numberOfPacketsExpected - previousNumberOfPacketsExpected
				receivedInterval := numberOfPacketsReceived - previousNumberOfPacketsReceived
				lostInterval := expectedInterval - receivedInterval
				var fractionLost uint8

				if expectedInterval != 0 && lostInterval > 0 {
					fractionLost = uint8(float64(lostInterval<<8) / float64(expectedInterval))
				}

				packetRR := NewPacketRR()
				packetRRBlock := NewReportBlock()
				packetRRBlock.SSRC = ssrcId
				packetRRBlock.FractionLost = fractionLost
				packetRRBlock.TotalLost = numberOfPacketsLosts
				packetRRBlock.HighestSeq = highestSeqNumber
				packetRRBlock.Jitter = uint32(interarrivalJitter) // sampling jitter
				packetRRBlock.LSR = lastSRTimestamp
				/*
					The delay, expressed in units of 1/65536 seconds, between
					 receiving the last SR packet from source SSRC_n and sending this
					 reception report block.  If no SR packet has been received yet
					 from SSRC_n, the DLSR field is set to zero.
				*/
				if lastSRTimestamp != 0 {
					packetRRBlock.DLSR = uint32(time.Now().Sub(lastSRArrival).Seconds() * 65536)
				}
				packetRR.ReportBlocks = append(packetRR.ReportBlocks, *packetRRBlock)
				// (force packet.Bytes() to set header values), optionnal, allows a correct packetRR.String()
				packetRR.Bytes()
				// push rtcp packet to output
				select {
				case rrr.Out <- packetRR:
					log.Prefix("OUT").Infof(packetRR.String())
				default:
					log.Prefix("OUT").Warnf("reporterRR.Out is full, dropping report rtcp %s", packetRR.String())
				}
				log.Debugf("ticker loop end")
			}
			previousNumberOfPacketsReceived = numberOfPacketsReceived
			previousNumberOfPacketsExpected = numberOfPacketsExpected
		}
	}
}
