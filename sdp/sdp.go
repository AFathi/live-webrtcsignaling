package sdp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

type PayloadType uint16

type Candidate struct {
	Foundation  string
	ComponentId int64
	Transport   string
	Priority    int64
	//
	Address net.IP
	IPv4    bool // read-only
	TTL     int
	Num     int
	//
	Port int
	Typ  string
}

type Attribute struct {
	K string
	V string
}

type Connection struct {
	Nettype  string
	Addrtype string
	Address  net.IP
	IPv4     bool // read-only
	TTL      int
	Num      int
}

type Bandwidth struct {
	Bwtype string
	Bw     int
}

type Fingerprint struct {
	Type string
	Hash string
}

// Real-time Transport Protocol
// @see https://tools.ietf.org/html/rfc4566#page-25
// a=rtpmap:<payload type> <encoding name>/<clock rate> [/<encoding parameters>]
// payload: integer id
// encoding name: defines the payload format (codec)
// clock rate
// params
type Rtp struct {
	Order int
	//
	PayloadType PayloadType
	Codec       string
	Rate        uint32
	Params      string
	// payload format (codec) parameters
	Fmtp []Attribute
	// immediate feedback mode: don't aggregate packets
	RtcpFb []string
}

type SsrcGroup struct {
	Typ        string
	SsrcIdList []uint32
}

// Media m=... struct with the parameters
// Maybe we should use pointers instead of values for substruct
//  but this struct is not meant to be heavily modified so ..
type Media struct {
	Type          string // ex: audio,video
	Port          int
	NumberOfPorts int
	Protocol      string // ex: UDP/TLS/RTP/SAVPF
	// if protocol = RTP/AVP || RTP/SAVP => PayloadTypes
	PayloadTypes []PayloadType
	// else : unknown string format
	Fmt string
	//
	RtpMap map[PayloadType]Rtp // rtp payload
	RtcpFb []string            // params applied to all payloads

	Connection Connection
	Bandwidth  Bandwidth

	// media attributes
	IceUfrag    string
	IcePwd      string
	Fingerprint Fingerprint

	// additional "raw" fields
	Candidates []Candidate
	Attributes []Attribute
	SsrcMap    map[uint32][]Attribute
	SsrcGroup  SsrcGroup
}

// get dynamic payload type number used for retransmission
// a=fmtp <number>: apt=<apt-value>
func (m *Media) GetDPTNRtx(ptn PayloadType) (PayloadType, error) {
	sptn := fmt.Sprintf("%d", ptn)
	for dptn, rtp := range m.RtpMap {
		for _, attribute := range rtp.Fmtp {
			if attribute.K == "apt" && attribute.V == sptn {
				return dptn, nil
			}
		}
	}
	return 0, errors.New("no dptn")
}

type Data struct {
	// Section session
	Version int
	Origin  struct {
		Username       string
		SessionId      int64 // it is RECOMMENDED that an NTP format timestamp is used
		SessionVersion int64 // it is RECOMMENDED that an NTP format timestamp is used
		NetType        string
		AddrType       string
		Address        string
	}
	Name       string
	Info       string
	URI        string
	Email      string
	Phone      string
	Connection Connection
	Bandwidth  Bandwidth
	TimeZones  string
	Encryption struct {
		Method string
		Key    string
	}
	// session attributes
	IceUfrag    string
	IcePwd      string
	Fingerprint struct {
		Type string
		Hash string
	}
	Attributes []Attribute

	// Section time description
	Timing struct {
		Start time.Time
		Stop  time.Time
	}
	TimingRepeat string

	// Section Media
	Medias []Media
}

func (d *Data) GetFirstMediaVideo() *Media {
	for _, media := range d.Medias {
		if media.Type == "video" {
			return &media
		}
	}
	return nil
}

type SDP struct {
	Data *Data
	log  Logger
	err  error
}

/*
 * GetVideoSSRCList return first media video ssrcId list
 */
func (sdp *SDP) GetVideoSSRCList() []uint32 {
	var result []uint32

	for _, media := range sdp.Data.Medias {
		if media.Type == "video" {
			// if group exist => return group info
			if media.SsrcGroup.Typ == "FID" && len(media.SsrcGroup.SsrcIdList) > 1 {
				return media.SsrcGroup.SsrcIdList
			}
			// return the first one (random) in the map
			for ssrcId, _ := range media.SsrcMap {
				result = append(result, ssrcId)
			}
			return result
		}
	}
	return result
}

/*
 * GetVideoSSRC return first media video first ssrcId
 */
func (sdp *SDP) GetVideoSSRC() uint32 {
	for _, media := range sdp.Data.Medias {
		if media.Type == "video" {
			// if group exist => 1st in the list
			if media.SsrcGroup.Typ == "FID" && len(media.SsrcGroup.SsrcIdList) > 1 {
				return media.SsrcGroup.SsrcIdList[0]
			}
			// return the first one (random) in the map
			for ssrcId, _ := range media.SsrcMap {
				return ssrcId
			}
		}
	}
	return 0
}

func (sdp *SDP) GetVideoPayloadType(codec string) uint16 {
	for _, media := range sdp.Data.Medias {
		if media.Type == "video" {
			for _, m := range media.RtpMap {
				if m.Codec == codec {
					return uint16(m.PayloadType)
				}
			}
		}
	}
	return 0
}

func (sdp *SDP) GetVideoClockRate(codec string) uint32 {
	for _, media := range sdp.Data.Medias {
		if media.Type == "video" {
			for _, m := range media.RtpMap {
				if m.Codec == codec {
					return m.Rate
				}
			}
		}
	}
	return 0
}

/*
 * GetAudioSSRC return first media video first ssrcId
 */
func (sdp *SDP) GetAudioSSRC() uint32 {
	for _, media := range sdp.Data.Medias {
		if media.Type == "audio" {
			// return the first one (random) in the map
			for ssrcId, _ := range media.SsrcMap {
				return ssrcId
			}
		}
	}
	return 0
}

func (sdp *SDP) GetAudioPayloadType(codec string) uint16 {
	for _, media := range sdp.Data.Medias {
		if media.Type == "audio" {
			for _, m := range media.RtpMap {
				if m.Codec == codec {
					return uint16(m.PayloadType)
				}
			}
		}
	}
	return 0
}

func (sdp *SDP) GetAudioClockRate(codec string) uint32 {
	for _, media := range sdp.Data.Medias {
		if media.Type == "audio" {
			for _, m := range media.RtpMap {
				if m.Codec == codec {
					return m.Rate
				}
			}
		}
	}
	return 0
}

func (sdp *SDP) GetRtxPayloadType(codec string) uint16 {
	videoPayloadType := sdp.GetVideoPayloadType(codec)
	for _, media := range sdp.Data.Medias {
		if media.Type == "video" {
			for _, rtp := range media.RtpMap {
				if rtp.Codec == "rtx" &&
					rtp.Fmtp[0].K == "apt" && rtp.Fmtp[0].V == fmt.Sprintf("%d", videoPayloadType) {
					return uint16(rtp.PayloadType)
				}
			}
		}
	}
	return 0
}

/*
 * GetRtxSSRC return first media video replay ssrcId
 */
func (sdp *SDP) GetRtxSSRC() uint32 {
	for _, media := range sdp.Data.Medias {
		if media.Type == "video" {
			// if group exist => 2nd in the list
			if media.SsrcGroup.Typ == "FID" && len(media.SsrcGroup.SsrcIdList) > 1 {
				return media.SsrcGroup.SsrcIdList[1]
			}
		}
	}
	return 0
}

func (sdp *SDP) LoadString(s string) error {
	sdp.Data, sdp.err = sdp.Parse(s)
	return sdp.err
}

func (sdp *SDP) LoadBytes(input []byte) error {
	sdp.Data, sdp.err = sdp.Parse(string(input[:]))
	return sdp.err
}

// the parse function fetch tokens from the lexer &
//   build in-memory struct
func (sdp *SDP) Parse(input string) (*Data, error) {
	p := &parser{
		input: input,
		line:  1,
		data:  new(Data),
		log:   sdp.log,
	}
	return p.run()
}

func (sdp *SDP) Write(ctx context.Context) string {
	if sdp == nil || sdp.Data == nil {
		return ""
	}
	return Write(ctx, sdp.Data)
}

func (sdp *SDP) SetLogger(l Logger) {
	sdp.log = l
}

type Dependencies struct {
	Logger Logger
}

func NewSDP(dep Dependencies) *SDP {
	sdp := new(SDP)
	sdp.log = dep.Logger
	sdp.Data = new(Data)
	//
	sdp.Data.Origin.SessionId = int64(TimeToNTP(time.Now()))
	sdp.Data.Origin.SessionVersion = int64(TimeToNTP(time.Now()))
	return sdp
}
