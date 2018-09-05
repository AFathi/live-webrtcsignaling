package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
)

const (
	_ = iota
	RTCP_REMB_ALGORITHM_SIMPLE
	RTCP_REMB_ALGORITHM_MATRIX
)

type RtcpContextRemb struct {
	Remb uint32
	Date time.Time
}

func (s *RtcpContextRemb) String() string {
	return fmt.Sprintf("{remb=%d,d=%v}", s.Remb, s.Date)
}

type RtcpContextRembs struct {
	ChInfos           chan interface{}
	data              *CircularFIFO // RtcpContextRemb
	dataAvg           *CircularFIFO // float64
	bitrate           int
	ChStopRembMonitor chan struct{}
}

func NewRtcpContextRembs(ctx context.Context, ChInfos chan interface{}, algo int) *RtcpContextRembs {
	r := new(RtcpContextRembs)
	r.ChInfos = ChInfos
	r.data = NewCircularFIFO(config.Rtcp.RembHistory)
	r.dataAvg = NewCircularFIFO(30)
	switch algo {
	case RTCP_REMB_ALGORITHM_MATRIX:
		go r.StartRembMonitor_AlgorithmMatrix(ctx)
	default:
		go r.StartRembMonitor_AlgorithmSimple(ctx)
	}
	return r
}

func (r *RtcpContextRembs) Push(packet *rtcp.PacketALFBRemb) {
	remb := new(RtcpContextRemb)
	remb.Date = time.Now()
	remb.Remb = packet.GetBitrate()
	r.data.PushBack(remb)
}

/*
 * Algorithm
 * - every second, update bitrate using remb.
 */
func (c *RtcpContextRembs) StartRembMonitor_AlgorithmSimple(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	c.ChStopRembMonitor = make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if c.data.size > 0 {
					c.changeBitrate(ctx, c.data.GetLast().(*RtcpContextRemb).Remb)
				}
			case <-c.ChStopRembMonitor:
				ticker.Stop()
				return
			}
		}
	}()
}

/*
 * Algorithm:
 * - every second, compute an average of last 3sec bitrates, stack into an avg queue
 * - if no bitrate found => stack in avg the lowest bitrate allowed
 * - let avg queue be : [avg1, avg2, ... avg30]
 *   we compute a matrix queue [ m1, m2, ... , m29]
 *   with mN = 1 if avgN+1>avgN, -1 if avgN+1<avgN or 0.
 *   we sum the matrix queue to obtain a matrix result.
 *
 * if the matrix result is positive and last avg is bigger than current bitrate
 *   we go up slowly
 * if the matrix result is positive and last avg is lower than current bitrate
 *   we go down to last avg
 * if the matrix result is negative and last avg is bigger than current bitrate
 *   we go down
 * if the matrix result is negative and last avg is lower than current bitrate
 *   we go down to last avg
 */
func (c *RtcpContextRembs) StartRembMonitor_AlgorithmMatrix(ctx context.Context) {
	log, _ := plogger.FromContext(ctx)
	log.Debugf("[REMB-MONITOR] START")
	ticker := time.NewTicker(1 * time.Second)
	c.ChStopRembMonitor = make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				if c.data.size > 0 {
					log.Debugf("[REMB-MONITOR] Loop BEGIN")
					avg := c.ComputeAvgFromLastRembs(3)
					log.Debugf("[REMB-MONITOR] pushing back avg=%f", avg)
					c.dataAvg.PushBack(avg)
					matrixResult := c.ComputeMatrixResult()
					log.Debugf("[REMB-MONITOR] matrix result=%d", matrixResult)
					switch {
					case matrixResult > 0 && int(avg) > c.bitrate+(config.Bitrates.Video.Step*3):
						// increase by 1/3
						c.changeBitrate(ctx, uint32(float64(c.bitrate)+(avg-float64(c.bitrate))/3.))
					case matrixResult > 0 && int(avg) > c.bitrate+config.Bitrates.Video.Step:
						c.changeBitrate(ctx, uint32(float64(c.bitrate+config.Bitrates.Video.Step)))
					case matrixResult > 0 && int(avg) < c.bitrate:
						c.changeBitrate(ctx, uint32(avg))
					case matrixResult < 0 && int(avg) >= c.bitrate-config.Bitrates.Video.Step:
						c.changeBitrate(ctx, uint32(float64(c.bitrate-config.Bitrates.Video.Step)))
					case matrixResult < 0 && int(avg) < c.bitrate-config.Bitrates.Video.Step:
						c.changeBitrate(ctx, uint32(avg))
					}
					log.Debugf("[REMB-MONITOR] Loop END")
				}
			case <-c.ChStopRembMonitor:
				log.Debugf("[REMB-MONITOR] STOP")
				ticker.Stop()
				return
			}
		}
	}()
}

func (c *RtcpContextRembs) changeBitrate(ctx context.Context, bitrate uint32) {
	log := plogger.FromContextSafe(ctx)
	// stepped bitrate
	newBitrate := int(float64(bitrate)/float64(config.Bitrates.Video.Step)) * config.Bitrates.Video.Step
	if newBitrate > config.Bitrates.Video.Max {
		//logger.Warnf("bitrate shouldn't exceed %d, b=%d (%f)", config.Bitrates.Video.Max, newBitrate, bitrate)
		newBitrate = config.Bitrates.Video.Max
	} else if newBitrate < config.Bitrates.Video.Min {
		//logger.Warnf("bitrate shouldn't be lower than %d, b=%d (%f)", config.Bitrates.Video.Min, newBitrate, bitrate)
		newBitrate = config.Bitrates.Video.Min
	}
	if c.bitrate != newBitrate {
		c.bitrate = newBitrate
		remb := &RtcpContextInfoRemb{
			Remb: c.bitrate,
			Date: time.Now(),
		}
		select {
		case c.ChInfos <- remb:
		default:
			log.Warnf("c.ChInfos is full, dropping REMB packet")
		}
	}

}

/*
 * ComputeAvgFromLastRembs computes an average remb value from
 *   last rembs values
 */
func (c *RtcpContextRembs) ComputeAvgFromLastRembs(sec int) (avg float64) {
	var rembs []uint32

	from := time.Now().Add(-1 * time.Duration(sec) * time.Second)
	c.data.Do(func(remb interface{}) {
		r := remb.(*RtcpContextRemb)
		if r.Date.Before(from) {
			rembs = append(rembs, r.Remb)
		}
	})

	if len(rembs) == 0 {
		avg = 0
	} else {
		for i := 0; i < len(rembs); i++ {
			avg += float64(rembs[i])
		}
		avg = avg / float64(len(rembs))
	}
	return
}

func (c *RtcpContextRembs) ComputeMatrixResult() int {
	var first bool = true
	var prev float64
	var matrixResult = 0

	c.dataAvg.Do(func(i interface{}) {
		avg := i.(float64)
		if first {
			first = false
			prev = avg
		} else {
			if prev < avg {
				matrixResult++
			} else if prev > avg {
				matrixResult--
			}
			prev = avg
		}
	})
	return matrixResult
}

func (c *RtcpContextRembs) StopRembMonitor() {
	c.ChStopRembMonitor <- struct{}{}
	close(c.ChStopRembMonitor)
}

func (r *RtcpContextRembs) String() string {
	var infos []string

	r.data.Do(func(i interface{}) {
		infoRemb := i.(*RtcpContextRemb)
		infos = append(infos, infoRemb.String())
	})
	return "Rembs=[" + strings.Join(infos, ", ") + "]"
}

func (c *RtcpContextRembs) Destroy() {
	c.StopRembMonitor()
}
