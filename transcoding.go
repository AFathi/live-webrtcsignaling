package main

import (
	"context"
	"net"
	"sync"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/gst"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

// WebRTC pipeline
// rtpbin name=rtpbin rtp-profile=4 \
// udpsrc port=10001 caps="application/x-rtp,media=(string)video,payload=(int)96,clock-rate=(int)90000,encoding-name=(string)VP8" ! rtpbin.recv_rtp_sink_0 \
// rtpbin. ! rtpvp8depay ! vp8dec ! x264enc ! queue ! mpegtsmux name=tsmux ! queue ! filesink location=prout.ts \
// udpsrc port=10002 ! rtpbin.recv_rtcp_sink_0 \
// udpsrc port=10003 caps="application/x-rtp,media=(string)audio,payload=(int)111,clock-rate=(int)48000,encoding-name=(string)OPUS" ! rtpbin.recv_rtp_sink_1 \
// rtpbin. ! rtpopusdepay ! opusdec ! voaacenc ! queue ! tsmux. \
// udpsrc port=10004 ! rtpbin.recv_rtcp_sink_1
type GstSession struct {
	id                    int
	ctx                   context.Context
	elements              *ProtectedMap
	callbackCtx           *gst.CallbackCtx
	callbackCtxEncoder    *gst.CallbackCtx
	AudioBufferList       chan []byte
	AudioDataStartFeeding chan bool
	AudioDataStarted      bool
	VideoBufferList       chan []byte
	VideoDataStartFeeding chan bool
	VideoDataStarted      bool
	AudioRtcpBufferList   chan []byte
	VideoRtcpBufferList   chan []byte
	EncodersMutex         sync.RWMutex
	Encoders              []*GstSession
	decoder               *GstSession
	audioReceived         bool
	videoReceived         bool
	WebrtcUpCh            chan bool
	WebrtcUpSignalSent    bool
	audioBitrate          int
	videoBitrate          int
	maxVideoBitrate       int
	limitVideoBitrate			int
	audio                 chan *srtp.PacketRTP
	video                 chan *srtp.PacketRTP
	RawAudioSampleList    chan *gst.GstSample
	RawVideoSampleList    chan *gst.GstSample
	/*VideoRtcpBufferList   chan []byte
	AudioRtcpBufferList   chan []byte*/
	CodecOption CodecOptions
	HardwareCodecUsed			bool
	lastJitters						[]uint32
}

var gstSessionId int32 = 0

func NewGstSession(ctx context.Context, audio chan *srtp.PacketRTP, video chan *srtp.PacketRTP, c *connectionUdp, rAddr *net.UDPAddr, vSsrcId uint32, aSsrcId uint32, codecOption CodecOptions, maxVideoBitrate int) (s *GstSession) {
	log := plogger.FromContextSafe(ctx)

	s = new(GstSession)
	s.elements = NewProtectedMap()
	s.AudioDataStartFeeding = make(chan bool)
	s.VideoDataStartFeeding = make(chan bool)
	s.AudioBufferList = make(chan []byte, 1000)
	s.VideoBufferList = make(chan []byte, 1000)
	s.AudioRtcpBufferList = make(chan []byte, 1000)
	s.VideoRtcpBufferList = make(chan []byte, 1000)
	s.AudioDataStarted = false
	s.VideoDataStarted = false
	s.audioBitrate = config.Bitrates.Audio.Max
	log.Warnf("NEWGSTSESSION WITH maxVideoBitrate = %d", maxVideoBitrate)
	s.videoBitrate = maxVideoBitrate / 2
	s.limitVideoBitrate = maxVideoBitrate
	s.maxVideoBitrate = maxVideoBitrate
	s.WebrtcUpCh = make(chan bool)
	s.WebrtcUpSignalSent = false
	s.audioReceived = false
	s.videoReceived = false
	s.audio = audio
	s.video = video
	s.RawAudioSampleList = make(chan *gst.GstSample, 10000)
	s.RawVideoSampleList = make(chan *gst.GstSample, 10000)
	// add prefix to logger
	s.ctx = plogger.NewContextAddPrefix(ctx, "GST")
	s.CodecOption = codecOption

	return
}

type CodecOptions int

const (
	CodecNone CodecOptions = iota
	CodecVP8
	CodecH264
)

func (s *GstSession) GetAudioBitrate() int {
	return s.audioBitrate
}

func (s *GstSession) GetVideoBitrate() int {
	return s.videoBitrate
}

func (s *GstSession) AdjustEncodersBitrate(ctx context.Context, bitrate uint32) {
	log := plogger.FromContextSafe(s.ctx)
	s.EncodersMutex.RLock()
	defer s.EncodersMutex.RUnlock()
	// search index
	for _, sEnc := range s.Encoders {
		if uint32(sEnc.videoBitrate) > bitrate {
			log.Warnf("DOWN ENCODING BITRATE %d -> %d because DECODING bitrate is %d\n", sEnc.videoBitrate, bitrate, bitrate)
			sEnc.SetEncodingVideoBitrate(int(bitrate))
			sEnc.limitVideoBitrate = int(bitrate)
		} else {
			if uint32(sEnc.videoBitrate) < bitrate && sEnc.limitVideoBitrate < int(bitrate) {
				log.Warnf("UP ENCODING BITRATE %d -> %d because DECODING bitrate is now higher %d", sEnc.videoBitrate, bitrate, bitrate)
				sEnc.limitVideoBitrate = int(bitrate)
				//sEnc.SetEncodingVideoBitrate(int(bitrate))
			}
		}
	}
}
