package main

import (
	"context"
	"errors"
	"fmt"
	"net"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/gst"
	"github.com/heytribe/live-webrtcsignaling/packet"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

func (s *GstSession) FlushStart(element *gst.GstElement) (err error) {
	event := gst.EventNewFlushStart()
	gst.ElementSendEvent(element, event)

	return
}

func (s *GstSession) FlushStop(element *gst.GstElement) (err error) {
	event := gst.EventNewFlushStop()
	gst.ElementSendEvent(element, event)

	return
}

func CreateEncoder(ctx context.Context, codecOption CodecOptions, c *connectionUdp, audioOut chan *srtp.PacketRTP, videoOut chan *srtp.PacketRTP, sDecoder *GstSession, rAddr *net.UDPAddr, vSsrcId uint32, aSsrcId uint32, maxVideoBitrate int) (s *GstSession, err error) {
	var e *gst.GstElement

	log := plogger.FromContextSafe(ctx).Prefix("GST:Encoder").Tag("gst")
	ctx = plogger.NewContext(ctx, log)
	s = NewGstSession(
		ctx,
		audioOut,
		videoOut,
		c,
		rAddr,
		vSsrcId,
		aSsrcId,
		codecOption,
		maxVideoBitrate,
	)
	s.callbackCtxEncoder = gst.NewCallbackCtx()
	s.decoder = sDecoder

	e, err = gst.PipelineNew("")
	if log.OnError(err, "Could not create a new GStreamer pipeline") {
		return
	}
	s.elements.Set("pencoder", e)

	e, err = gst.ElementFactoryMake("appsrc", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}

	gst.ObjectSet(ctx, e, "is-live", true)
	gst.ObjectSet(ctx, e, "format", 3)
	s.elements.Set("appsrcrawvideo", e)

	e, err = gst.ElementFactoryMake("appsrc", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}
	s.elements.Set("encoderAppSrcRtcpVideo", e)

	queueVideo, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}

	e, err = gst.ElementFactoryMake("appsrc", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}
	gst.ObjectSet(ctx, e, "is-live", true)
	gst.ObjectSet(ctx, e, "format", 3)
	s.elements.Set("appsrcrawaudio", e)

	e, err = gst.ElementFactoryMake("appsrc", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}
	s.elements.Set("encoderAppSrcRtcpAudio", e)

	queueAudio, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}

	switch codecOption {
	case CodecVP8:
		if config.Mode == ModeMCU {
			e, err = gst.ElementFactoryMake("vaapivp8enc", "")
			err = fmt.Errorf("Disabling VP8 HW Encoder")
			if log.OnError(err, "No VP8 VAAPI hardware encoder available") {
				e, err = gst.ElementFactoryMake("vp8enc", "")
				if log.OnError(err, "Could not create a Gstreamer element factory") {
					return
				}
				gst.ObjectSet(ctx, e, "target-bitrate", s.videoBitrate)
				gst.ObjectSet(ctx, e, "min-quantizer", 2)
				gst.ObjectSet(ctx, e, "max-quantizer", 56)
				gst.ObjectSet(ctx, e, "static-threshold", 1)
				//gst.ObjectSet(ctx, e, "keyframe-max-dist", 999999)
				gst.ObjectSet(ctx, e, "keyframe-max-dist", 3000)
				gst.ObjectSet(ctx, e, "threads", config.CpuCores)
				gst.ObjectSet(ctx, e, "end-usage", config.Vp8.EndUsage)
				gst.ObjectSet(ctx, e, "cpu-used", config.Vp8.CpuUsed)
				gst.ObjectSet(ctx, e, "token-partitions", config.Vp8.TokenPartitions)
				gst.ObjectSet(ctx, e, "deadline", config.Vp8.Deadline)
				gst.ObjectSet(ctx, e, "error-resilient", config.Vp8.ErrorResilient)
				gst.ObjectSet(ctx, e, "undershoot", 100)
				gst.ObjectSet(ctx, e, "overshoot", 15)
				gst.ObjectSet(ctx, e, "buffer-size", 1000)
				gst.ObjectSet(ctx, e, "buffer-initial-size", 5000)
				gst.ObjectSet(ctx, e, "buffer-optimal-size", 600)
				//gst.ObjectSet(ctx, e, "max-intra-bitrate", 1400)
				gst.ObjectSet(ctx, e, "keyframe-mode", 0)
				s.elements.Set("videoCodec", e)
			} else {
				log.Infof("Using VP8 Hardware Encoder")
				gst.ObjectSet(ctx, e, "rate-control", 2)
				log.Warnf("SET START VIDEOBITRATE LISTENER TO %d kbits/s", s.videoBitrate/1000)
				gst.ObjectSet(ctx, e, "bitrate", s.videoBitrate/1000)
				gst.ObjectSet(ctx, e, "keyframe-period", 0)
				s.elements.Set("videoCodec", e)

				e, err = gst.ElementFactoryMake("vaapipostproc", "")
				if log.OnError(err, "Could not create a Gstreamer element factory") {
					return
				}
				gst.ObjectSet(ctx, e, "width", 160)
				gst.ObjectSet(ctx, e, "height", 0)
				s.elements.Set("vaapiPostProcessing", e)

				s.HardwareCodecUsed = true
			}

			e, err = gst.ElementFactoryMake("capsfilter", "")
			if log.OnError(err, "Could not create a Gstreamer element factory") {
				return
			}
			caps := gst.CapsFromString("video/x-vp8,profile=(string)1")
			gst.ObjectSet(ctx, e, "caps", caps)
			s.elements.Set("videoCodecCaps", e)
		}

		e, err = gst.ElementFactoryMake("rtpvp8pay", "")
		if log.OnError(err, "Could not create a Gstreamer element factory") {
			return
		}
		log.Debugf("Set ssrc of vp8 pay to %d", vSsrcId)
		gst.ObjectSet(ctx, e, "ssrc", vSsrcId)
		gst.ObjectSet(ctx, e, "pt", 96)
		//gst.ObjectSet(ctx, e, "max-ptime", 40000)
		//gst.ObjectSet(ctx, e, "min-ptime", 40000)
		gst.ObjectSet(ctx, e, "mtu", 1200)
		gst.ObjectSet(ctx, e, "picture-id-mode", 2)
		gst.ObjectSet(ctx, e, "perfect-rtptime", true)
		s.elements.Set("rtpVideoPay", e)
	case CodecH264:
		if config.Mode == ModeMCU {
			var videoCaps *gst.GstElement
			videoCaps, err = gst.ElementFactoryMake("capsfilter", "")
			if log.OnError(err, "Could not create a Gstreamer element factory") {
				return
			}
			s.elements.Set("videoCodecCaps", videoCaps)

			e, err = gst.ElementFactoryMake("vaapih264enc", "")
			err = fmt.Errorf("Disabling H264 HW Encoder")
			if log.OnError(err, "No H264 VAAPI hardware encoder available") {
				e, err = gst.ElementFactoryMake("openh264enc", "")
				if log.OnError(err, "Could not create a Gstreamer element factory") {
					return
				}
				gst.ObjectSet(ctx, e, "bitrate", s.videoBitrate)
				gst.ObjectSet(ctx, e, "max-bitrate", s.videoBitrate)
				gst.ObjectSet(ctx, e, "rate-control", 1)
				gst.ObjectSet(ctx, e, "usage-type", 0)
				gst.ObjectSet(ctx, e, "complexity", 1)
				gst.ObjectSet(ctx, e, "qp-min", 4)
				gst.ObjectSet(ctx, e, "qp-max", 51)
				gst.ObjectSet(ctx, e, "slice-mode", 5)
				gst.ObjectSet(ctx, e, "deblocking", 0)
				gst.ObjectSet(ctx, e, "enable-denoise", true)
				s.elements.Set("videoCodec", e)

				/*e, err = gst.ElementFactoryMake("x264enc", "")
				if log.OnError(err, "Could not create a Gstreamer element factory") {
					return
				}
				//gst.ObjectSet(ctx, e, "sliced-threads", true)
				gst.ObjectSet(ctx, e, "threads", config.CpuCores)
				gst.ObjectSet(ctx, e, "byte-stream", true)
				gst.ObjectSet(ctx, e, "bitrate", s.videoBitrate/1000)
				gst.ObjectSet(ctx, e, "speed-preset", 6)
				gst.ObjectSet(ctx, e, "tune", 0x00000004)
				//gst.ObjectSet(ctx, e, "cabac", false)
				//gst.ObjectSet(ctx, e, "b-adapt", false)
				//gst.ObjectSet(ctx, e, "dct8x8", false)
				//gst.ObjectSet(ctx, e, "option-string", "level=3.1")*/

				caps := gst.CapsFromString("video/x-h264,alignment=(string)au,stream-format=(string)byte-stream,profile=(string)baseline")
				gst.ObjectSet(ctx, videoCaps, "caps", caps)
			} else {
				log.Infof("Using H264 Hardware Encoder")
				log.Warnf("SET START VIDEOBITRATE LISTENER TO %d kbits/s", s.videoBitrate/1000)
				gst.ObjectSet(ctx, e, "bitrate", s.videoBitrate/1000)
				gst.ObjectSet(ctx, e, "cabac", false)
				gst.ObjectSet(ctx, e, "dct8x8", false)
				gst.ObjectSet(ctx, e, "max-bframes", 0)
				gst.ObjectSet(ctx, e, "rate-control", 2)
				gst.ObjectSet(ctx, e, "keyframe-period", 300)

				s.elements.Set("videoCodec", e)

				e, err = gst.ElementFactoryMake("vaapipostproc", "")
				if log.OnError(err, "Could not create a Gstreamer element factory") {
					return
				}
				gst.ObjectSet(ctx, e, "width", 160)
				gst.ObjectSet(ctx, e, "height", 0)
				s.elements.Set("vaapiPostProcessing", e)

				caps := gst.CapsFromString("video/x-h264,alignment=(string)au,stream-format=(string)byte-stream,profile=(string)constrained-baseline")
				gst.ObjectSet(ctx, videoCaps, "caps", caps)
				s.HardwareCodecUsed = true
			}
		}

		e, err = gst.ElementFactoryMake("rtph264pay", "")
		if log.OnError(err, "Could not create a Gstreamer element factory") {
			return
		}
		gst.ObjectSet(ctx, e, "ssrc", vSsrcId)
		gst.ObjectSet(ctx, e, "pt", 102)
		gst.ObjectSet(ctx, e, "mtu", 1200)
		gst.ObjectSet(ctx, e, "perfect-rtptime", true)
		gst.ObjectSet(ctx, e, "config-interval", -1)
		s.elements.Set("rtpVideoPay", e)
	default:
		err = fmt.Errorf("Unknown codec option %d", codecOption)
		return
	}

	var opusEnc *gst.GstElement
	var opusParse *gst.GstElement
	//if config.Mode == ModeMCU {
		opusEnc, err = gst.ElementFactoryMake("opusenc", "")
		if log.OnError(err, "Could not create a Gstreamer element factory") {
			return
		}
		gst.ObjectSet(ctx, opusEnc, "bandwidth", -1000)
		gst.ObjectSet(ctx, opusEnc, "bitrate", s.audioBitrate)
		gst.ObjectSet(ctx, opusEnc, "audio-type", 2048)
		gst.ObjectSet(ctx, opusEnc, "inband-fec", true)
		gst.ObjectSet(ctx, opusEnc, "frame-size", 20)
		gst.ObjectSet(ctx, opusEnc, "bitrate-type", 1)
		//gst.ObjectSet(ctx, opusEnc, "hard-resync", true)
		gst.ObjectSet(ctx, opusEnc, "perfect-timestamp", true)

		opusParse, err = gst.ElementFactoryMake("opusparse", "")
		if logOnError(err, "Could not create a Gstreamer element factory") {
			return
		}
	//}

	rtpOpusPay, err := gst.ElementFactoryMake("rtpopuspay", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}
	log.Debugf("Set ssrc of opus pay to %d", aSsrcId)
	gst.ObjectSet(ctx, rtpOpusPay, "ssrc", aSsrcId)
	gst.ObjectSet(ctx, rtpOpusPay, "pt", 111)
	gst.ObjectSet(ctx, rtpOpusPay, "mtu", 1300)
	//gst.ObjectSet(ctx, rtpOpusPay, "ptime-multiple", 10000)
	//gst.ObjectSet(ctx, rtpOpusPay, "min-ptime", 10000)
	//gst.ObjectSet(ctx, rtpOpusPay, "max-ptime", 10000)
	gst.ObjectSet(ctx, rtpOpusPay, "perfect-rtptime", true)

	appSinkEncodedRtpVideo, err := gst.ElementFactoryMake("appsink", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}
	gst.ObjectSet(ctx, appSinkEncodedRtpVideo, "sync", false)
	s.elements.Set("appSinkEncodedRtpVideo", appSinkEncodedRtpVideo)

	appSinkEncodedRtpAudio, err := gst.ElementFactoryMake("appsink", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}
	gst.ObjectSet(ctx, appSinkEncodedRtpAudio, "sync", false)
	s.elements.Set("appSinkEncodedRtpAudio", appSinkEncodedRtpAudio)

	qv1, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}
	s.elements.Set("qv1", qv1)

	qv2, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}

	qv3, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}

	qv4, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}

	qa1, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}

	qa2, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}

	qa3, err := gst.ElementFactoryMake("queue", "")
	if log.OnError(err, "Could not create a Gstreamer element factory") {
		return
	}

	switch config.Mode {
	case ModeMCU:
		gst.BinAddMany(s.elements.Get("pencoder").(*gst.GstElement),
			s.elements.Get("appsrcrawvideo").(*gst.GstElement), queueVideo,
			//s.elements.Get("vaapiPostProcessing").(*gst.GstElement),
			s.elements.Get("videoCodec").(*gst.GstElement),
			qv1, s.elements.Get("videoCodecCaps").(*gst.GstElement),
			qv2,
			qv3, s.elements.Get("rtpVideoPay").(*gst.GstElement),
			qv4, appSinkEncodedRtpVideo,
			s.elements.Get("appsrcrawaudio").(*gst.GstElement), queueAudio, opusEnc, qa1,
			opusParse, qa2, rtpOpusPay, qa3, appSinkEncodedRtpAudio,
		)
	case ModeSFU:
		gst.BinAddMany(
			s.elements.Get("pencoder").(*gst.GstElement),
			s.elements.Get("appsrcrawvideo").(*gst.GstElement),
			queueVideo,
			s.elements.Get("rtpVideoPay").(*gst.GstElement),
			qv2,
			appSinkEncodedRtpVideo,
			s.elements.Get("appsrcrawaudio").(*gst.GstElement),
			qa1,
			opusEnc,
			opusParse,
			qa2,
			rtpOpusPay,
			appSinkEncodedRtpAudio,
		)
	default:
		err = fmt.Errorf("Unknown mode %d", config.Mode)
		return
	}

	var gstElementList []*gst.GstElement

	switch codecOption {
	case CodecH264:
		gstElementList = append(gstElementList, s.elements.Get("appsrcrawvideo").(*gst.GstElement))
		if config.Mode == ModeMCU {
			/*if s.HardwareCodecUsed == true {
				gstElementList = append(gstElementList, s.elements.Get("vaapiPostProcessing").(*gst.GstElement))
			}*/
			gstElementList = append(gstElementList, s.elements.Get("videoCodec").(*gst.GstElement))
			gstElementList = append(gstElementList, qv1)
			if s.HardwareCodecUsed == false {
				gstElementList = append(gstElementList, s.elements.Get("videoCodecCaps").(*gst.GstElement))
				gstElementList = append(gstElementList, qv2)
			}
		}
		gstElementList = append(gstElementList, s.elements.Get("rtpVideoPay").(*gst.GstElement))
		gstElementList = append(gstElementList, appSinkEncodedRtpVideo)
	case CodecVP8:
		gstElementList = append(gstElementList, s.elements.Get("appsrcrawvideo").(*gst.GstElement))
		if config.Mode == ModeMCU {
			/*if s.HardwareCodecUsed == true {
				gstElementList = append(gstElementList, s.elements.Get("vaapiPostProcessing").(*gst.GstElement))
			}*/
			gstElementList = append(gstElementList, s.elements.Get("videoCodec").(*gst.GstElement))
			gstElementList = append(gstElementList, qv2)
			if s.HardwareCodecUsed == false {
				gstElementList = append(gstElementList, s.elements.Get("videoCodecCaps").(*gst.GstElement))
				gstElementList = append(gstElementList, qv3)
			}
		}
		gstElementList = append(gstElementList, s.elements.Get("rtpVideoPay").(*gst.GstElement))
		gstElementList = append(gstElementList, appSinkEncodedRtpVideo)
	}

	err = gst.ElementLinkMany(ctx, gstElementList...)
	if log.OnError(err, "could not link video elements") {
		return
	}

	switch config.Mode {
	case ModeMCU:
		err = gst.ElementLinkMany(ctx,
			s.elements.Get("appsrcrawaudio").(*gst.GstElement),
			//opusEnc,
			//qa1,
			//opusParse,
			//qa2,
			rtpOpusPay,
			appSinkEncodedRtpAudio,
		)
	case ModeSFU:
		err = gst.ElementLinkMany(ctx,
			s.elements.Get("appsrcrawaudio").(*gst.GstElement),
			opusEnc,
			qa1,
			rtpOpusPay,
			qa2,
			appSinkEncodedRtpAudio,
		)
	default:
		err = fmt.Errorf("Unknown mode %d", config.Mode)
		return
	}

	if log.OnError(err, "Could not link video elements") {
		return
	}

	// update pad offset to change its time
	gst.AdjustClockOffset(sDecoder.elements.Get("pdecoder").(*gst.GstElement),
		s.elements.Get("pencoder").(*gst.GstElement))

	//s.GetAndSetDecoderCaps(ctx)

	stateReturn := gst.ElementSetState(s.elements.Get("pencoder").(*gst.GstElement), gst.StatePlaying)
	log.Infof("State return of pencoder pipeline is %#v", stateReturn)

	//gst.DebugBinToDotFile(s.elements.Get("pencoder").(*gst.GstElement), "pencoder_pipeline_graph")

	/*encoder := NewEncoder()
	encoder.videoBitrate = s.videoBitrate*/
	sDecoder.EncodersMutex.Lock()
	sDecoder.Encoders = append(sDecoder.Encoders, s)
	sDecoder.EncodersMutex.Unlock()

	go s.handleRawVideoData(ctx, s)
	go s.handleRawAudioData(ctx, s)
	go s.handleRtpVideoEncodedData(ctx, rAddr)
	go s.handleRtpAudioEncodedData(ctx, rAddr)

	/*go func() {
		time.Sleep(10 * time.Second)
		s.FlushStart(s.elements.Get("appsrcrawvideo").(*gst.GstElement))
		s.FlushStop(s.elements.Get("appsrcrawvideo").(*gst.GstElement))
		gst.ObjectSet(ctx, s.elements.Get("vaapiPostProcessing").(*gst.GstElement), "width", 320)
	}()*/

	return
}

func (s *GstSession) StartPipelines() {
}

func (s *GstSession) dumpPipelineToDotFile() {
	gst.DebugBinToDotFile(s.elements.Get("pdecoder").(*gst.GstElement), "pdecoder_pipeline_graph")
}

func (s *GstSession) GetAndSetDecoderCaps(ctx context.Context) {
	log := plogger.FromContextSafe(ctx)
	element := s.elements.Get("appsrcrawvideo").(*gst.GstElement)
	appSinkRawVideoPad := gst.ElementGetStaticPad(s.decoder.elements.Get("appsinkrawvideo").(*gst.GstElement), "sink")
	caps := gst.PadGetCurrentCaps(appSinkRawVideoPad)
	log.Warnf("Set Encoder video caps to %s", gst.CapsToString(caps))
	gst.ObjectSet(ctx, element, "caps", caps)
}

func (s *GstSession) handleRawVideoData(ctx context.Context, e *GstSession) {
	var err error
	var gstSample *gst.GstSample
	//var buffer *gst.GstBuffer
	//var oldBufferSize uint
	var oldBufferWidth uint32
	var oldBufferHeight uint32
	//var bufferSize uint

	log := plogger.FromContextSafe(ctx)
	element := s.elements.Get("appsrcrawvideo").(*gst.GstElement)
	for {
		select {
		case <-ctx.Done():
			log.Infof("goroutine handleRawVideoData exit")
			s.decoder.EncodersMutex.Lock()
			// search index
			for i := 0; i < len(s.decoder.Encoders); i++ {
				if e == s.decoder.Encoders[i] {
					log.Infof("found the encoder entry, delete it")
					s.decoder.Encoders = append(s.decoder.Encoders[:i], s.decoder.Encoders[i+1:]...)
					log.Infof("s.decoder.Encoders is now %#v", s.decoder.Encoders)
					break
				}
			}
			s.decoder.EncodersMutex.Unlock()
			gst.ElementSetState(s.elements.Get("pencoder").(*gst.GstElement), gst.StateNull)
			return
		case gstSample = <-e.RawVideoSampleList:
			log.Debugf("RECEIVED RAW VIDEO DATA")
			if (gstSample.Width != 0 && gstSample.Width != oldBufferWidth) ||
				 (gstSample.Height != 0 && gstSample.Height != oldBufferHeight) {
				s.FlushStart(s.elements.Get("appsrcrawvideo").(*gst.GstElement))
				s.FlushStop(s.elements.Get("appsrcrawvideo").(*gst.GstElement))
				//oldBufferSize = bufferSize
				// hacky (test)
				if gstSample.Width != 0 {
					oldBufferWidth = gstSample.Width
				}
				if gstSample.Height != 0 {
					oldBufferHeight = gstSample.Height
				}
				appSinkRawVideoPad := gst.ElementGetStaticPad(s.decoder.elements.Get("appsinkrawvideo").(*gst.GstElement), "sink")
				caps := gst.PadGetCurrentCaps(appSinkRawVideoPad)
				log.Warnf("Resolution change: %s %d %d", gst.CapsToString(caps), gstSample.Width, gstSample.Height)
				//gst.ObjectSet(ctx, element, "caps", caps)
			}
			err = gst.AppSrcPushSample(element, gstSample)
			log.OnError(err, "Could not handle raw video data")
			gst.SampleUnref(gstSample)
		}
	}
}

func (s *GstSession) handleRawAudioData(ctx context.Context, e *GstSession) {
	var err error
	var gstSample *gst.GstSample

	log, _ := plogger.FromContext(s.ctx)
	element := s.elements.Get("appsrcrawaudio").(*gst.GstElement)
	for {
		select {
		case <-ctx.Done():
			log.Infof("goroutine handleRawAudioData exit")
			return
		case gstSample = <-e.RawAudioSampleList:
			err = gst.AppSrcPushSample(element, gstSample)
			log.OnError(err, "Could not handle raw audio data")
			gst.SampleUnref(gstSample)
		}
	}
}

func (s *GstSession) handleRtpVideoEncodedData(ctx context.Context, rAddr *net.UDPAddr) {
	var err error
	var gstSample *gst.GstSample
	var gstBuffer *gst.GstBuffer

	log, _ := plogger.FromContext(s.ctx)
	e := s.elements.Get("appSinkEncodedRtpVideo").(*gst.GstElement)
	for {
		if s.videoReceived == false {
			s.videoReceived = true
			s.checkAndSendWebrtcUp()
		}
		gstSample, err = gst.AppSinkPullSample(e)
		if err != nil {
			if gst.AppSinkIsEOS(e) == true {
				log.Infof("goroutine handleRtpVideoEncodedData exit EOS")
				return
			} else {
				log.Warnf("handleRtpVideoEncodedData: could not get sample from appSinkEncodedRtpVideo")
				continue
			}
		}
		gstBuffer, err = gst.SampleGetBuffer(gstSample)
		if log.OnError(err, "could not get gstBuffer from gstSample") {
			continue
		}
		data, err := gst.BufferGetData(gstBuffer)
		if log.OnError(err, "Could not get raw rtp encoded video packet from GstBuffer") {
			continue
		}
		gst.SampleUnref(gstSample)
		udpPacket := packet.NewUDPFromData(data, rAddr)
		rtpPacket := srtp.NewPacketRTP(udpPacket)

		// Push in jitterbuffer (the module will save the packet and send it on the network
		log.Debugf("SEQ %d", rtpPacket.GetSeqNumber())
		s.video <- rtpPacket
	}
}

func (s *GstSession) handleRtpAudioEncodedData(ctx context.Context, rAddr *net.UDPAddr) {
	var err error
	var gstSample *gst.GstSample
	var gstBuffer *gst.GstBuffer

	log, _ := plogger.FromContext(s.ctx)
	e := s.elements.Get("appSinkEncodedRtpAudio").(*gst.GstElement)
	for {
		if s.audioReceived == false {
			s.audioReceived = true
			s.checkAndSendWebrtcUp()
		}
		gstSample, err = gst.AppSinkPullSample(e)
		if err != nil {
			if gst.AppSinkIsEOS(e) == true {
				log.Infof("goroutine handleRtpAudioEncodedData exit EOS")
				return
			} else {
				log.Warnf("handleRtpAudioEncodedData: could not get sample from appSinkEncodedRtpAudio")
				continue
			}
		}
		gstBuffer, err = gst.SampleGetBuffer(gstSample)
		if log.OnError(err, "could not get gstBuffer from gstSample") {
			continue
		}
		data, err := gst.BufferGetData(gstBuffer)
		if log.OnError(err, "Could not get raw rtp encoded audio packet from GstBuffer") {
			continue
		}
		gst.SampleUnref(gstSample)
		udpPacket := packet.NewUDPFromData(data, rAddr)
		rtpPacket := srtp.NewPacketRTP(udpPacket)

		s.audio <- rtpPacket
	}
}

// Set bitrate in bits/s
func (s *GstSession) SetEncodingVideoBitrate(v int) error {
	log, _ := plogger.FromContext(s.ctx)
	log.Warnf("MAX VIDEOBITRATE IS %d / LIMIT VIDEOBITRATE IS %d / ENCODING BITRATE REQUESTED IS %d", s.maxVideoBitrate, s.limitVideoBitrate, v)
	if s.maxVideoBitrate != 0 && (s.maxVideoBitrate < v || s.limitVideoBitrate < v) {
		if s.limitVideoBitrate < v {
			log.Infof("bitrate is already at the maximum because the decoding bitrate could not be higher (%d kbits/s)", s.limitVideoBitrate)
		} else {
			log.Infof("bitrate is already at the maximum value of %d", s.maxVideoBitrate)
		}
		return nil
	}
	e := s.elements.Get("videoCodec")
	if e == nil {
		err := errors.New(`No encoder found s.elements.Get("videoCodec") == nil`)
		log.OnError(err, "")
		return err
	}
	bitrate := v
	log.Warnf("SET LISTENER BITRATE TO %d bits/s", bitrate)

	if s.HardwareCodecUsed == true {
		bitrate /= 1000
	}
	switch s.CodecOption {
	case CodecVP8:
		if s.HardwareCodecUsed == true {
			gst.ObjectSet(s.ctx, e.(*gst.GstElement), "bitrate", bitrate)
		} else {
			gst.ObjectSet(s.ctx, e.(*gst.GstElement), "target-bitrate", bitrate)
		}
	case CodecH264:
		if s.HardwareCodecUsed == true {
			stateReturn := gst.ElementSetState(s.elements.Get("videoCodec").(*gst.GstElement), gst.StateNull)
			log.Infof("State return of videoCodec element is %#v", stateReturn)
			gst.BinRemove(s.elements.Get("pencoder").(*gst.GstElement), s.elements.Get("videoCodec").(*gst.GstElement))
			log.Warnf("SET START VIDEOBITRATE LISTENER TO %d kbits/s", bitrate)
			e, err := gst.ElementFactoryMake("vaapih264enc", "")
			if err != nil {
			  return errors.New("Could not create a GStreamer element factory")
			}
			gst.ObjectSet(s.ctx, e, "bitrate", bitrate)
			gst.ObjectSet(s.ctx, e, "cabac", false)
			gst.ObjectSet(s.ctx, e, "dct8x8", false)
			gst.ObjectSet(s.ctx, e, "max-bframes", 0)
			gst.ObjectSet(s.ctx, e, "rate-control", 1)
			gst.ObjectSet(ctx, e, "keyframe-period", 300)
			s.elements.Set("videoCodec", e)
			gst.BinAdd(s.elements.Get("pencoder").(*gst.GstElement), e)
			gst.ElementLinkMany(
				s.ctx,
				s.elements.Get("appsrcrawvideo").(*gst.GstElement),
				e,
				s.elements.Get("qv1").(*gst.GstElement),
			)
			stateReturn = gst.ElementSetState(e, gst.StatePlaying)
			log.Infof("State return of videoCodec element is %#v", stateReturn)
			//gst.ObjectSet(s.ctx, e.(*gst.GstElement), "bitrate", bitrate)
		} else {
			log.Warnf("SET bitrate and maxbitrate to %d", bitrate)
			gst.ObjectSet(s.ctx, e.(*gst.GstElement), "bitrate", bitrate)
			gst.ObjectSet(s.ctx, e.(*gst.GstElement), "max-bitrate", bitrate)
		}
	}

	s.videoBitrate = v
	return nil
}

func (s *GstSession) GetVideoEncodingBitrate() int {
	return s.videoBitrate
}

func (s *GstSession) SetEncodingAudioBitrate(v int) error {
	log, _ := plogger.FromContext(s.ctx)
	opusEnc := s.elements.Get("opusenc")
	if opusEnc == nil {
		err := errors.New(`No encoder found s.elements.Get("opusenc") == nil`)
		log.OnError(err, "")
		return err
	}
	gst.ObjectSet(s.ctx, opusEnc.(*gst.GstElement), "bitrate", v)
	s.audioBitrate = v
	return nil
}

func (s *GstSession) ForceKeyFrame() (err error) {
	e := s.elements.Get("videoCodec").(*gst.GstElement)
	if e == nil {
		err = errors.New(`No encoder found s.elements.Get("videoCodec") == nil`)
		return
	}
	event := gst.EventNewCustom(gst.EventCustomDownstream, gst.StructureNewEmpty("GstForceKeyUnit", false))
	gst.ElementSendEvent(e, event)

	return
}

func (s *GstSession) AddJitterStat(jitter uint32) {
	log, _ := plogger.FromContext(s.ctx)
	log.Warnf("SET JITTER WITH ADDJISTERSTAT TO %d", jitter)
	if len(s.lastJitters) > 0 {
		if jitter > 1500 {
			videoBitrate := int(float64(s.videoBitrate) * float64(0.9))
			if videoBitrate < 32000 {
				videoBitrate = 32000
			}
			log.Warnf("videoBitrate is %d", videoBitrate)
			s.SetEncodingVideoBitrate(videoBitrate)
		} else {
			if jitter < 500 {
				videoBitrate := int(float64(s.videoBitrate) * float64(1.1))
				if videoBitrate > 1024000 {
					videoBitrate = 1024000
				}
				log.Warnf("videoBitrate is %d", videoBitrate)
				s.SetEncodingVideoBitrate(videoBitrate)
			}
		}
	}
	s.lastJitters = append(s.lastJitters, jitter)
	if len(s.lastJitters) > 50 {
		s.lastJitters = s.lastJitters[1:51]
	}
}

func (s *GstSession) SetMaxVideoEncodingBitrate(bitrate int) {
	fmt.Printf("CALL TO GSTSESSION SETMAXVIDEOENCODINGBITRATE with %d", bitrate)
	s.maxVideoBitrate = bitrate
	if s.GetVideoBitrate() > s.maxVideoBitrate {
		s.SetEncodingVideoBitrate(s.maxVideoBitrate)
	}
}
