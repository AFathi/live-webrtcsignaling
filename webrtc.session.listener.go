package main

import (
	"context"
	"reflect"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/rtcp"
	"github.com/heytribe/live-webrtcsignaling/srtp"
)

/*
* Pipeline listener:
* Link node UDP => node Demux
* Link node Demux.OutPacketSRTP => node SRTP
* Link node SRTP.OutPacketRTP => WARNING normalement, pas de trafic RTP
* Link node SRTP.OutPacketRTCP => nodeRTCP
* raw data => encoder GSTREAMER
* encoder GSTREAMER => nodeJitterBufferVideo
* encoder GSTREAMER => nodeJitterBufferAudio
* nodeJitterBufferVideo => cUdp
* nodeJitterBufferVideo => cUdp
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
func (w *WebRTCSession) listenerStateManager(ctx context.Context, vSsrcId uint32, aSsrcId uint32, rtxSsrcId uint32, nodeSRTP *PipelineNodeSRTP, webRTCSessionPublisher *WebRTCSession, gstreamerAudioOutput, gstreamerVideoOutput chan *srtp.PacketRTP) {
	log := plogger.FromContextSafe(ctx).Prefix("STATE-MANAGER").Tag("webrtcsession-listener")
	ctx = plogger.NewContext(ctx, log)

	if webRTCSessionPublisher == nil {
		log.Warnf("webRTCSessionPublisher is nil, session disconnected ?")
		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Debugf("LISTENER CTX DONE")
			return
		case stunState := <-w.stunCtx.ChState:
			log.Debugf("stunState %v", stunState)
			//
			if stunState == StunStateCompleted {
				log.Infof("Stun Session state is now completed for video(and/or audio)")

				var err error
				// Create DTLS Server attached to the listen port
				log.Infof("Running DTLS server connection for video/audio on port %d, waiting for connection from %s:%d", w.listenPort, w.stunCtx.RAddr.IP.String(), w.stunCtx.RAddr.Port)
				w.dtlsServerAccept(ctx)

				// HOOKING
				log.Infof("listener: PUSHING SRTP SESSION INTO nodeSRTP")
				nodeSRTP.SetSession(ctx, w.c.srtpSession)

				codec, _ := w.c.wsConn.getPublisherCodec(ctx)
				w.c.gstSession, err = CreateEncoder(ctx, codec, w.c, gstreamerAudioOutput, gstreamerVideoOutput, webRTCSessionPublisher.c.gstSession, w.stunCtx.RAddr, vSsrcId, aSsrcId, w.GetMaxVideoBitrate())
				if err != nil {
					log.Errorf("could not create encoder: %#v", err)
					return
				}
				//w.getBusMessages(ctx, w.c.gstSession.elements.Get("pencoder").(*gst.GstElement), w.c.gstSession.id)
				log.Infof("Waiting for receiving audio and video streams from gstreamer pipeline...")
				<-w.c.gstSession.WebrtcUpCh
				log.Infof("connection is up, sending WebRTC up event")
				eventWebrtcUp(ctx, webRTCSessionPublisher.c.wsConn.socketId, webRTCSessionPublisher.c.wsConn.userId, w.c.wsConn.socketId)
				log.Infof("running bus message management")
			}
		}
	}
}

/*
 * Goroutine to handle pipeline events
 */
func (w *WebRTCSession) listenerBusManager(ctx context.Context, gstInPipeline, gstOutPipeline *Pipeline) {
	log := plogger.FromContextSafe(ctx).Prefix("BUS").Tag("webrtcsession-listener")
	ctx = plogger.NewContext(ctx, log)
	lastFractionPacketLost := uint8(0)
	log.Infof("start")
	//pipeleNodeJitterBufferAudio := gstOutPipeline.Get("jitteraudio").(*PipelineNodeJitterListener)
	pipeleNodeJitterBufferVideo := gstOutPipeline.Get("jittervideo").(*PipelineNodeJitterListener)
	for {
		select {
		case <-ctx.Done():
			log.Debugf("BUS MANAGER CTX DONE")
			return
		case event := <-gstInPipeline.Bus:
			switch e := event.(type) {
			case *PipelineMessageError:
				log.Errorf("PipelineMessageError: %s", e.err)
			case *PipelineMessageStart:
				log.Infof("PipelineMessageStart")
			case *PipelineMessageStop:
				log.Infof("PipelineMessageStop")
			case *RtcpContextInfoRemb:
				log.Infof("[ LISTENER ] REMB %d", e.Remb)
				// saving remb received
				w.lastRembs = append(w.lastRembs, e.Remb)
				if len(w.lastRembs) > 50 {
					w.lastRembs = w.lastRembs[1:51]
				}
				// using remb
				//videoBitrate := e.Remb - w.c.gstSession.GetAudioBitrate()
				// 1 / 256 * packetLostFraction == packet loss rate (0.00 -> 1.00)
				packetLossRate := (float64(1) / float64(256)) * float64(lastFractionPacketLost)
				videoBitrate := w.c.gstSession.GetVideoEncodingBitrate()
				if packetLossRate < 0.02 {
					videoBitrate = int(float64(e.Remb) * float64(1.1))
				}
				if packetLossRate > 0.1 {
					videoBitrate = int(float64(e.Remb) * (float64(1) - float64(0.5)*packetLossRate))
				}
				if videoBitrate > e.Remb {
					videoBitrate = e.Remb
				}
				log.Warnf("PACKET LOSS RATE IS %f -- REMB IS %d -- REMB CORRECTED VIDEOBITRATE IS %d", packetLossRate, e.Remb, videoBitrate)
				if videoBitrate >= config.Bitrates.Video.Min && videoBitrate <= config.Bitrates.Video.Max {
					// changing bitrate
					w.lastEncodingBitrate = append(w.lastEncodingBitrate, videoBitrate)
					if len(w.lastEncodingBitrate) > 50 {
						w.lastEncodingBitrate = w.lastEncodingBitrate[1:51]
					}
					/*videoBitrate -= config.Bitrates.Audio.Max
					if videoBitrate < 32000 {
						videoBitrate = 32000
					}*/
					w.c.gstSession.SetEncodingVideoBitrate(videoBitrate)
				} else {
					// keeping bitrate
					currentEncodingBitrate := int(config.Bitrates.Video.Start)
					if len(w.lastEncodingBitrate) > 0 {
						currentEncodingBitrate = w.lastEncodingBitrate[len(w.lastEncodingBitrate)-1]
					}
					w.lastEncodingBitrate = append(w.lastEncodingBitrate, currentEncodingBitrate)
					if len(w.lastEncodingBitrate) > 50 {
						w.lastEncodingBitrate = w.lastEncodingBitrate[1:51]
					}
				}
			case *RtcpContextInfoFIR:
				log.Infof("RtcpContextInfoFIR")
				switch config.Mode {
				case ModeMCU:
					w.c.gstSession.ForceKeyFrame()
				case ModeSFU:
					nodeVideo := w.webRTCSessionPublisher.p.Get("jittervideo").(*PipelineNodeJitterPublisher)
					nodeVideo.SendFIR()
					log.Infof("Reforward RTCP FIR with a RTCP PLI to the publisher XXX to be changed by a real FIR")
				}
			case *rtcp.PacketPSFBPli:
				log.Infof("PacketPSFBPli")
				switch config.Mode {
				case ModeMCU:
					w.c.gstSession.ForceKeyFrame()
				case ModeSFU:
					nodeVideo := w.webRTCSessionPublisher.p.Get("jittervideo").(*PipelineNodeJitterPublisher)
					nodeVideo.SendPLI()
					log.Infof("Reforward RTCP PLI to the publisher")
				}

			case *rtcp.PacketRTPFBNack:
				log.Infof("PacketRTPFBNack")
				ssrc := e.PacketRTPFB.SenderSSRC
				for _, n := range e.RTPFBNacks {
					go pipeleNodeJitterBufferVideo.SendRTX(n.GetSequences(), ssrc)
				}
			case *rtcp.PacketRR:
				ssrc := w.sdpCtx.offer.GetVideoSSRC()
				for _, rb := range e.ReportBlocks {
					if rb.SSRC == ssrc {
						lastFractionPacketLost = rb.FractionLost
						//w.c.gstSession.AddJitterStat(rb.Jitter)
					}
				}
			case *PipelineMessageInBps:
				// skip
			case *PipelineMessageOutBps:
				// skip
			}
		case event := <-gstOutPipeline.Bus:
			switch e := event.(type) {
			case *PipelineMessageInBps:
				// skip
			case *PipelineMessageOutBps:
				// saving bandwidth estimates
				w.lastBandwidthEstimates = append(w.lastBandwidthEstimates, e.Bps)
				if len(w.lastBandwidthEstimates) > 50 {
					w.lastBandwidthEstimates = w.lastBandwidthEstimates[1:51]
				}
			default:
				log.Warnf("unknown pipeline event received %s %v", reflect.TypeOf(e).String(), e)
			}
		}
	}
}

func (w *WebRTCSession) serveWebRTCListener(ctx context.Context, codecOption CodecOptions, webRTCSessionPublisher *WebRTCSession) {
	log := plogger.FromContextSafe(ctx).Prefix("LISTENER").Tag("webrtcsession-listener")
	ctx = plogger.NewContext(ctx, log)

	if webRTCSessionPublisher == nil {
		log.Warnf("webRTCSessionPublisher is nil, session is disconnected ?")
		return
	}

	/*
			 * Listener is composed of two pipelines :
			 *  - gstin pipeline:   read udp sock, parse RTCP (remb, fir, pli, nacks) => call gstramer api / send data
			 *  - gstout pipeline: buffurise rtp data, send RTCP SR
		   *
			 *  Input raw data
			 *        |
			 *  +-----v-----+                                               +-----------+
			 *  |           |--> OUTPUT -> [ GSTOUT_PIPELINE ] -> write |           |
			 *  | Gstreamer |                                               |  udpConn  | <=> client browser
			 *  |           |<-- func call <- [ GSTIN_PIPELINE ] <- read |           |
			 *  +-----------+                                               +-----------+
	*/
	var video RtpInfo
	var audio RtpInfo
	switch codecOption {
	case CodecVP8:
		video = RtpInfo{
			ssrcId:         w.sdpCtx.offer.GetVideoSSRC(),
			payloadType:    w.sdpCtx.offer.GetVideoPayloadType("VP8"),
			rtxPayloadType: w.sdpCtx.offer.GetRtxPayloadType("VP8"),
			clockRate:      w.sdpCtx.offer.GetVideoClockRate("VP8"),
		}
	case CodecH264:
		video = RtpInfo{
			ssrcId:         w.sdpCtx.offer.GetVideoSSRC(),
			payloadType:    w.sdpCtx.offer.GetVideoPayloadType("H264"),
			rtxPayloadType: w.sdpCtx.offer.GetRtxPayloadType("H264"),
			clockRate:      w.sdpCtx.offer.GetVideoClockRate("H264"),
		}
	}

	audio = RtpInfo{
		ssrcId:      w.sdpCtx.offer.GetAudioSSRC(),
		payloadType: w.sdpCtx.offer.GetAudioPayloadType("opus"),
		clockRate:   w.sdpCtx.offer.GetAudioClockRate("opus"),
	}

	// video replay channel
	rtx := struct {
		ssrcId uint32
	}{w.sdpCtx.offer.GetRtxSSRC()}

	if video.payloadType == 0 || audio.payloadType == 0 {
		log.Errorf("missing payloadtype %d %d", video.payloadType, audio.payloadType)
		return
	}

	if video.ssrcId == 0 || audio.ssrcId == 0 || rtx.ssrcId == 0 {
		log.Errorf("missing ssrc %d %d %d", video.ssrcId, audio.ssrcId, rtx.ssrcId)
		return
	}

	// gstIn pipeline nodes
	gstInPipeline := NewPipeline()
	nodeUDP := NewPipelineNodeUDP(w.udpConn)
	nodeDemux := NewPipelineNodeDemux()
	nodeSRTP := NewPipelineNodeSRTP()
	nodeRTCP := NewPipelineNodeRTCP()

	gstInPipeline.Register("udp", nodeUDP)
	gstInPipeline.Register("demux", nodeDemux)
	gstInPipeline.Register("srtp", nodeSRTP)
	gstInPipeline.Register("rtcp", nodeRTCP)
	gstInPipeline.Run(ctx)

	// gstOut pipeline nodes
	gstOutPipeline := NewPipeline()
	// XXX FIX FIX FIX rtx payload type is not +1, it depends of the codec, should fix like publisher
	nodeJitterBufferVideo := NewPipelineNodeJitterListener(ctx, codecOption, video.payloadType, video.payloadType+1, video.clockRate, video.ssrcId, rtx.ssrcId, JitterStreamVideo, config.Bitrates.Video, w.stunCtx.rtt)
	nodeJitterBufferAudio := NewPipelineNodeJitterListener(ctx, codecOption, audio.payloadType, video.payloadType+1, audio.clockRate, audio.ssrcId, rtx.ssrcId, JitterStreamAudio, config.Bitrates.Audio, w.stunCtx.rtt)
	nodeUdpSink := NewPipelineNodeUDPSink(w.c)
	nodeReporterSRVideo := NewPipelineNodeRTCPReporterSR(video.ssrcId, video.clockRate)

	gstOutPipeline.Register("jitteraudio", nodeJitterBufferAudio)
	gstOutPipeline.Register("jittervideo", nodeJitterBufferVideo)
	gstOutPipeline.Register("udpsink", nodeUdpSink)
	gstOutPipeline.Register("reporterSRVideo", nodeReporterSRVideo)
	gstOutPipeline.Run(ctx)

	gstreamerAudioOutput := make(chan *srtp.PacketRTP, 1000)
	gstreamerVideoOutput := make(chan *srtp.PacketRTP, 1000)

	go w.listenerStateManager(ctx,
		video.ssrcId, audio.ssrcId, rtx.ssrcId, nodeSRTP, webRTCSessionPublisher,
		gstreamerAudioOutput, gstreamerVideoOutput)
	go w.listenerBusManager(ctx, gstInPipeline, gstOutPipeline)

	/*
	 * Link nodes of GSTIN_PIPELINE
	 */
	go func() {
		log := log.Prefix("Pipeline").Prefix("GSTIN")
		ctx := plogger.NewContext(ctx, log)

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
				log.Warnf("!!!! should not receive RTP packet !!!! (wtf?) %v", packet)
			case packet := <-nodeSRTP.OutPacketRTCP:
				log.Debugf("nodeRTCP start")
				select {
				case nodeRTCP.In <- packet:
				default:
					log.Warnf("nodeRTCP.In is full, dropping packet from nodeSRTP.OutPacketRTCP")
				}
				log.Debugf("nodeRTCP finished")
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
	}()

	// Sending FIR to the publisher to start with a key frame
	nodeVideo := w.webRTCSessionPublisher.p.Get("jittervideo").(*PipelineNodeJitterPublisher)
	log.Infof("Sending FIR")
	nodeVideo.SendFIR()

	func() {
		log := log.Prefix("Pipeline").Prefix("GSTOUT")
		ctx := plogger.NewContext(ctx, log)

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
				exit = true
			case packet := <-gstreamerAudioOutput:
				log.Debugf("nodeJitterBufferAudio.In START")
				select {
				case nodeJitterBufferAudio.In <- packet:
				default:
					log.Warnf("nodeJitterBufferAudio.In is full, dropping packet from gstreamerAudioOutput")
				}
				log.Debugf("nodeJitterBufferAudio.In FINISHED")
			case packet := <-gstreamerVideoOutput:
				log.Debugf("nodeJitterBufferVideo.In START")
				select {
				case nodeJitterBufferVideo.In <- packet:
				default:
					log.Warnf("nodeJitterBufferVideo.In is full, dropping packet from gstreamerVideoOutput")
				}
				log.Debugf("nodeJitterBufferVideo.In FINISHED")
				log.Debugf("nodeReporterSRVideo.In START")
				select {
				case nodeReporterSRVideo.InRTP <- packet:
				default:
					log.Warnf("nodeReporterSRVideo.In is full, dropping packet from gstreamerVideoOutput")
				}
				log.Debugf("nodeReporterSRVideo.In FINISHED")
			case packet := <-nodeJitterBufferAudio.OutRTP:
				log.Debugf("nodeUdpSink.InRTP START")
				select {
				case nodeUdpSink.InRTP <- packet:
				default:
					log.Warnf("nodeUdpSink.InRTP is full, dropping packet from nodeJitterBufferAudio.OutRTP")
				}
				log.Debugf("nodeUdpSink.InRTP FINISHED")
			case packet := <-nodeJitterBufferAudio.OutRTCP:
				log.Debugf("nodeUdpSink.InRTCP START")
				select {
				case nodeUdpSink.InRTCP <- packet:
				default:
					log.Warnf("nodeUdpSink.InRTCP is full, dropping packet from nodeJitterBufferAudio.OutRTCP")
				}
				log.Debugf("nodeUdpSink.InRTCP FINISHED")
			case packet := <-nodeJitterBufferVideo.OutRTP:
				log.Debugf("nodeUdpSink.InRTP START")
				select {
				case nodeUdpSink.InRTP <- packet:
				default:
					log.Warnf("nodeUdpSink.InRTP is full, dropping packet from nodeJitterBufferVideo.OutRTP")
				}
				log.Debugf("nodeUdpSink.InRTP FINISHED")
			case packet := <-nodeJitterBufferVideo.OutRTCP:
				log.Debugf("nodeUdpSink.InRTCP START")
				select {
				case nodeUdpSink.InRTCP <- packet:
				default:
					log.Warnf("nodeUdpSink.InRTCP is full, dropping packet from nodeJitterBufferVideo.OutRTCP")
				}
				log.Debugf("nodeUdpSink.InRTCP FINISHED")
			case packet := <-nodeReporterSRVideo.Out:
				log.Debugf("nodeReporterSRVideo.Out START")
				// sending rtcp packet
				if w.stunCtx == nil || w.stunCtx.RAddr == nil {
					log.Warnf("cannot send RTCP SR, missing w.stunCtx.RAddr")
				} else {
					rtcpPacketSR := &RtpUdpPacket{
						RAddr: w.stunCtx.RAddr,
						Data:  packet.Bytes(),
					}
					log.Infof("ReporterSR sending report SR %s", packet.String())
					w.c.writeSrtpRtcpTo(ctx, rtcpPacketSR)
				}
				log.Debugf("nodeReporterSRVideo.Out FINISHED")
			}
		}
	}()
}
