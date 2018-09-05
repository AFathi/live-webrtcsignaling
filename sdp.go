package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"

	"encoding/json"
	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-webrtcsignaling/dtls"
	"github.com/heytribe/live-webrtcsignaling/sdp"
	"github.com/kr/pretty"
)

// besoin de ca pour la partie transcoding :
// - session gstreamer rattachée a une WEBRTC session
// - si un gars envoie un flux publisher
//    pouvoir rattacher une autre conn websocket
//    pouvoir rattacher l'e
type SdpContext struct {
	offer  *sdp.SDP
	answer *sdp.SDP
	//
	iceState         string
	iceCandidatePort int
}

func NewSdpCtx() *SdpContext {
	sdpContext := new(SdpContext)

	return sdpContext
}

func parseSDP(ctx context.Context, offer string) (*sdp.SDP, error) {
	log := plogger.FromContextSafe(ctx).Tag("sdp").Prefix("SDP")
	log.Prefix("OFFER").Debugf(offer)
	sdp := sdp.NewSDP(sdp.Dependencies{Logger: sdp.Logger(log)})
	err := sdp.LoadString(offer)
	if err != nil {
		log.Prefix("OFFER").Errorf(err.Error())
	} else {
		log.Prefix("OFFER").Debugf("%# v", pretty.Formatter(sdp))
	}
	return sdp, err
}

func (s *SdpContext) answerSDP(ctx context.Context, preferredCodecOption CodecOptions, listenPort int) (answer string, err error) {
	var address string = config.Network.PublicIPV4

	log := plogger.FromContextSafe(ctx).Tag("sdp").Prefix("SDP")
	if s.offer == nil {
		err = errors.New("could not answer to SDP, no offers received from this SDP context")
		return
	}

	fmt.Printf("SDP OFFER RECEIVED %# v\n", pretty.Formatter(s.offer.Data))

	s.answer = sdp.NewSDP(sdp.Dependencies{Logger: sdp.Logger(log)})
	s.answer.Data.Origin.Username = "-"
	s.answer.Data.Origin.SessionId = randInt64()
	s.answer.Data.Origin.SessionVersion = 2
	s.answer.Data.Origin.Address = address // FIXME
	s.answer.Data.Origin.NetType = "IN"
	s.answer.Data.Origin.AddrType = "IP4"
	s.answer.Data.Name = "Tribe MCU"
	s.answer.Data.Info = "Tribe MCU Server"

	priority := int64(math.Pow(2, 24)*126 + math.Pow(2, 8)*65535 + math.Pow(2, 0)*256)
	iceUfrag := randString(4)
	icePwd := randString(22)

	var midAttributeAudio string
	var midAttributeVideo string

	//
	// adding audio medias: search audio media in offer
	//
	for _, offerMedia := range s.offer.Data.Medias {
		if offerMedia.Type == "audio" {
			//
			// foreach sdp offer media audio, we output a sdp answer media audio
			//   IF the media contains an "opus" codec.
			//
			var answerRtpOpus sdp.Rtp
			var offerRtpOpus bool
			// audio found, searching "opus"
			for _, rtp := range offerMedia.RtpMap {
				if rtp.Codec == "opus" {
					// FIXME: returning the last one ?
					offerRtpOpus = true
					answerRtpOpus = rtp
				}
			}
			if offerRtpOpus == false {
				continue
			}

			// searching mid attribute in offer media
			//  we need this because chrome is using mid:audio & ffox: mid:sdparta_...
			//  @see https://groups.google.com/forum/#!topic/jssip/lPFjVp-_XZA
			midAttributeAudio = "audio"
			for i := 0; i < len(offerMedia.Attributes); i++ {
				if offerMedia.Attributes[i].K == "mid" {
					midAttributeAudio = offerMedia.Attributes[i].V
				}
			}

			// codec is "opus", we can answer
			// creating answer sdp media audio
			var answerMediaAudio = sdp.Media{
				Type:     "audio",
				Port:     offerMedia.Port,
				Protocol: offerMedia.Protocol,
				Fmt:      strconv.Itoa(int(answerRtpOpus.PayloadType)),
				Connection: sdp.Connection{
					Nettype:  "IN",
					Addrtype: "IP4",
					Address:  net.ParseIP(address),
				},
				/*Bandwidth: sdp.Bandwidth{
					Bwtype: "AS",
					Bw:     int(config.Bitrates.Audio.Max / 1000),
				},*/
				IceUfrag: iceUfrag,
				IcePwd:   icePwd,
				Fingerprint: sdp.Fingerprint{
					Type: "sha-256",
					Hash: dtls.GetLocalFingerprint(),
				},
				//
				// This is the MCU IP:Port
				// candidate typ is "host" (no NAT server side)
				//
				// (candidates types could be : "host", "srflx", "prflx", and "relay")
				Candidates: []sdp.Candidate{
					sdp.Candidate{
						Foundation:  "1",
						ComponentId: 1,
						Transport:   "udp",
						Priority:    priority,
						Address:     net.ParseIP(address),
						Port:        listenPort,
						Typ:         "host",
					},
				},
				Attributes: []sdp.Attribute{
					sdp.Attribute{K: "recvonly", V: ""},
					sdp.Attribute{K: "mid", V: midAttributeAudio},
					sdp.Attribute{K: "rtcp-mux", V: ""},
					// the client to send ICE candidates on the fly
					sdp.Attribute{K: "ice-options", V: "trickle"},
					sdp.Attribute{K: "setup", V: "active"},
					// RFC3550 defines the capability to extend the RTP header.
					// This line defines extensions which will be used in RTP headers so that
					// the receiver can decode it correctly and extract the metadata.
					// In this case the browser is indicating that we are going to include
					// information on the audio level in the RTP header as defined in RFC6464.
					//sdp.Attribute{K: "extmap", V: "1 urn:ietf:params:rtp-hdrext:ssrc-audio-level"},
				},
			}
			answerRtpOpus.RtcpFb = []string{}
			// hydrating RtpMap
			answerMediaAudio.RtpMap = make(map[sdp.PayloadType]sdp.Rtp)
			// single audio, answerRtpOpus should have an order=0
			answerRtpOpus.Order = 0
			answerRtpOpus.RtcpFb = []string{}
			answerMediaAudio.RtpMap[answerRtpOpus.PayloadType] = answerRtpOpus
			// ssrc
			answerMediaAudio.SsrcMap = make(map[uint32][]sdp.Attribute)
			//
			answerMediaAudio.PayloadTypes = append(answerMediaAudio.PayloadTypes, answerRtpOpus.PayloadType)
			// adding answerMediaAudio to output
			s.answer.Data.Medias = append(s.answer.Data.Medias, answerMediaAudio)
		}
		if offerMedia.Type == "video" {
			//
			// foreach sdp offer media video, we output a sdp answer media video
			//   IF the media contains a "VP8" codec.
			//
			var answerRtpH264 sdp.Rtp
			var answerRtpVP8 sdp.Rtp
			var answerRtpRtx sdp.Rtp
			var offerRtpVP8 bool
			var offerRtpH264 bool
			var videoPayloadType sdp.PayloadType
			for _, rtp := range offerMedia.RtpMap {
				//fmt.Printf("CHECKING RTP PT %d NAME %s\n", rtp.PayloadType, rtp.Codec)
				switch rtp.Codec {
				case "H264":
					// Checking profile
					for _, fmtp := range rtp.Fmtp {
						/*
							Profile 42C02A :
							@see https://stackoverflow.com/questions/23494168/h264-profile-iop-explained
							@see https://tools.ietf.org/html/rfc6184#section-8.1

							@see https://en.wikipedia.org/wiki/H.264/MPEG-4_AVC
								0x42 => Baseline Profile
								0xC0 => constrained
								Primarily for low-cost applications, this profile is most typically used in videoconferencing and mobile applications. It corresponds to the subset of features that are in common between the Baseline, Main, and High Profiles
						*/
						if fmtp.K == "profile-level-id" &&
							(fmtp.V == "42e01f" || fmtp.V == "42C02A") {
							for _, fmtp2 := range rtp.Fmtp {
								if fmtp2.K == "packetization-mode" && fmtp2.V == "1" {
									offerRtpH264 = true
									answerRtpH264 = rtp
									/*answerRtpH264.Fmtp = []sdp.Attribute{
										sdp.Attribute{K: "profile-level-id", V: "42e01f"},
									}*/
								}
							}
						}
					}
				case "VP8":
					offerRtpVP8 = true
					answerRtpVP8 = rtp
				}
			}

			// try to select the preferred codec if available
			var answerRtp sdp.Rtp
			if preferredCodecOption == CodecH264 && offerRtpH264 {
				answerRtp = answerRtpH264
			} else if preferredCodecOption == CodecVP8 && offerRtpVP8 {
				answerRtp = answerRtpVP8
			} else if offerRtpH264 {
				answerRtp = answerRtpH264
			} else if offerRtpVP8 {
				answerRtp = answerRtpVP8
			} else {
			// no suitable codec found
				continue
			}
			videoPayloadType = answerRtp.PayloadType

			// Searching RTX
			foundRtx := false
			for _, rtp := range offerMedia.RtpMap {
				if rtp.Codec == "rtx" && rtp.Fmtp[0].K == "apt" && rtp.Fmtp[0].V == fmt.Sprintf("%d", videoPayloadType) {
					answerRtpRtx = rtp
					foundRtx = true
				}
			}
			if foundRtx == false {
				log.Infof("could not find RTX channel")
			}

			answerRtp.Order = 0
			if foundRtx {
				answerRtpRtx.Order = 1
			}

			// searching mid attribute in offer media
			//  we need this because chrome is using mid:audio & ffox: mid:sdparta_...
			//  @see https://groups.google.com/forum/#!topic/jssip/lPFjVp-_XZA
			midAttributeVideo = "video"
			for i := 0; i < len(offerMedia.Attributes); i++ {
				if offerMedia.Attributes[i].K == "mid" {
					midAttributeVideo = offerMedia.Attributes[i].V
				}
			}

			// saving IceUfrag & IcePwd
			/*offerVideoIceUfrag = offerMedia.IceUfrag
			offerVideoIcePwd = offerMedia.IcePwd*/
			// codec is "VP8", we can answer
			// creating answer sdp media video
			var answerMediaVideo = sdp.Media{
				Type:     "video",
				Port:     offerMedia.Port,
				Protocol: offerMedia.Protocol,
				Fmt:      strconv.Itoa(int(answerRtp.PayloadType)),
				Connection: sdp.Connection{
					Nettype:  "IN",
					Addrtype: "IP4",
					Address:  net.ParseIP(address),
				},
				/*Bandwidth: sdp.Bandwidth{
					Bwtype: "AS",
					Bw:     int(config.Bitrates.Video.Max / 1000),
				},*/
				IceUfrag: iceUfrag,
				IcePwd:   icePwd,
				Fingerprint: struct {
					Type string
					Hash string
				}{
					Type: "sha-256",
					Hash: dtls.GetLocalFingerprint(),
				},
				Candidates: []sdp.Candidate{
					sdp.Candidate{
						Foundation:  "1",
						ComponentId: 1,
						Transport:   "udp",
						Priority:    priority,
						Address:     net.ParseIP(address),
						Port:        listenPort,
						Typ:         "host",
					},
				},
				Attributes: []sdp.Attribute{
					sdp.Attribute{K: "recvonly", V: ""},
					sdp.Attribute{K: "mid", V: midAttributeVideo},
					sdp.Attribute{K: "rtcp-mux", V: ""},
					sdp.Attribute{K: "ice-options", V: "trickle"},
					sdp.Attribute{K: "setup", V: "active"},
					//sdp.Attribute{K: "extmap", V: "4 urn:3gpp:video-orientation"},
					//sdp.Attribute{K: "extmap", V: "6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"},
				},
			}
			// Check RtcpFb parameters and append all options supported
			rtcpFbSupported := make(map[string]bool)
			rtcpFbSupported["ccm fir"] = true
			rtcpFbSupported["nack"] = true
			rtcpFbSupported["nack pli"] = true
			rtcpFbSupported["goog-remb"] = true
			rtcpFb := []string{}
			for _, fb := range answerRtp.RtcpFb {
				if rtcpFbSupported[fb] {
					rtcpFb = append(rtcpFb, fb)
				}
			}
			answerRtp.RtcpFb = rtcpFb
			// hydrating RtpMap
			answerMediaVideo.RtpMap = make(map[sdp.PayloadType]sdp.Rtp)
			answerMediaVideo.RtpMap[answerRtp.PayloadType] = answerRtp
			if foundRtx {
				answerMediaVideo.RtpMap[answerRtpRtx.PayloadType] = answerRtpRtx
			}
			// ssrc
			answerMediaVideo.SsrcMap = make(map[uint32][]sdp.Attribute)
			//
			answerMediaVideo.PayloadTypes = append(answerMediaVideo.PayloadTypes, answerRtp.PayloadType)
			if foundRtx {
				answerMediaVideo.PayloadTypes = append(answerMediaVideo.PayloadTypes, answerRtpRtx.PayloadType)
			}
			// adding answerMediaVideo to output
			s.answer.Data.Medias = append(s.answer.Data.Medias, answerMediaVideo)
		}
	}

	// BUNDLE groupings establishes a relationship between several media lines
	//  included in the SDP, commonly audio and video.
	// In WebRTC it’s use to multiplex several media flows in the same RTP session
	//  as described in https://tools.ietf.org/html/draft-ietf-mmusic-sdp-bundle-negotiation-39
	s.answer.Data.Attributes = append(s.answer.Data.Attributes, sdp.Attribute{
		K: "group",
		V: "BUNDLE " + midAttributeAudio + " " + midAttributeVideo,
	})

	fmt.Printf("SDP ANSWER %# v\n", pretty.Formatter(s.answer.Data))

	answer = s.answer.Write(ctx)

	return
}

func (s *SdpContext) createSdpOffer(ctx context.Context, codecOption CodecOptions, listenPort int) {
	var address string = config.Network.PublicIPV4

	log, _ := plogger.FromContext(ctx)
	priority := int64(math.Pow(2, 24)*126 + math.Pow(2, 8)*65535 + math.Pow(2, 0)*256)
	s.offer = sdp.NewSDP(sdp.Dependencies{Logger: sdp.Logger(log)})
	s.offer.Data.Origin.Username = "-"
	s.offer.Data.Origin.SessionId = randInt64()
	s.offer.Data.Origin.SessionVersion = 1
	s.offer.Data.Origin.Address = address // FIXME
	s.offer.Data.Origin.NetType = "IN"
	s.offer.Data.Origin.AddrType = "IP4"
	s.offer.Data.Name = "Tribe MCU"
	s.offer.Data.Info = "Tribe MCU Server"
	s.offer.Data.Attributes = append(s.offer.Data.Attributes, sdp.Attribute{
		K: "group",
		V: "BUNDLE audio video",
	})
	s.offer.Data.Attributes = append(s.offer.Data.Attributes, sdp.Attribute{
		K: "msid-semantic",
		V: "WMS tribemcu",
	})

	iceUfrag := randString(4)
	icePwd := randString(22)

	// Building audio SDP part
	offerASsrcId := randUint32()
	opusPayloadTypeId := sdp.PayloadType(111)
	opusFmtp := []sdp.Attribute{}
	//		sdp.Attribute{K: "minptime", V: "10"},
	//sdp.Attribute{K: "useinbandfec", V: "1"},
	//}
	opusRtcpFb := []string{}
	opusRtp := sdp.Rtp{
		Order:       0,
		PayloadType: opusPayloadTypeId,
		Codec:       "opus",
		Rate:        48000,
		Params:      "2",
		Fmtp:        opusFmtp,
		RtcpFb:      opusRtcpFb,
	}

	offerMediaAudio := sdp.Media{
		Type: "audio",
		//Port:     listenPort,
		Port:     9,
		Protocol: "UDP/TLS/RTP/SAVPF",
		Fmt:      strconv.Itoa(int(opusRtp.PayloadType)),
		Connection: sdp.Connection{
			Nettype:  "IN",
			Addrtype: "IP4",
			Address:  net.ParseIP(address),
		},
		IceUfrag: iceUfrag,
		IcePwd:   icePwd,
		Fingerprint: sdp.Fingerprint{
			Type: "sha-256",
			Hash: dtls.GetLocalFingerprint(),
		},
		Candidates: []sdp.Candidate{
			sdp.Candidate{
				Foundation:  "1",
				ComponentId: 1,
				Transport:   "udp",
				Priority:    priority,
				Address:     net.ParseIP(address),
				Port:        listenPort,
				Typ:         "host",
			},
		},
		Attributes: []sdp.Attribute{
			sdp.Attribute{K: "sendonly", V: ""},
			sdp.Attribute{K: "mid", V: "audio"},
			sdp.Attribute{K: "rtcp-mux", V: ""},
			sdp.Attribute{K: "ice-options", V: "trickle"},
			sdp.Attribute{K: "setup", V: "actpass"},
			//sdp.Attribute{K: "extmap", V: "1 urn:ietf:params:rtp-hdrext:ssrc-audio-level"},
		},
	}

	offerMediaAudio.RtpMap = make(map[sdp.PayloadType]sdp.Rtp)
	offerMediaAudio.RtpMap[opusPayloadTypeId] = opusRtp
	offerMediaAudio.SsrcMap = make(map[uint32][]sdp.Attribute)
	offerMediaAudio.SsrcMap[offerASsrcId] = []sdp.Attribute{
		sdp.Attribute{K: "cname", V: "tribemcuserver"},
		sdp.Attribute{K: "msid", V: "tribemcu tribemcua0"},
		sdp.Attribute{K: "mslabel", V: "tribemcu"},
		sdp.Attribute{K: "label", V: "tribemcua0"},
	}
	offerMediaAudio.PayloadTypes = append(offerMediaAudio.PayloadTypes, opusPayloadTypeId)
	s.offer.Data.Medias = append(s.offer.Data.Medias, offerMediaAudio)

	// Building video SDP part
	var videoRtp sdp.Rtp
	var videoPayloadType sdp.PayloadType
	var videoFmtp []sdp.Attribute
	var videoRtpRtx sdp.Rtp
	var videoPayloadTypeRtx sdp.PayloadType
	var videoFmtpRtx []sdp.Attribute
	var codecName string
	videoRtcpFb := []string{
		"ccm fir",
		"nack",
		"nack pli",
		"goog-remb",
	}
	offerVSsrcId := randUint32()
	switch codecOption {
	case CodecVP8:
		videoPayloadType = sdp.PayloadType(96)
		videoPayloadTypeRtx = sdp.PayloadType(97)
		codecName = "VP8"
	case CodecH264:
		videoPayloadType = sdp.PayloadType(102)
		videoPayloadTypeRtx = sdp.PayloadType(103)
		codecName = "H264"
		videoFmtp = []sdp.Attribute{
			sdp.Attribute{K: "level-asymmetry-allowed", V: "1"},
			sdp.Attribute{K: "packetization-mode", V: "1"},
			sdp.Attribute{K: "profile-level-id", V: "42e01f"},
		}
	}

	videoRtp = sdp.Rtp{
		Order:       0,
		PayloadType: videoPayloadType,
		Codec:       codecName,
		Rate:        90000,
		Params:      "",
		Fmtp:        videoFmtp,
		RtcpFb:      videoRtcpFb,
	}

	videoFmtpRtx = []sdp.Attribute{
		sdp.Attribute{K: "apt", V: fmt.Sprintf("%d", videoPayloadType)},
	}
	videoRtpRtx = sdp.Rtp{
		Order:       1,
		PayloadType: videoPayloadTypeRtx,
		Codec:       "rtx",
		Rate:        90000,
		Params:      "",
		Fmtp:        videoFmtpRtx,
		RtcpFb:      []string{},
	}

	offerMediaVideo := sdp.Media{
		Type: "video",
		//Port:     listenPort,
		Port:     9,
		Protocol: "UDP/TLS/RTP/SAVPF",
		Fmt:      strconv.Itoa(int(videoRtp.PayloadType)),
		Connection: sdp.Connection{
			Nettype:  "IN",
			Addrtype: "IP4",
			Address:  net.ParseIP(address),
		},
		IceUfrag: iceUfrag,
		IcePwd:   icePwd,
		Fingerprint: struct {
			Type string
			Hash string
		}{
			Type: "sha-256",
			Hash: dtls.GetLocalFingerprint(),
		},
		Candidates: []sdp.Candidate{
			sdp.Candidate{
				Foundation:  "1",
				ComponentId: 1,
				Transport:   "udp",
				Priority:    priority,
				Address:     net.ParseIP(address),
				Port:        listenPort,
				Typ:         "host",
			},
		},
		Attributes: []sdp.Attribute{
			sdp.Attribute{K: "sendonly", V: ""},
			sdp.Attribute{K: "mid", V: "video"},
			sdp.Attribute{K: "rtcp-mux", V: ""},
			sdp.Attribute{K: "ice-options", V: "trickle"},
			sdp.Attribute{K: "setup", V: "actpass"},
			//sdp.Attribute{K: "extmap", V: "4 urn:3gpp:video-orientation"},
			//sdp.Attribute{K: "extmap", V: "6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"},
		},
	}

	offerMediaVideo.RtpMap = make(map[sdp.PayloadType]sdp.Rtp)
	offerMediaVideo.RtpMap[videoPayloadType] = videoRtp
	offerMediaVideo.RtpMap[videoPayloadTypeRtx] = videoRtpRtx
	offerMediaVideo.SsrcMap = make(map[uint32][]sdp.Attribute)
	offerMediaVideo.SsrcMap[offerVSsrcId] = []sdp.Attribute{
		sdp.Attribute{K: "cname", V: "tribemcuserver"},
		sdp.Attribute{K: "msid", V: "tribemcu tribemcuv0"},
		sdp.Attribute{K: "mslabel", V: "tribemcu"},
		sdp.Attribute{K: "label", V: "tribemcuv0"},
	}
	// hardcoding replay to 424242
	offerRtxSsrcId := uint32(offerVSsrcId+1)
	offerMediaVideo.SsrcMap[offerRtxSsrcId] = []sdp.Attribute{
		sdp.Attribute{K: "cname", V: "tribemcuserver"},
		sdp.Attribute{K: "msid", V: "tribemcu tribemcuv0"},
		sdp.Attribute{K: "mslabel", V: "tribemcu"},
		sdp.Attribute{K: "label", V: "tribemcuv0"},
	}
	// replay group
	offerMediaVideo.SsrcGroup.Typ = "FID"
	offerMediaVideo.SsrcGroup.SsrcIdList = append(offerMediaVideo.SsrcGroup.SsrcIdList, offerVSsrcId)
	offerMediaVideo.SsrcGroup.SsrcIdList = append(offerMediaVideo.SsrcGroup.SsrcIdList, offerRtxSsrcId)
	//
	offerMediaVideo.PayloadTypes = append(offerMediaVideo.PayloadTypes, videoPayloadType)
	offerMediaVideo.PayloadTypes = append(offerMediaVideo.PayloadTypes, videoPayloadTypeRtx)
	s.offer.Data.Medias = append(s.offer.Data.Medias, offerMediaVideo)

	return
}

// JSON marshaling
type jsonSdpContext struct {
	Offer            string `json:"offer"`
	Answer           string `json:"answer"`
	IceState         string `json:"iceState"`
	IceCandidatePort int    `json:"iceCandidatePort"`
}

func newJsonSdpContext(sdpCtx *SdpContext) jsonSdpContext {
	ctx := getServerStateContext()
	return jsonSdpContext{
		sdpCtx.offer.Write(ctx),
		sdpCtx.answer.Write(ctx),
		sdpCtx.iceState,
		sdpCtx.iceCandidatePort,
	}
}

func (sdpCtx *SdpContext) MarshalJSON() ([]byte, error) {
	return json.Marshal(newJsonSdpContext(sdpCtx))
}
