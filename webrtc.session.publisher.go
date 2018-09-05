package main

import (
	"context"
	"reflect"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

/*
 * FIXME: refactor
 *  we should avoid inter-locking by tearing up multiple goroutines.
 *
 * Pipeline publisher:
 * cUdp => node UDP
 * Link node UDP => node Demux
 * Link node Demux.OutPacketSRTP => node SRTP
 * Link node SRTP.OutPacketRTP => nodeSplitRTPAV
 * Link node SRTP.OutPacketRTCP => nodeSplitRTCPAV
 * Link node nodeSplitRTPAV.video => nodeJitterBufferVideo
 * Link node nodeSplitRTPAV.audio => nodeJitterBufferAudio
 * Link node nodeSplitRTCPAV.video => nodeRTCPVideo
 * Link node nodeSplitRTCPAV.audio => nodeRTCPAudio
 * nodeJitterBufferVideo.Out => decoder GSTREAMER => raw data
 * nodeJitterBufferAudio.Out => decoder GSTREAMER => raw data
 * Link node nodeSplitRTCPAV.audio => nodeReporterRRAudio.In
 * Link node nodeSplitRTCPAV.video => nodeReporterRRVideo.In
 */

/*
* Specific goroutine for stun state.
*
* FIXME: refactoring
*  attacher le pipeline a la session
*
* we handle stun messages in a specific go func
* until DTLS & STUN are in their own object & goroutine
* Adding them to pipeline output hooks is locking the MCU.
 */
func (w *WebRTCSession) publisherStateManager(ctx context.Context, decoderAudioIn chan *srtp.PacketRTP, decoderVideoIn chan *srtp.PacketRTP, vSsrcId uint32, vPayloadType uint16, aSsrcId uint32, aPayloadType uint16, nodeSRTP *PipelineNodeSRTP) {
	var err error

	log := plogger.FromContextSafe(ctx).Prefix("STATE-MANAGER").Tag("webrtcsession-publisher")
	ctx = plogger.NewContext(ctx, log)
	for {
		select {
		case <-ctx.Done():
			log.Debugf("STATE MANAGER CTX DONE")
			return
		case stunState := <-w.stunCtx.ChState:
			log.Debugf("FOR LOOP stunState %v", stunState)
			//
			if stunState == StunStateCompleted {
				log.Infof("Stun Session state is now completed for video(and/or audio)")
				nodeVideo := w.p.Get("jittervideo").(*PipelineNodeJitterPublisher)
				nodeVideo.SetRaddr(ctx, w.stunCtx.RAddr)
				nodeAudio := w.p.Get("jitteraudio").(*PipelineNodeJitterPublisher)
				nodeAudio.SetRaddr(ctx, w.stunCtx.RAddr)
				// create Decoder
				log.Debugf("Creating a new session as publisher")
				codec, _ := w.c.wsConn.getPublisherCodec(ctx)
				w.c.gstSession, err = NewDecoder(ctx, codec, decoderAudioIn, decoderVideoIn, w.c, w.stunCtx.RAddr, vSsrcId, vPayloadType, aSsrcId, aPayloadType)
				if err != nil {
					log.Errorf("could not create decoder: %#v", err)
					return
				}
				//w.getBusMessages(ctx, w.c.gstSession.elements.Get("pdecoder").(*gst.GstElement), w.c.gstSession.id)
				log.Infof("Running DTLS client connection for video/audio")
				w.dtlsClientConnect(ctx)
				// hooked
				log.Infof("publisher: PUSHING SRTP SESSION INTO nodeSRTP")
				nodeSRTP.SetSession(ctx, w.c.srtpSession)
				//

				log.Infof("Waiting for receiving audio and video streams from gstreamer pipeline...")
				<-w.c.gstSession.WebrtcUpCh
				log.Infof("connection is up, sending WebRTC up event")
				eventWebrtcUp(ctx, `publisher`, ``, w.c.wsConn.socketId)
				log.Infof("connecting all listeners to %s", w.c.wsConn.socketId)
				w.connectListeners(ctx, w.c.wsConn)
			}
		}
	}
}

/*
 * Goroutine to handle pipeline events
 */
func (w *WebRTCSession) publisherBusManager(ctx context.Context) {
	log := plogger.FromContextSafe(ctx).Prefix("BUS").Tag("webrtcsession-publisher")
	ctx = plogger.NewContext(ctx, log)
	log.Infof("start")
	for {
		select {
		case <-ctx.Done():
			log.Debugf("BUS MANAGER CTX DONE")
			return
		case event := <-w.p.Bus:
			switch e := event.(type) {
			case *PipelineMessageError:
				log.Errorf("PipelineMessageError: %s", e.err)
			case *PipelineMessageStart:
				log.Infof("PipelineMessageStart")
			case *PipelineMessageStop:
				log.Infof("PipelineMessageStop")
			case *PipelineMessageRRStats:
				log.Infof("PipelineMessageRRStats difference=%d jitter=%d ssrc=%d", e.InterarrivalDifference, e.InterarrivalJitter, e.SSRC)
				/*nodeVideo := w.p.Get("jittervideo").(*PipelineNodeJitterPublisher)
				nodeVideo.AddStat(e.InterarrivalDifference)*/
			case *rtcp.PacketALFBRemb:
				log.Infof("*rtcp.PacketALFBRemb received, adjusting encoders bitrates to decoder max bitrate")
				// saving remb sent
				w.lastRembs = append(w.lastRembs, int(e.GetBitrate()))
				if len(w.lastRembs) > 50 {
					w.lastRembs = w.lastRembs[1:51]
				}
				w.c.gstSession.AdjustEncodersBitrate(ctx, e.GetBitrate())
			case *PipelineMessageSetJitterSize:
				log.Infof("PipelineMessageJitterSize size=%d", e.size)
				nodeAudio := w.p.Get("jitteraudio").(*PipelineNodeJitterPublisher)
				nodeAudio.SetJitterSize(e.size)
			case *PipelineMessageInBps:
				// saving bandwidth estimates
				w.lastBandwidthEstimates = append(w.lastBandwidthEstimates, e.Bps)
				if len(w.lastBandwidthEstimates) > 50 {
					w.lastBandwidthEstimates = w.lastBandwidthEstimates[1:51]
				}
			case *PipelineMessageOutBps:
				// skip.
			default:
				log.Warnf("unknown pipeline event received %s %v", reflect.TypeOf(e).String(), e)
			}
		}
	}
}

type RtpInfo struct {
	ssrcId         uint32
	payloadType    uint16
	rtxPayloadType uint16
	clockRate      uint32
}

func (w *WebRTCSession) serveWebRTCPublisher(ctx context.Context, codecOption CodecOptions) {
	log := plogger.FromContextSafe(ctx).Prefix("PUBLISHER").Tag("webrtcsession-publisher")
	ctx = plogger.NewContext(ctx, log)

	/*
	 * Publisher pipeline
	 */
	var video RtpInfo
	var audio RtpInfo
	switch codecOption {
	case CodecVP8:
		video = RtpInfo{
			ssrcId:         w.sdpCtx.offer.GetVideoSSRC(),
			payloadType:    w.sdpCtx.answer.GetVideoPayloadType("VP8"),
			rtxPayloadType: w.sdpCtx.answer.GetRtxPayloadType("VP8"),
			clockRate:      w.sdpCtx.offer.GetVideoClockRate("VP8"),
		}
	case CodecH264:
		video = RtpInfo{
			ssrcId:         w.sdpCtx.offer.GetVideoSSRC(),
			payloadType:    w.sdpCtx.answer.GetVideoPayloadType("H264"),
			rtxPayloadType: w.sdpCtx.answer.GetRtxPayloadType("H264"),
			clockRate:      w.sdpCtx.offer.GetVideoClockRate("H264"),
		}
	}

	audio = RtpInfo{
		ssrcId:      w.sdpCtx.offer.GetAudioSSRC(),
		payloadType: w.sdpCtx.offer.GetAudioPayloadType("opus"),
		clockRate:   w.sdpCtx.offer.GetAudioClockRate("opus"),
	}

	rtx := struct {
		ssrcId uint32
	}{w.sdpCtx.offer.GetRtxSSRC()}

	if video.ssrcId == 0 || audio.ssrcId == 0 {
		log.Errorf("missing ssrc %d %d", video.ssrcId, audio.ssrcId)
		return
	}

	if rtx.ssrcId == 0 {
		log.Infof("missing rtx ssrcId")
	}

	w.p = NewPipeline()
	nodeUDP := NewPipelineNodeUDP(w.udpConn)
	nodeDemux := NewPipelineNodeDemux()
	nodeSRTP := NewPipelineNodeSRTP()
	nodeSplitRTPAV := NewPipelineNodeSplitRTPAV([]uint32{audio.ssrcId}, []uint32{video.ssrcId, rtx.ssrcId})
	nodeSanitizerAudio := NewPipelineNodeSanitizer()
	nodeSanitizerVideo := NewPipelineNodeSanitizer()
	nodeSplitRTCPAV := NewPipelineNodeSplitRTCPAV([]uint32{audio.ssrcId}, []uint32{video.ssrcId, rtx.ssrcId})
	nodeJitterBufferVideo := NewPipelineNodeJitterPublisher(ctx, codecOption, video.payloadType, video.rtxPayloadType, video.clockRate, video.ssrcId, 0, JitterStreamVideo, config.Bitrates.Video, w.stunCtx.rtt)
	nodeJitterBufferAudio := NewPipelineNodeJitterPublisher(ctx, CodecNone, audio.payloadType, audio.rtxPayloadType, audio.clockRate, audio.ssrcId, 0, JitterStreamAudio, config.Bitrates.Audio, w.stunCtx.rtt)
	nodeRTCPAudio := NewPipelineNodeRTCP()
	nodeRTCPVideo := NewPipelineNodeRTCP()
	nodeReporterRRVideo := NewPipelineNodeRTCPReporterRR(video.ssrcId, video.clockRate)
	nodeUDPSink := NewPipelineNodeUDPSink(w.c)

	w.p.Register("udp", nodeUDP)
	w.p.Register("demux", nodeDemux)
	w.p.Register("srtp", nodeSRTP)
	w.p.Register("splitrtpav", nodeSplitRTPAV)
	w.p.Register("sanitizerAudio", nodeSanitizerAudio)
	w.p.Register("sanitizerVideo", nodeSanitizerVideo)
	w.p.Register("splitrtcpav", nodeSplitRTCPAV)
	w.p.Register("jitteraudio", nodeJitterBufferAudio)
	w.p.Register("jittervideo", nodeJitterBufferVideo)
	w.p.Register("rtcpaudio", nodeRTCPAudio)
	w.p.Register("rtcpvideo", nodeRTCPVideo)
	w.p.Register("reporterRRVideo", nodeReporterRRVideo)
	w.p.Register("udpsink", nodeUDPSink)
	w.p.Run(ctx)

	// FIXME: push encoder into a pipeline node
	var decoderAudioIn chan *srtp.PacketRTP
	var decoderVideoIn chan *srtp.PacketRTP
	decoderAudioIn = make(chan *srtp.PacketRTP, 128)
	decoderVideoIn = make(chan *srtp.PacketRTP, 128)

	log = log.Prefix("Pipeline")
	ctx = plogger.NewContext(ctx, log)

	go w.publisherStateManager(ctx, decoderAudioIn, decoderVideoIn, video.ssrcId, video.payloadType, audio.ssrcId, audio.payloadType, nodeSRTP)
	go w.publisherBusManager(ctx)

	/*
	 * Link nodes of publisher pipeline
	 */
	exit := false
	i := 0
	for exit == false {
		i++
		log.Debugf("FOR LOOP %d", i)
		select {
		/*
		 * exit
		 */
		case <-ctx.Done():
			log.Debugf("CTX DONE")
			exit = true
		case packet := <-nodeUDP.Out:
			log.Debugf("nodeDemux.In START")
			select {
			case nodeDemux.In <- packet:
			default:
				log.Warnf("nodeDemux.In is full, dropping packet from nodeUDP.out")
			}
			log.Debugf("nodeDemux.In FINISHED")
		case packet := <-nodeDemux.OutPacketSRTP:
			log.Debugf("nodeSRTP.In START")
			select {
			case nodeSRTP.In <- packet:
			default:
				log.Warnf("nodeSRTP.In is full, dropping packet from nodeDemux.OutPacketSRTP")
			}
			log.Debugf("nodeSRTP.In FINISHED")
		case packet := <-nodeSRTP.OutPacketRTP:
			log.Debugf("nodeSplitRTPAV.In START")
			select {
			case nodeSplitRTPAV.In <- packet:
			default:
				log.Warnf("nodeSplitRTPAV.In is full, dropping packet from nodeSRTP.OutPacketRTP")
			}
			log.Debugf("nodeSplitRTPAV.In FINISHED")
		case packet := <-nodeSRTP.OutPacketRTCP:
			log.Debugf("nodeSplitRTCPAV START")
			select {
			case nodeSplitRTCPAV.In <- packet:
			default:
				log.Warnf("nodeSplitRTCPAV.In is full, dropping packet from nodeSRTP.OutPacketRTP")
			}
			log.Debugf("nodeSplitRTCPAV FINISHED")
		case packet := <-nodeSplitRTPAV.OutPacketRTPAudio:
			log.Debugf("nodeSanitizerAudio start")
			select {
			case nodeSanitizerAudio.In <- packet:
			default:
				log.Warnf("nodeSanitizerAudio.In is full, dropping packet from nodeSplitRTPAV.OutPacketRTPAudio")
			}
			log.Debugf("nodeSanitizerAudio finished")
		case packet := <-nodeSanitizerAudio.Out:
			log.Debugf("nodeJitterBufferAudio start")
			select {
			case nodeJitterBufferAudio.In <- packet:
			default:
				log.Warnf("nodeJitterBufferAudio.In is full, dropping packet from nodeSanitizerAudio.Out")
			}
			log.Debugf("nodeJitterBufferAudio finished")
		case packet := <-nodeSplitRTPAV.OutPacketRTPVideo:
			log.Debugf("nodeSanitizerVideo start")
			select {
			case nodeSanitizerVideo.In <- packet:
			default:
				log.Warnf("nodeSanitizerVideo.In is full, dropping packet from nodeSplitRTPAV.OutPacketRTPVideo")
			}
			log.Debugf("nodeSanitizerVideo finished")
		case packet := <-nodeSanitizerVideo.Out:
			log.Debugf("nodeJitterBufferVideo start")
			select {
			case nodeJitterBufferVideo.In <- packet:
			default:
				log.Warnf("nodeJitterBufferVideo.In is full, dropping packet from nodeSanitizerVideo.Out")
			}
			log.Debugf("nodeJitterBufferVideo finished")
		case packet := <-nodeSplitRTCPAV.OutPacketRTCPAudio:
			log.Debugf("nodeRTCPAudio start")
			select {
			case nodeRTCPAudio.In <- packet:
			default:
				log.Warnf("nodeRTCPAudio.In is full, dropping packet from nodeSplitRTCPAV.OutPacketRTPAudio")
			}
			log.Debugf("nodeRTCPAudio finished")
		case packet := <-nodeSplitRTCPAV.OutPacketRTCPVideo:
			log.Debugf("nodeRTCPVideo start")
			select {
			case nodeRTCPVideo.In <- packet:
			default:
				log.Warnf("nodeRTCPVideo.In is full, dropping packet from nodeSplitRTPAV.OutPacketRTPVideo")
			}
			log.Debugf("nodeRTCPVideo finished")
			log.Debugf("nodeReporterRRVideo.InRTCP start")
			select {
			case nodeReporterRRVideo.InRTCP <- packet:
			default:
				log.Warnf("nodeReporterRRVideo.InRTCP is full, dropping packet from nodeJitterBufferVideo.Out")
			}
			log.Debugf("nodeReporterRRVideo.InRTCP finished")
		case packet := <-nodeJitterBufferAudio.Out:
			log.Debugf("decoderAudioIn start")
			select {
			case decoderAudioIn <- packet:
			default:
				log.Warnf("decoderAudioIn is full, dropping packet from nodeJitterBufferAudio.Out")
			}
			log.Debugf("decoderAudioIn finished")
		case packet := <-nodeJitterBufferVideo.Out:
			log.Debugf("decoderVideoIn start")
			select {
			case decoderVideoIn <- packet:
			default:
				log.Warnf("decoderVideoIn is full, dropping packet from nodeJitterBufferVideo.Out")
			}
			log.Debugf("decoderVideoIn finished")
			log.Debugf("nodeReporterRRVideo.InRTP start")
			select {
			case nodeReporterRRVideo.InRTP <- packet:
			default:
				log.Warnf("nodeReporterRRVideo.InRTP is full, dropping packet from nodeJitterBufferVideo.Out")
			}
			log.Debugf("nodeReporterRRVideo.InRTP finished")
		case rctpRR := <-nodeReporterRRVideo.Out:
			log.Debugf("nodeReporterRRVideo.Out start")
			w.c.writeSrtpRtcpTo(ctx, &RtpUdpPacket{
				RAddr: w.stunCtx.RAddr,
				Data:  rctpRR.Bytes(),
			})
			log.Infof("ReporterRR Video sending report RR %s", rctpRR.String())
			log.Debugf("nodeReporterRRVideo.Out finished")
		case packet := <-nodeJitterBufferAudio.OutRTCP:
			log.Debugf("nodeUDPSink.InRTCP start")
			select {
			case nodeUDPSink.InRTCP <- packet:
			default:
				log.Warnf("nodeUDPSink.InRTCP is full, dropping packet from nodeJitterBufferAudio.OutRTCP")
			}
			log.Debugf("nodeUDPSink.InRTCP finished")
		case packet := <-nodeJitterBufferVideo.OutRTCP:
			log.Debugf("nodeUDPSink.InRTCP start")
			select {
			case nodeUDPSink.InRTCP <- packet:
			default:
				log.Warnf("nodeUDPSink.InRTCP is full, dropping packet from nodeJitterBufferVideo.OutRTCP")
			}
			log.Debugf("nodeUDPSink.InRTCP finished")
		/*
		 * Push stun packet to stun context
		 */
		case packetSTUN := <-nodeDemux.OutPacketSTUN:
			log.Debugf("packetSTUN START")
			if w.stunCtx == nil {
				log.Errorf("[ UDP ] could not found stun session for local address %s", w.c.conn.LocalAddr().String())
			} else {
				if err := w.stunCtx.handleStunMessage(ctx, w.c, packetSTUN); err != nil {
					rAddr := packetSTUN.GetRAddr()
					log.Errorf("could not handle STUN message for %s:%d : %s", rAddr.IP, rAddr.Port, err.Error())
					log.Errorf("dropping STUN packet silentely")
				}
			}
			log.Debugf("packetSTUN finished")
		/*
		 * Push dtls packet to dtls context
		 */
		case packetDTLS := <-nodeDemux.OutPacketDTLS:
			log.Debugf("packetDTLS start")
			rAddr := packetDTLS.GetRAddr()
			if w.c.dtlsSession != nil {
				log.Infof("[ CONNUDP ] ( %s:%d -> %s ) Data to be handled by OpenSSL is %d", rAddr.IP.String(), rAddr.Port, w.c.conn.LocalAddr().String(), packetDTLS.GetSize())
				w.c.dtlsSession.HandleData(packetDTLS.GetData())
			} else {
				log.Errorf("[ CONNUDP ] ( %s:%d -> %s ) Unknown DTLS session, could not handle this DTLS packet", rAddr.IP.String(), rAddr.Port, w.c.conn.LocalAddr().String())
				// FIXME: skip or break ?
			}
			log.Debugf("packetDTLS finished")
		}
	}
}
