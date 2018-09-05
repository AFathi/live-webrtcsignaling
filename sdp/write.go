package sdp

import (
	"context"
	"fmt"
	"strings"

	plogger "github.com/heytribe/go-plogger"
)

type res struct {
	lines []string
}

func (r *res) push(s string, args ...interface{}) {
	r.lines = append(r.lines, fmt.Sprintf(s, args...))
}

func (r *res) get() string {
	return strings.Join(r.lines, "\r\n") + "\r\n"
}

func Write(ctx context.Context, sdp *Data) string {
	var r res

	log := plogger.FromContextSafe(ctx)

	// section session
	// mandatory fields & attributes
	r.push("v=%d", sdp.Version)
	r.push("o=%s %d %d %s %s %s",
		sdp.Origin.Username,
		sdp.Origin.SessionId,
		sdp.Origin.SessionVersion,
		sdp.Origin.NetType,
		sdp.Origin.AddrType,
		sdp.Origin.Address)
	r.push("s=%s", sdp.Name)
	// optionnal fields & attributes
	if len(sdp.Info) > 0 {
		r.push("i=%s", sdp.Info)
	}
	if len(sdp.Email) > 0 {
		r.push("e=%s", sdp.Email)
	}
	if len(sdp.Phone) > 0 {
		r.push("p=%s", sdp.Phone)
	}
	if len(sdp.Connection.Nettype) > 0 &&
		len(sdp.Connection.Addrtype) > 0 &&
		len(sdp.Connection.Address) > 0 {
		var ttl, num string

		if sdp.Connection.TTL > 0 {
			ttl = fmt.Sprintf("/%d", sdp.Connection.TTL)
		}
		if sdp.Connection.Num > 0 {
			num = fmt.Sprintf("/%d", sdp.Connection.Num)
		}
		r.push("c=%s %s %s%s%s",
			sdp.Connection.Nettype,
			sdp.Connection.Addrtype,
			sdp.Connection.Address,
			ttl,
			num)
	}
	if len(sdp.Bandwidth.Bwtype) > 0 {
		r.push("b=%s:%d",
			sdp.Bandwidth.Bwtype,
			sdp.Bandwidth.Bw)
	}
	if len(sdp.TimeZones) > 0 {
		r.push("z=%s", sdp.TimeZones)
	}
	if len(sdp.Encryption.Method) > 0 {
		if sdp.Encryption.Method == "prompt" {
			r.push("k=prompt")
		} else {
			r.push("k=%s:%s", sdp.Encryption.Method, sdp.Encryption.Key)
		}
	}
	// session specific attributes
	if len(sdp.IceUfrag) != 0 {
		r.push("a=ice-ufrag:%s", sdp.IceUfrag)
	}
	if len(sdp.IcePwd) != 0 {
		r.push("a=ice-pwd:%s", sdp.IcePwd)
	}
	if len(sdp.Fingerprint.Type) != 0 && len(sdp.Fingerprint.Hash) != 0 {
		r.push("a=fingerprint:%s %s", sdp.Fingerprint.Type, sdp.Fingerprint.Hash)
	}

	// section timing
	r.push("t=%d %d",
		TimeToNTP(sdp.Timing.Start),
		TimeToNTP(sdp.Timing.Stop))
	if len(sdp.TimingRepeat) > 0 {
		r.push("r=" + sdp.TimingRepeat)
	}

	for _, attribute := range sdp.Attributes {
		if len(attribute.V) == 0 {
			r.push("a=%s", attribute.K)
		} else {
			r.push("a=%s:%v", attribute.K, attribute.V)
		}
	}

	// section media
	for _, media := range sdp.Medias {
		var fmtOrPayloadTypes string

		if strings.Contains(media.Protocol, "RTP/AVP") ||
			strings.Contains(media.Protocol, "RTP/SAVP") {
			// using payloads
			var payloadsStrings []string
			for _, payload := range media.PayloadTypes {
				payloadsStrings = append(payloadsStrings, fmt.Sprintf("%d", payload))
			}
			fmtOrPayloadTypes = strings.Join(payloadsStrings, " ")
		} else {
			fmtOrPayloadTypes = media.Fmt
		}
		if media.NumberOfPorts > 1 {
			r.push("m=%s %d/%d %s %s",
				media.Type, media.Port, media.NumberOfPorts,
				media.Protocol, fmtOrPayloadTypes)
		} else {
			r.push("m=%s %d %s %s",
				media.Type, media.Port,
				media.Protocol, fmtOrPayloadTypes)
		}
		//
		if len(media.Connection.Nettype) > 0 &&
			len(media.Connection.Addrtype) > 0 &&
			len(media.Connection.Address) > 0 {
			var ttl, num string

			if media.Connection.TTL > 0 {
				ttl = fmt.Sprintf("/%d", media.Connection.TTL)
			}
			if media.Connection.Num > 0 {
				num = fmt.Sprintf("/%d", media.Connection.Num)
			}
			r.push("c=%s %s %s%s%s",
				media.Connection.Nettype,
				media.Connection.Addrtype,
				media.Connection.Address,
				ttl,
				num)
		}
		if len(media.Bandwidth.Bwtype) > 0 {
			r.push("b=%s:%d",
				media.Bandwidth.Bwtype,
				media.Bandwidth.Bw)
		}
		if len(media.IceUfrag) != 0 {
			r.push("a=ice-ufrag:%s", media.IceUfrag)
		}
		if len(media.IcePwd) != 0 {
			r.push("a=ice-pwd:%s", media.IcePwd)
		}
		if len(media.Fingerprint.Type) != 0 && len(media.Fingerprint.Hash) != 0 {
			r.push("a=fingerprint:%s %s", media.Fingerprint.Type, media.Fingerprint.Hash)
		}
		// building an orderd list of rtp address
		var orderedRtpList = make([]Rtp, len(media.RtpMap))
		for _, rtp := range media.RtpMap {
			if rtp.Order >= len(media.RtpMap) {
				log.Errorf("write: cannot save rtp with order %d , len=%d, rtp %v", rtp.Order, len(media.RtpMap), rtp)
			} else {
				orderedRtpList[rtp.Order] = rtp
			}
		}

		for _, rtp := range orderedRtpList {
			if len(rtp.Params) > 0 {
				r.push("a=rtpmap:%d %s/%d/%s", rtp.PayloadType, rtp.Codec, rtp.Rate, rtp.Params)
			} else {
				r.push("a=rtpmap:%d %s/%d", rtp.PayloadType, rtp.Codec, rtp.Rate)
			}
			if len(rtp.Fmtp) != 0 {
				fmtpAttributes := ""
				for z, attr := range rtp.Fmtp {
					if z > 0 {
						fmtpAttributes += ";"
					}
					fmtpAttributes += attr.K + "=" + attr.V
				}
				r.push("a=fmtp:%d %s", rtp.PayloadType, fmtpAttributes)
			}
			for _, rtcpFb := range rtp.RtcpFb {
				r.push("a=rtcp-fb:%d %s", rtp.PayloadType, rtcpFb)
			}
		}
		// globals rtcp-fb
		for _, rtcpFb := range media.RtcpFb {
			r.push("a=rtcp-fb:* %s", rtcpFb)
		}

		for _, attribute := range media.Attributes {
			if len(attribute.V) == 0 {
				r.push("a=%s", attribute.K)
			} else {
				r.push("a=%s:%v", attribute.K, attribute.V)
			}
		}

		for _, candidate := range media.Candidates {
			var ttl, num string

			if candidate.TTL > 0 {
				ttl = fmt.Sprintf("/%d", sdp.Connection.TTL)
			}
			if candidate.Num > 0 {
				num = fmt.Sprintf("/%d", sdp.Connection.Num)
			}
			r.push("a=candidate:%s %d %s %d %s%s%s %d typ %s",
				candidate.Foundation, candidate.ComponentId,
				candidate.Transport, candidate.Priority,
				candidate.Address, ttl, num,
				candidate.Port,
				candidate.Typ)
		}
		r.push("a=end-of-candidates")

		if media.SsrcGroup.Typ == "FID" && len(media.SsrcGroup.SsrcIdList) == 2 {
			r.push("a=ssrc-group:FID %d %d", media.SsrcGroup.SsrcIdList[0], media.SsrcGroup.SsrcIdList[1])

			for _, ssrcId := range media.SsrcGroup.SsrcIdList {
				for _, attribute := range media.SsrcMap[ssrcId] {
					if attribute.V == "" {
						r.push("a=ssrc:%d %s", ssrcId, attribute.K)
					} else {
						r.push("a=ssrc:%d %s:%s", ssrcId, attribute.K, attribute.V)
					}
				}
			}
		} else {
			for ssrcId, attributes := range media.SsrcMap {
				for _, attribute := range attributes {
					if attribute.V == "" {
						r.push("a=ssrc:%d %s", ssrcId, attribute.K)
					} else {
						r.push("a=ssrc:%d %s:%s", ssrcId, attribute.K, attribute.V)
					}
				}
			}
		}
	}

	return r.get()
}
