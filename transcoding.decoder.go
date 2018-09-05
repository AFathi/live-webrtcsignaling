package main

import (
	"context"
	"fmt"
	"net"
	//"errors"
	//"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/gst"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

func NewDecoderParse(ctx context.Context, codecOption CodecOptions, audioIn chan *srtp.PacketRTP, videoIn chan *srtp.PacketRTP, c *connectionUdp, rAddr *net.UDPAddr, vSsrcId uint32, vPayloadType uint16, aSsrcId uint32, aPayloadType uint16) (s *GstSession, err error) {
	log := plogger.FromContextSafe(ctx).Prefix("GST:Decoder").Tag("gst")
	ctx = plogger.NewContext(ctx, log)
	s = NewGstSession(ctx, audioIn, videoIn, c, rAddr, vSsrcId, aSsrcId, codecOption, 0)
	s.callbackCtx = gst.NewCallbackCtx()

	e, err := gst.ParseLaunchFull(fmt.Sprintf(`
		appsrc name=appsrcrtpvideo is-live=true do-timestamp=true format=3 caps=application/x-rtp,media=(string)video,payload=(int)%d,clock-rate=(int)90000,encoding-name=(string)H264 !
		queue ! rtph264depay !
		queue ! h264parse config-interval=-1 !
		queue ! vaapih264dec !
		queue ! appsink name=appsinkrawvideo
		appsrc name=appsrcrtpaudio do-timestamp=true is-live=true format=3 caps=application/x-rtp,media=(string)audio,payload=(int)%d,clock-rate=(int)48000,encoding-name=(string)OPUS !
		queue ! rtpopusdepay !
		queue ! opusparse !
		queue ! opusdec plc=true inband-fec=true !
		queue ! appsink name=appsinkrawaudio`, vPayloadType, aPayloadType), nil, gst.ParseFlagNone)
	if log.OnError(err, "Could not create a new GStreamer pipeline") {
		return
	}
	s.elements.Set("pdecoder", e)

	s.elements.Set("appsrcrtpvideo", gst.ElementGetByName(e, "appsrcrtpvideo"))
	s.elements.Set("appsrcrtpaudio", gst.ElementGetByName(e, "appsrcrtpaudio"))
	s.elements.Set("appsinkrawvideo", gst.ElementGetByName(e, "appsinkrawvideo"))
	s.elements.Set("appsinkrawaudio", gst.ElementGetByName(e, "appsinkrawaudio"))

	stateReturn := gst.ElementSetState(
		s.elements.Get("pdecoder").(*gst.GstElement), gst.StatePlaying,
	)
	log.Warnf("State return of pdecoder pipeline is %#v", stateReturn)

	go s.handleVideoData(ctx, rAddr, vSsrcId, aSsrcId)
	go s.handleAudioData(ctx)
	go s.handleVideoRawData(ctx)
	go s.handleAudioRawData(ctx)

	return
}

func NewDecoder(ctx context.Context, codecOption CodecOptions, audioIn chan *srtp.PacketRTP, videoIn chan *srtp.PacketRTP, c *connectionUdp, rAddr *net.UDPAddr, vSsrcId uint32, vPayloadType uint16, aSsrcId uint32, aPayloadType uint16) (s *GstSession, err error) {
	var e *gst.GstElement

	log := plogger.FromContextSafe(ctx).Prefix("GST:Decoder").Tag("gst")
	ctx = plogger.NewContext(ctx, log)
	s = NewGstSession(ctx, audioIn, videoIn, c, rAddr, vSsrcId, aSsrcId, codecOption, 0)
	s.callbackCtx = gst.NewCallbackCtx()

	e, err = gst.PipelineNew("")
	if log.OnError(err, "Could not create a new GStreamer pipeline") {
		return
	}
	s.elements.Set("pdecoder", e)

	e, err = gst.ElementFactoryMake("appsrc", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}
	gst.ObjectSet(ctx, e, "do-timestamp", true)
	gst.ObjectSet(ctx, e, "is-live", true)
	gst.ObjectSet(ctx, e, "format", 3)
	var videoCaps *gst.GstCaps
	switch codecOption {
	case CodecVP8:
		videoCaps = gst.CapsFromString(fmt.Sprintf("application/x-rtp,media=(string)video,payload=(int)%d,clock-rate=(int)90000,encoding-name=(string)VP8", vPayloadType))
	case CodecH264:
		//videoCaps = gst.CapsFromString(fmt.Sprintf("application/x-rtp, media=(string)video, payload=(int)%d, clock-rate=(int)90000, encoding-name=(string)H264, packetization-mode=(int)1, profile-level-id=(string)42e01f", vPayloadType))
		videoCaps = gst.CapsFromString(fmt.Sprintf("application/x-rtp,media=(string)video,payload=(int)%d,clock-rate=(int)90000,encoding-name=(string)H264", vPayloadType))
	default:
		err = fmt.Errorf("Unknown codec option %d", codecOption)
		return
	}
	gst.ObjectSet(ctx, e, "caps", videoCaps)
	//s.callbackCtx.SetNeedDataCallback(ctx, e, needDataCallback, id, 0)
	s.elements.Set("appsrcrtpvideo", e)

	e, err = gst.ElementFactoryMake("appsrc", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}
	gst.ObjectSet(ctx, e, "do-timestamp", true)
	gst.ObjectSet(ctx, e, "is-live", true)
	gst.ObjectSet(ctx, e, "format", 3)
	audioCaps := gst.CapsFromString(fmt.Sprintf("application/x-rtp,media=(string)audio,payload=(int)%d,clock-rate=(int)48000,encoding-name=(string)OPUS", aPayloadType))
	gst.ObjectSet(ctx, e, "caps", audioCaps)
	//s.callbackCtx.SetNeedDataCallback(ctx, e, needDataCallback, id, 1)
	s.elements.Set("appsrcrtpaudio", e)

	switch codecOption {
	case CodecVP8:
		e, err = gst.ElementFactoryMake("rtpvp8depay", "")
		if log.OnError(err, "Could not create a GStreamer element factory") {
			return
		}
		s.elements.Set("rtpVideoDepay", e)

		if config.Mode == ModeMCU {
			e, err = gst.ElementFactoryMake("vaapivp8dec", "")
			err = fmt.Errorf("Disabling VP8 HW")
			if log.OnError(err, "No VP8 VAAPI hardware decoder available") {
				e, err = gst.ElementFactoryMake("vp8dec", "")
				if log.OnError(err, "Could not create a Gstreamer element factory") {
					return
				}
				gst.ObjectSet(ctx, e, "threads", 8)
				gst.ObjectSet(ctx, e, "enable-denoise", true)
				gst.ObjectSet(ctx, e, "complexity", "high")
				gst.ObjectSet(ctx, e, "post-processing", true)
			} else {
				log.Infof("Using VP8 Hardware Decoder")
				s.HardwareCodecUsed = true
			}
			s.elements.Set("videoCodec", e)
		}
	case CodecH264:
		e, err = gst.ElementFactoryMake("rtph264depay", "")
		if log.OnError(err, "Could not create a GStreamer element factory") {
			return
		}
		s.elements.Set("rtpVideoDepay", e)

			e, err = gst.ElementFactoryMake("capsfilter", "")
			if log.OnError(err, "Could not create a Gstreamer element factory") {
				return
			}
			caps := gst.CapsFromString("video/x-h264,stream-format=(string)byte-stream,alignment=(string)au")
			gst.ObjectSet(ctx, e, "caps", caps)
			s.elements.Set("videoCodecFilterAfterDepay", e)

			e, err = gst.ElementFactoryMake("h264parse", "")
			if log.OnError(err, "Could not create a Gstreamer element factory") {
				return
			}
			gst.ObjectSet(ctx, e, "config-interval", -1)
			s.elements.Set("videoCodecParser", e)
if config.Mode == ModeMCU {
			//e, err = gst.ElementFactoryMake("avdec_h264", "")
			e, err = gst.ElementFactoryMake("vaapih264dec", "")
			err = fmt.Errorf("Disabling H264 HW")
			if log.OnError(err, "No H264 VAAPI hardware decoder available") {
				// No VAAPI ? Trying to create H264 SW decoder
				e, err = gst.ElementFactoryMake("openh264dec", "")
				if log.OnError(err, "Could not create a GStreamer element factory") {
					return
				}
			} else {
				s.HardwareCodecUsed = true
				log.Infof("Using H264 Hardware Decoder")
			}
			s.elements.Set("videoCodec", e)
		}
	default:
		err = fmt.Errorf("Unknown codec option %d", codecOption)
		return
	}

	var qv5 *gst.GstElement
	if features.IsActive(ctx, "facedetect") && config.Mode == ModeMCU {
		// I420 -> to RGB
		e, err = gst.ElementFactoryMake("videoconvert", "I420toRGB")
		if log.OnError(err, "Could not create colorspace GStreamer element factory") {
			return
		}
		s.elements.Set("I420toRGB", e)

		e, err = gst.ElementFactoryMake("facedetect", "facedetect")
		if log.OnError(err, "Could not create facedetect GStreamer element factory") {
			return
		}
		s.elements.Set("facedetect", e)

		// I420 -> to RGB
		e, err = gst.ElementFactoryMake("videoconvert", "RGBtoI420")
		if log.OnError(err, "Could not create colorspace GStreamer element factory") {
			return
		}
		s.elements.Set("RGBtoI420", e)
		e, err = gst.ElementFactoryMake("capsfilter", "")
		if log.OnError(err, "Could not create a Gstreamer element factory") {
			return
		}
		convertCaps := gst.CapsFromString("video/x-raw,format=I420")
		gst.ObjectSet(ctx, e, "caps", convertCaps)
		s.elements.Set("capsfilterI420", e)

		qv5, err = gst.ElementFactoryMake("queue", "qv5")
		if log.OnError(err, "could not create a Gstreamer element factory") {
			return
		}
	}

	if config.Mode == ModeMCU {
		e, err = gst.ElementFactoryMake("videorate", "")
		if log.OnError(err, "Could not create a GStreamer element factory") {
			return
		}
		//gst.ObjectSet(ctx, e, "max-rate", 25)
		//gst.ObjectSet(ctx, e, "drop-only", true)
		s.elements.Set("videorate", e)

		e, err = gst.ElementFactoryMake("capsfilter", "")
		if log.OnError(err, "Could not create a GStreamer element factory") {
			return
		}
		caps := gst.CapsFromString("video/x-raw,framerate=25/1")
		gst.ObjectSet(ctx, e, "caps", caps)
		s.elements.Set("videoratecaps", e)
	}

	e, err = gst.ElementFactoryMake("rtpopusdepay", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}
	s.elements.Set("rtpopusdepay", e)

	e, err = gst.ElementFactoryMake("opusparse", "")
	if log.OnError(err, "Could not create a GStreamer element factory") {
		return
	}
	s.elements.Set("opusParse", e)

  var opusDec *gst.GstElement
	//if config.Mode == ModeMCU {
		opusDec, err = gst.ElementFactoryMake("opusdec", "")
		if log.OnError(err, "Could not create a GStreamer element factory") {
			return
		}
		gst.ObjectSet(ctx, opusDec, "plc", true)
		gst.ObjectSet(ctx, opusDec, "use-inband-fec", true)
	//}

	e, err = gst.ElementFactoryMake("appsink", "")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}
	//gst.ObjectSet(ctx, e, "sync", false)
	s.elements.Set("appsinkrawvideo", e)

	e, err = gst.ElementFactoryMake("appsink", "")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}
	//gst.ObjectSet(ctx, e, "sync", false)
	s.elements.Set("appsinkrawaudio", e)

	qv1, err := gst.ElementFactoryMake("queue", "qv1")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}

	qv2, err := gst.ElementFactoryMake("queue", "qv2")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}

	qv3, err := gst.ElementFactoryMake("queue", "qv3")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}

	qv4, err := gst.ElementFactoryMake("queue", "qv4")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}

	qa1, err := gst.ElementFactoryMake("queue", "qa1")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}

	qa2, err := gst.ElementFactoryMake("queue", "qa2")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}

	qa3, err := gst.ElementFactoryMake("queue", "qa3")
	if log.OnError(err, "could not create a Gstreamer element factory") {
		return
	}

	var videoCodecParser *gst.GstElement
	var videoCodecFilterAfterDepay *gst.GstElement
	if e := s.elements.Get("videoCodecParser"); e != nil {
		videoCodecParser = e.(*gst.GstElement)
		videoCodecFilterAfterDepay = s.elements.Get("videoCodecFilterAfterDepay").(*gst.GstElement)
	}

	switch config.Mode {
	case ModeMCU:
		gst.BinAddMany(s.elements.Get("pdecoder").(*gst.GstElement),
			s.elements.Get("appsrcrtpvideo").(*gst.GstElement),
			s.elements.Get("appsrcrtpaudio").(*gst.GstElement),
			s.elements.Get("rtpVideoDepay").(*gst.GstElement),
			s.elements.Get("videoCodec").(*gst.GstElement),
			videoCodecParser, videoCodecFilterAfterDepay,
			s.elements.Get("videorate").(*gst.GstElement),
			s.elements.Get("videoratecaps").(*gst.GstElement),
			s.elements.Get("appsinkrawvideo").(*gst.GstElement),
			s.elements.Get("rtpopusdepay").(*gst.GstElement),
			s.elements.Get("opusParse").(*gst.GstElement),
			opusDec,
			s.elements.Get("appsinkrawaudio").(*gst.GstElement),
			qv1, qv2, qv3, qv4, qv5, qa1, qa2, qa3,
		)
	case ModeSFU:
		gst.BinAddMany(
			s.elements.Get("pdecoder").(*gst.GstElement),
			s.elements.Get("appsrcrtpvideo").(*gst.GstElement),
			s.elements.Get("appsrcrtpaudio").(*gst.GstElement),
			s.elements.Get("rtpVideoDepay").(*gst.GstElement),
			videoCodecParser,
			videoCodecFilterAfterDepay,
			s.elements.Get("appsinkrawvideo").(*gst.GstElement),
			s.elements.Get("rtpopusdepay").(*gst.GstElement),
			s.elements.Get("opusParse").(*gst.GstElement),
			s.elements.Get("appsinkrawaudio").(*gst.GstElement),
			opusDec,
			qv1, qv2, qv3, qv4, qv5, qa1, qa2, qa3,
		)
	default:
		err = fmt.Errorf("Unknown mode %d", config.Mode)
		return
	}

	if features.IsActive(ctx, "facedetect") {
		gst.BinAddMany(
			s.elements.Get("pdecoder").(*gst.GstElement),
			s.elements.Get("I420toRGB").(*gst.GstElement),
			s.elements.Get("facedetect").(*gst.GstElement),
			s.elements.Get("RGBtoI420").(*gst.GstElement),
			s.elements.Get("capsfilterI420").(*gst.GstElement))
	}

	var gstElementList []*gst.GstElement

	gstElementList = append(gstElementList, s.elements.Get("appsrcrtpvideo").(*gst.GstElement))
	//gstElementList = append(gstElementList, s.elements.Get("queueRtpVideo").(*gst.GstElement))
	gstElementList = append(gstElementList, s.elements.Get("rtpVideoDepay").(*gst.GstElement))
	switch codecOption {
	case CodecH264:
			gstElementList = append(gstElementList, qv1)
			gstElementList = append(gstElementList, videoCodecFilterAfterDepay)
		if config.Mode == ModeMCU {
			gstElementList = append(gstElementList, qv2)
			gstElementList = append(gstElementList, videoCodecParser)
			gstElementList = append(gstElementList, qv3)
			gstElementList = append(gstElementList, s.elements.Get("videoCodec").(*gst.GstElement))
			gstElementList = append(gstElementList, qv4)
			gstElementList = append(gstElementList, s.elements.Get("videorate").(*gst.GstElement))
			gstElementList = append(gstElementList, s.elements.Get("videoratecaps").(*gst.GstElement))
		}
	case CodecVP8:
		gstElementList = append(gstElementList, qv1)
		if config.Mode == ModeMCU {
			gstElementList = append(gstElementList, s.elements.Get("videoCodec").(*gst.GstElement))
			gstElementList = append(gstElementList, qv2)
			gstElementList = append(gstElementList, s.elements.Get("videorate").(*gst.GstElement))
			gstElementList = append(gstElementList, s.elements.Get("videoratecaps").(*gst.GstElement))
			gstElementList = append(gstElementList, qv3)
		}
	}
	if features.IsActive(ctx, "facedetect") {
		gstElementList = append(gstElementList, s.elements.Get("I420toRGB").(*gst.GstElement))
		gstElementList = append(gstElementList, s.elements.Get("facedetect").(*gst.GstElement))
		gstElementList = append(gstElementList, s.elements.Get("RGBtoI420").(*gst.GstElement))
		gstElementList = append(gstElementList, s.elements.Get("capsfilterI420").(*gst.GstElement))
		gstElementList = append(gstElementList, qv5)
	}
	gstElementList = append(gstElementList, s.elements.Get("appsinkrawvideo").(*gst.GstElement))
	err = gst.ElementLinkMany(ctx, gstElementList...)
	if log.OnError(err, "could not link video elements") {
		return
	}

	switch config.Mode {
	case ModeMCU:
		err = gst.ElementLinkMany(ctx,
			s.elements.Get("appsrcrtpaudio").(*gst.GstElement),
			s.elements.Get("rtpopusdepay").(*gst.GstElement),
			qa1,
			s.elements.Get("opusParse").(*gst.GstElement),
			qa2,
			//opusDec,
			s.elements.Get("appsinkrawaudio").(*gst.GstElement),
		)
	case ModeSFU:
		err = gst.ElementLinkMany(ctx,
			s.elements.Get("appsrcrtpaudio").(*gst.GstElement),
			s.elements.Get("rtpopusdepay").(*gst.GstElement),
			qa1,
			s.elements.Get("opusParse").(*gst.GstElement),
			qa2,
			opusDec,
			qa3,
			s.elements.Get("appsinkrawaudio").(*gst.GstElement),
		)
	default:
		err = fmt.Errorf("Unknown mode %d", config.Mode)
		return
	}

	if log.OnError(err, "could not link video elements") {
		return
	}

	stateReturn := gst.ElementSetState(
		s.elements.Get("pdecoder").(*gst.GstElement), gst.StatePlaying,
	)
	log.Warnf("State return of pdecoder pipeline is %#v", stateReturn)

	go s.handleVideoData(ctx, rAddr, vSsrcId, aSsrcId)
	go s.handleAudioData(ctx)
	go s.handleVideoRawData(ctx)
	go s.handleAudioRawData(ctx)

	return
}

func (s *GstSession) checkAndSendWebrtcUp() {
	if s.WebrtcUpSignalSent == false && s.videoReceived == true && s.audioReceived == true {
		go func(ch chan bool) {
			ch <- true
		}(s.WebrtcUpCh)
		s.WebrtcUpSignalSent = true
	}
}

func (s *GstSession) handleVideoData(ctx context.Context, rAddr *net.UDPAddr, vSsrcId uint32, aSsrcId uint32) {
	log, _ := plogger.FromContext(ctx)
	e := s.elements.Get("appsrcrtpvideo").(*gst.GstElement)

	/*
	 * pipelining: packets => jitterbuffer => rtcpReporterRR => output
	 */
	for {
		select {
		case <-ctx.Done():
			log.Infof("goroutine handleVideoData exit")
			gst.ElementSetState(s.elements.Get("pdecoder").(*gst.GstElement), gst.StateNull)
			return
		case p := <-s.video:
			if s.videoReceived == false {
				s.videoReceived = true
				s.checkAndSendWebrtcUp()
			}
			/* tempfix: disabling reporterRR
				s.videoRtcpReporterRR.PushPacket(data)
			case p := <-s.videoRtcpReporterRR.GetOutCh():
			*/
			if p == nil || p.GetData() == nil {
				log.Warnf("packet is nil, packet dump: %#v", p)
				continue
			}
			data := p.GetData()
			buffer, err := gst.BufferNewWrapped(data)
			if log.OnError(err, "could not handle video data") {
				return
			}
			err = gst.AppSrcPushBuffer(e, buffer)
			if log.OnError(err, "Could not handle video data") {
				return
			}
			log.Debugf("Pushed a RTP Video buffer")
			//log.Warnf("RTP VIDEO PUSHED BUFFER")
			/* tempfix: disabling reporterRR
			case p := <-s.videoRtcpReporterRR.GetOutRtcpCh():
				s.cUdp.send <- packet.IPacketUDP(p).(*packet.UDP)
			*/
		}

	}
}

func (s *GstSession) handleAudioData(ctx context.Context) {
	log, _ := plogger.FromContext(ctx)
	e := s.elements.Get("appsrcrtpaudio").(*gst.GstElement)
	//<-s.AudioDataStartFeeding
	for {
		select {
		case <-ctx.Done():
			log.Infof("goroutine handleAudioData exit")
			return
		case p := <-s.audio:
			if s.audioReceived == false {
				s.audioReceived = true
				s.checkAndSendWebrtcUp()
			}
			if p == nil || p.GetData() == nil {
				log.Warnf("packet is nil, packet dump: %#v", p)
				continue
			}
			buffer, err := gst.BufferNewWrapped(p.GetData())
			if log.OnError(err, "could not handleData") {
				return
			}
			err = gst.AppSrcPushBuffer(e, buffer)
			if log.OnError(err, "Could not handleData") {
				return
			}
			log.Debugf("Pushed a RTP Audio buffer")
		}
	}
}

func (s *GstSession) handleAudioRawData(ctx context.Context) {
	var gstSample *gst.GstSample
	var err error

	log := plogger.FromContextSafe(ctx)
	e := s.elements.Get("appsinkrawaudio").(*gst.GstElement)
	for {
		gstSample, err = gst.AppSinkPullSample(e)
		if err != nil {
			if gst.AppSinkIsEOS(e) == true {
				log.Infof("goroutine handleAudioRawData exit EOS")
				return
			} else {
				log.Warnf("handleAudioRawData: could not get sample from appsinkrawaudio")
				continue
			}
		}
		log.Debugf("Send a raw audio buffer")
		s.EncodersMutex.RLock()
		for _, e := range s.Encoders {
			e.RawAudioSampleList <- gst.SampleRef(gstSample)
		}
		s.EncodersMutex.RUnlock()
		gst.SampleUnref(gstSample)
	}
}

func (s *GstSession) handleVideoRawData(ctx context.Context) {
	var gstSample *gst.GstSample
	var err error

	log := plogger.FromContextSafe(ctx)
	e := s.elements.Get("appsinkrawvideo").(*gst.GstElement)
	for {
		gstSample, err = gst.AppSinkPullSample(e)
		if err != nil {
			if gst.AppSinkIsEOS(e) == true {
				log.Infof("goroutine handleVideoRawData exit EOS")
				return
			} else {
				log.Warnf("handleAudioRawData: could not get sample from appsinkrawvideo")
				continue
			}
		}
		s.EncodersMutex.RLock()
		for _, e := range s.Encoders {
			log.Debugf("Send a raw video buffer")
			e.RawVideoSampleList <- gst.SampleRef(gstSample)
		}
		s.EncodersMutex.RUnlock()
		//log.Warnf("RAW DATA SAMPLE UNREF")
		gst.SampleUnref(gstSample)
	}
}
