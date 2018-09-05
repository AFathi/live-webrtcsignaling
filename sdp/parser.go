package sdp

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type parser struct {
	input      string
	lineTokens []token
	line       int
	state      parserStateFn

	lexer          *Lexer
	lexerPrevToken token // simple handling of
	lexerToken     token // backup() & next()
	lexerNextToken token

	section sectionType
	data    *Data

	log Logger
	err error
}

type parserStateFn func(*parser) parserStateFn

// reading next token from the lexer.
func (p *parser) next() token {
	if p.lexerNextToken.typ == tokenEOF {
		// blank next() : calling the lexer
		p.lexerPrevToken = p.lexerToken
		p.lexerToken = p.lexer.NextToken()
		// debug purpose: saving the current line tokens
		if p.lexerToken.typ == tokenEOL {
			p.lineTokens = append(p.lineTokens, p.lexerToken)
			p.log.Debugf(p.prettyPrintTokenLine())
			p.line++
			p.lineTokens = []token{}
		} else {
			p.lineTokens = append(p.lineTokens, p.lexerToken)
		}
	} else {
		// next() after backup() : token from memory
		p.lexerPrevToken = p.lexerToken
		p.lexerToken = p.lexerNextToken
		p.lexerNextToken = p.lexer.NewTokenEOF()
	}
	return p.lexerToken
}

func (p *parser) nextUntil(typ tokenType) []token {
	var tokens []token

	for {
		token := p.next()
		if token.typ == tokenEOF {
			break
		}
		if token.typ == typ {
			p.backup()
			break
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// can be done only once
func (p *parser) backup() {
	p.lexerNextToken = p.lexerToken
	p.lexerToken = p.lexerPrevToken
	p.lexerPrevToken = p.lexer.NewTokenEOF()
}

func (p *parser) addMedia() *Media {
	var media Media
	media.RtpMap = make(map[PayloadType]Rtp)
	media.SsrcMap = make(map[uint32][]Attribute)
	p.data.Medias = append(p.data.Medias, media)
	return p.getCurrentMedia()
}

func (p *parser) getCurrentMedia() *Media {
	if len(p.data.Medias) == 0 {
		return nil
	}
	return &p.data.Medias[len(p.data.Medias)-1]
}

func (p *parser) lex(input string) {
	p.lexer = &Lexer{
		input:  input,
		Tokens: make(chan token),
		line:   1,
		log:    p.log,
	}
	p.lexerPrevToken = p.lexer.NewTokenEOF()
	p.lexerToken = p.lexer.NewTokenEOF()
	p.lexerNextToken = p.lexer.NewTokenEOF()
	go p.lexer.run()
}

func (p *parser) run() (*Data, error) {
	// lexer: split text into token
	p.lex(p.input)
	// analyse tokens
Loop:
	for {
		token := p.next()
		switch {
		case token.typ == tokenEOF:
			break Loop
		case token.typ == tokenField:
			p.backup()
			p.err = parseLine(p)
			if p.err != nil {
				p.log.Errorf(p.prettyPrintErr())
				break Loop
			}
		default:
			p.log.Warnf("not a token field => skip")
		}
	}
	return p.data, p.err
}

func (p *parser) prettyPrintErr() string {
	if p.err == nil {
		return ""
	}
	var tokens string = p.prettyPrintTokenLine()
	return fmt.Sprintf("parser: line(%d)=[%s] err=%s", p.line, tokens, p.err.Error())
}

func (p *parser) prettyPrintTokenLine() string {
	var tokens string
	for i := 0; i < len(p.lineTokens); i++ {
		tokens += fmt.Sprint(p.lineTokens[i].String() + " ")
	}
	return tokens
}

func parseLine(p *parser) error {
	token := p.next()
	if token.typ != tokenField {
		return errors.New("expecting tokenField")
	}
	switch token.val {
	case "v":
		return parseAttributesSessionVersion(p)
	case "o":
		return parseAttributesSessionOrigin(p)
	case "s":
		return parseAttributesSessionName(p)
	case "i":
		return parseAttributesSessionInformation(p)
	case "u":
		return parseAttributesSessionURI(p)
	case "e":
		return parseAttributesSessionEmailAddress(p)
	case "p":
		return parseAttributesSessionPhoneNumber(p)
	case "c":
		if p.section == sectionMedia {
			return parseAttributesMediaConnectionData(p)
		}
		return parseAttributesSessionConnectionData(p)
	case "b":
		if p.section == sectionMedia {
			return parseAttributesMediaBandwidth(p)
		}
		return parseAttributesSessionBandwidth(p)
	case "t":
		p.section = sectionTime
		return parseAttributesTiming(p)
	case "r":
		return parseAttributesRepeatTimes(p)
	case "z":
		return parseAttributesTimeZones(p)
	case "k":
		return parseAttributesSessionEncryptionKeys(p)
	case "a":
		if p.section == sectionMedia {
			return parseAttributesMediaAttribute(p)
		}
		return parseAttributesSessionAttribute(p)
	case "m":
		p.section = sectionMedia
		return parseAttributesMediaDescription(p)
	default:
		return nil
	}
}

// Protocol Version ("v=")
// v=0
// (MANDATORY)
func parseAttributesSessionVersion(p *parser) error {
	var err error

	p.data.Version, err = parseInt(p)
	if err != nil {
		return errors.New("Protocol Version: no token attribute")
	}
	return parseEOL(p)
}

// Origin ("o=")
// o=<username> <sess-id> <sess-version> <nettype> <addrtype>
//        <unicast-address>
// (MANDATORY)
func parseAttributesSessionOrigin(p *parser) error {
	var err error

	p.data.Origin.Username, err = parseWord(p)
	if err != nil {
		return errors.New("Origin username")
	}
	p.data.Origin.SessionId, err = parseInt64(p)
	if err != nil {
		return errors.New("Origin sess-id")
	}
	p.data.Origin.SessionVersion, err = parseInt64(p)
	if err != nil {
		return errors.New("Origin sess-version")
	}
	p.data.Origin.NetType, err = parseWord(p)
	if err != nil {
		return errors.New("Origin nettype")
	}
	p.data.Origin.AddrType, err = parseWord(p)
	if err != nil {
		return errors.New("Origin addrtype")
	}
	p.data.Origin.Address, err = parseWord(p)
	if err != nil {
		return errors.New("Origin unicast-address")
	}
	return parseEOL(p)
}

// Session Name ("s=")
// s=<session name>
// (MANDATORY)
func parseAttributesSessionName(p *parser) error {
	var err error

	p.data.Name, err = parseString(p)
	if err != nil {
		return errors.New("Session name")
	}
	return parseEOL(p)
}

// Session Information ("i=")
// i=<session information>
// (OPTIONAL)
func parseAttributesSessionInformation(p *parser) error {
	var err error

	p.data.Info, err = parseString(p)
	if err != nil {
		return errors.New("Session information")
	}
	return parseEOL(p)
}

// URI ("u=")
// u=<uri>
// (OPTIONAL)
func parseAttributesSessionURI(p *parser) error {
	var err error

	p.data.URI, err = parseString(p)
	if err != nil {
		return errors.New("Session URI")
	}
	return parseEOL(p)
}

// Email Address
// e=<email-address>
// (OPTIONAL)
func parseAttributesSessionEmailAddress(p *parser) error {
	var err error

	p.data.Email, err = parseString(p)
	if err != nil {
		return errors.New("Session email")
	}
	return parseEOL(p)
}

// Phone Number
// p=<phone-number>
// (OPTIONAL)
func parseAttributesSessionPhoneNumber(p *parser) error {
	var err error

	p.data.Phone, err = parseString(p)
	if err != nil {
		return errors.New("Session phone number")
	}
	return parseEOL(p)
}

// Connection Data ("c=")
// c=<nettype> <addrtype> <connection-address>
//  & connection-address = <base multicast address>[/<ttl>]/<number of addresses>
// (OPTIONAL)
func parseAttributesSessionConnectionData(p *parser) error {
	var err error

	p.data.Connection.Nettype, err = parseWord(p)
	if err != nil {
		return errors.New("nettype")
	}
	p.data.Connection.Addrtype, err = parseWord(p)
	if err != nil {
		return errors.New("addrtype")
	}
	//
	addr, v4, ttl, num, err := parseConnectionAddr(p)
	if err != nil {
		return err
	}
	p.data.Connection.Address = addr
	p.data.Connection.IPv4 = v4
	p.data.Connection.TTL = ttl
	p.data.Connection.Num = num
	return parseEOL(p)
}

// Bandwidth ("b=")
// b=<bwtype>:<bandwidth>
func parseAttributesSessionBandwidth(p *parser) error {
	var err error

	p.data.Bandwidth.Bwtype, err = parseWord(p)
	if err != nil {
		return errors.New("Bandwidth type")
	}
	err = parseSeparator(p, ':')
	if err != nil {
		return err
	}
	p.data.Bandwidth.Bw, err = parseInt(p)
	return parseEOL(p)
}

// Timing ("t=")
// t=<start-time> <stop-time>
func parseAttributesTiming(p *parser) error {
	var err error
	var start, stop uint64

	start, err = parseUint64(p)
	if err != nil {
		return errors.New("timing start-time")
	}
	stop, err = parseUint64(p)
	if err != nil {
		return errors.New("timing stop-time")
	}
	p.data.Timing.Start = NTPToTime(start)
	p.data.Timing.Stop = NTPToTime(stop)
	return parseEOL(p)
}

// Repeat Times ("r=")
// r=<repeat interval> <active duration> <offsets from start-time>
func parseAttributesRepeatTimes(p *parser) error {
	var err error

	p.data.TimingRepeat, err = parseString(p)
	if err != nil {
		return errors.New("repeat times")
	}
	return parseEOL(p)
}

// Time Zones ("z=")
// z=<adjustment time> <offset> <adjustment time> <offset> ....
func parseAttributesTimeZones(p *parser) error {
	var err error

	p.data.TimeZones, err = parseString(p)
	if err != nil {
		return errors.New("time zones")
	}
	return parseEOL(p)
}

// Encryption Keys ("k=")
// k=<method>
// k=<method>:<encryption key>
func parseAttributesSessionEncryptionKeys(p *parser) error {
	var err error

	p.data.Encryption.Method, err = parseWord(p)
	if err != nil {
		return errors.New("encryption method")
	}
	if parseSeparator(p, ':') != nil {
		p.data.Encryption.Key, err = parseString(p)
		if err != nil {
			return errors.New("encryption key")
		}
	}
	return parseEOL(p)
}

// session attributes
// Attributes ("a=")
// a=<attribute>
// a=<attribute>:<value>
func parseAttributesSessionAttribute(p *parser) error {
	var attribute string
	var err error

	attribute, err = parseWord(p)
	if err != nil {
		return errors.New("malformed session attribute")
	}
	if parseSeparator(p, ':') != nil {
		// parsing simple: <attribute>
		p.data.Attributes = append(
			p.data.Attributes,
			Attribute{
				K: attribute,
				V: "",
			})
		return parseEOL(p)
	}
	// we are parsing an <attribute><:><value>
	switch attribute {
	case "ice-ufrag":
		p.data.IceUfrag, err = parseAttributeIceUfrag(p)
		if err != nil {
			return err
		}
	case "ice-pwd":
		p.data.IcePwd, err = parseAttributeIcePwd(p)
		if err != nil {
			return err
		}
	case "fingerprint":
		p.data.Fingerprint.Type, p.data.Fingerprint.Hash, err = parseAtributeFingerprint(p)
		if err != nil {
			return err
		}
	default:
		value, err := parseString(p)
		if err != nil {
			return errors.New("session attribute KV")
		}
		p.data.Attributes = append(
			p.data.Attributes,
			Attribute{
				K: attribute,
				V: value,
			})
	}
	return parseEOL(p)
}

// media attributes
// Attributes ("a=")
// a=<attribute>
// a=<attribute><:>[<value>]*
//
// specific attributes are:
func parseAttributesMediaAttribute(p *parser) error {
	var attribute string
	var err error

	attribute, err = parseWord(p)
	if err != nil {
		return errors.New("malformed media attribute")
	}
	media := p.getCurrentMedia()
	if media == nil {
		return errors.New("no media")
	}
	if parseSeparator(p, ':') != nil {
		// parsing simple: <attribute>
		media.Attributes = append(
			media.Attributes,
			Attribute{
				K: attribute,
				V: "",
			})
		return parseEOL(p)
	}
	// we are parsing an <attribute><:><value>
	switch attribute {
	case "ice-ufrag":
		media.IceUfrag, err = parseAttributeIceUfrag(p)
		if err != nil {
			return err
		}
	case "ice-pwd":
		media.IcePwd, err = parseAttributeIcePwd(p)
		if err != nil {
			return err
		}
	case "fingerprint":
		media.Fingerprint.Type, media.Fingerprint.Hash, err = parseAtributeFingerprint(p)
		if err != nil {
			return err
		}
	case "rtpmap":
		// rtpmap:<payload type> <encoding name>/<clock rate> [/<encoding parameters>]
		var payloadType PayloadType
		var payloadTypeInt int

		payloadTypeInt, err = parseInt(p)
		payloadType = PayloadType(payloadTypeInt)
		if err != nil {
			return errors.New("rtpmap: cannot parse payloadType")
		}
		rtp, ok := media.RtpMap[payloadType]
		if ok == false {
			return errors.New(fmt.Sprintf("rtpmap:unknown payloadType %d", payloadTypeInt))
		}
		rtp.Codec, err = parseWord(p)
		if err != nil {
			return errors.New("media attribute rtpmap codec")
		}
		err = parseSeparator(p, '/')
		if err != nil {
			return errors.New("media attribute rtpmap expecting '/'")
		}
		rtp.Rate, err = parseUint32(p)
		if err != nil {
			return errors.New("media attribute rtpmap rate")
		}
		// optional parameter
		err = parseSeparator(p, '/')
		if err == nil {
			rtp.Params, err = parseString(p)
			if err != nil {
				return errors.New("media attribute rtpmap params")
			}
		} else {
			err = nil // optional parameter => no error
		}
		media.RtpMap[rtp.PayloadType] = rtp
	case "fmtp":
		var payloadType PayloadType
		var payloadTypeInt int

		payloadTypeInt, err = parseInt(p)
		payloadType = PayloadType(payloadTypeInt)
		if err != nil {
			return errors.New("fmtp attribute payloadType")
		}
		attributes, err := parseAttributesFMT(p)
		if err != nil {
			if err.Error() == "separator" {
				p.log.Warnf("Parser ignoring fmtp attributes line containing an attribute without value: %#v", attributes)
				break
			}
			return errors.New("fmtp attributes")
		}
		// search inside current media the corresponding rtp
		rtp, ok := media.RtpMap[payloadType]
		if ok == false {
			return errors.New("fmtp payloadType not registered")
		}
		rtp.Fmtp = attributes
		media.RtpMap[payloadType] = rtp
	case "rtcp-fb":
		var rtcpfb string
		var payloadTypeInt int
		var payloadType PayloadType

		token := p.next()
		switch {
		case token.typ == tokenAttributeWord && token.val == "*":
			// we need to apply the config to all the payloadTypes !
			rtcpfb, err = parseString(p)
			if err != nil {
				return errors.New("rtcp-fb value")
			}
			media.RtcpFb = append(media.RtcpFb, rtcpfb)
		case token.typ == tokenAttributeInteger:
			p.backup()
			payloadTypeInt, err = parseInt(p)
			payloadType = PayloadType(payloadTypeInt)
			if err != nil {
				return errors.New("rtcp-fb payloadType")
			}
			rtp, ok := media.RtpMap[payloadType]
			if ok == false {
				return errors.New("rtcp-fb payloadType not registered")
			}
			rtcpfb, err = parseString(p)
			if err != nil {
				return errors.New("rtcp-fb value")
			}
			rtp.RtcpFb = append(rtp.RtcpFb, rtcpfb)
			media.RtpMap[payloadType] = rtp
		default:
			return errors.New("rtcp-fb")
		}
	case "candidate":
		var candidate Candidate

		candidate.Foundation, err = parseWord(p)
		if err != nil {
			return errors.New("candidate attribute foundation")
		}
		candidate.ComponentId, err = parseInt64(p)
		if err != nil {
			return errors.New("candidate attribute component-id")
		}
		candidate.Transport, err = parseWord(p)
		if err != nil {
			return errors.New("candidate attribute transport")
		}
		candidate.Priority, err = parseInt64(p)
		if err != nil {
			return errors.New("candidate attribute priority")
		}
		addr, v4, ttl, num, err := parseConnectionAddr(p)
		if err != nil {
			return err
		}
		candidate.Address = addr
		candidate.IPv4 = v4
		candidate.TTL = ttl
		candidate.Num = num
		candidate.Port, err = parseInt(p)
		if err != nil {
			return errors.New("candidate attribute port")
		}
		w, err := parseWord(p)
		if err != nil || w != "typ" {
			return errors.New("candidate attribute typ")
		}
		candidate.Typ, err = parseWord(p)
		if err != nil {
			return errors.New("candidate attribyte typ val")
		}
		media.Candidates = append(media.Candidates, candidate)
		return parseUntilEOL(p)
	case "ssrc":
		// a=ssrc:<ssrc-id> <attribute>
		// a=ssrc:<ssrc-id> <attribute>:<value>
		var ssrcId uint32
		var ssrcK, ssrcV string

		ssrcId, err = parseUint32(p)
		if err != nil {
			return errors.New("ssrc-id")
		}
		ssrcK, err = parseWord(p)
		if err != nil {
			return errors.New("ssrc-K")
		}
		if parseSeparator(p, ':') == nil {
			ssrcV, err = parseString(p)
			if err != nil {
				return errors.New("ssrc-V")
			}
		}
		// saving the data
		ssrc, _ := media.SsrcMap[ssrcId]
		ssrcAttr := Attribute{K: ssrcK, V: ssrcV}
		ssrc = append(ssrc, ssrcAttr)
		media.SsrcMap[ssrcId] = ssrc
	case "ssrc-group":
		media.SsrcGroup.Typ, err = parseWord(p)
		if err != nil {
			return errors.New("ssrc-group typ")
		}
		for {
			var i uint32

			i, err = parseUint32(p)
			if err != nil {
				if parseEOL(p) == nil {
					p.backup()
					break
				}
				return err
			}
			media.SsrcGroup.SsrcIdList = append(media.SsrcGroup.SsrcIdList, i)
		}
	default:
		value, err := parseString(p)
		if err != nil {
			return errors.New("media attribute KV")
		}
		media.Attributes = append(
			media.Attributes,
			Attribute{
				K: attribute,
				V: value,
			})
	}
	return parseEOL(p)
}

// Media Descriptions ("m=")
// m=<media> <port> <proto> <fmt> ...
func parseAttributesMediaDescription(p *parser) error {
	var err error

	media := p.addMedia()
	media.Type, err = parseWord(p)
	if err != nil {
		return errors.New("media type")
	}
	media.Port, media.NumberOfPorts, err = parseMediaPort(p)
	if err != nil {
		return err
	}
	media.Protocol, err = parseWord(p)
	if err != nil {
		return errors.New("media protocol")
	}
	// contextual lexing
	if strings.Contains(media.Protocol, "RTP/AVP") ||
		strings.Contains(media.Protocol, "RTP/SAVP") {
		// @see https://tools.ietf.org/html/rfc4566#page-24
		// fmt contains payloadTypes
		media.PayloadTypes, err = parsePayloadTypesUntilEOF(p)
		// creating rtpmap using PayloadTypes list.
		for i := 0; i < len(media.PayloadTypes); i++ {
			var rtp Rtp
			rtp.PayloadType = media.PayloadTypes[i]
			rtp.Order = i
			media.RtpMap[rtp.PayloadType] = rtp
		}
	} else {
		media.Fmt, err = parseString(p)
	}
	if err != nil {
		return errors.New("media fmt")
	}
	return parseEOL(p)
}

func parsePayloadTypesUntilEOF(p *parser) ([]PayloadType, error) {
	var payloadTypes []PayloadType
	var payloadType PayloadType
	var err error

	for {
		payloadType, err = parsePayloadType(p)
		if err != nil {
			if parseEOL(p) == nil {
				p.backup() // EOL is pushed back on stack
				return payloadTypes, nil
			}
			return payloadTypes, err
		}
		payloadTypes = append(payloadTypes, payloadType)
	}
}

func parsePayloadType(p *parser) (PayloadType, error) {
	var payloadType PayloadType
	var payloadTypeInt int
	var err error

	payloadTypeInt, err = parseInt(p)
	payloadType = PayloadType(payloadTypeInt)
	if err != nil {
		return 0, errors.New("payloadType")
	}
	return payloadType, nil
}

func parseMediaPort(p *parser) (int, int, error) {
	var port, num int
	var err error

	port, err = parseInt(p)
	if err != nil {
		return 0, 0, errors.New("media port")
	}
	if parseSeparator(p, '/') == nil {
		num, err = parseInt(p)
		if err != nil {
			return 0, 0, errors.New("media port num")
		}
	} else {
		num = 1
	}
	return port, num, nil
}

// Connection Data ("c=")
// c=<nettype> <addrtype> <connection-address>
//  & connection-address = <base multicast address>[/<ttl>]/<number of addresses>
// (OPTIONAL)
func parseAttributesMediaConnectionData(p *parser) error {
	var err error

	media := p.getCurrentMedia()
	if media == nil {
		return errors.New("no media")
	}
	media.Connection.Nettype, err = parseWord(p)
	if err != nil {
		return errors.New("nettype")
	}
	media.Connection.Addrtype, err = parseWord(p)
	if err != nil {
		return errors.New("addrtype")
	}
	//
	addr, v4, ttl, num, err := parseConnectionAddr(p)
	if err != nil {
		return err
	}
	media.Connection.Address = addr
	media.Connection.IPv4 = v4
	media.Connection.TTL = ttl
	media.Connection.Num = num
	return parseEOL(p)
}

// Bandwidth ("b=")
// b=<bwtype>:<bandwidth>
func parseAttributesMediaBandwidth(p *parser) error {
	var err error

	media := p.getCurrentMedia()
	media.Bandwidth.Bwtype, err = parseWord(p)
	if err != nil {
		return errors.New("Bandwidth type")
	}
	err = parseSeparator(p, ':')
	if err != nil {
		return err
	}
	media.Bandwidth.Bw, err = parseInt(p)
	return parseEOL(p)
}

func parseAttributesFMT(p *parser) ([]Attribute, error) {
	var attributes []Attribute

	for {
		token := p.next()
		switch {
		case token.typ == tokenEOF:
			p.backup()
			return attributes, nil
		case token.typ == tokenEOL:
			p.backup()
			return attributes, nil
		case token.typ == tokenSeparator && token.val == ";":
		default:
			p.backup()
			key, err := parseWord(p)
			if err != nil {
				return nil, err
			}
			err = parseSeparator(p, '=')
			if err != nil {
				return []Attribute{{K: key, V: ""}}, err
			}
			val, err := parseWord(p)
			if err != nil {
				return nil, err
			}
			attributes = append(attributes, Attribute{K: key, V: val})
		}
	}
}

// ice-ufrag:F7gI
func parseAttributeIceUfrag(p *parser) (string, error) {
	iceUfrag, err := parseString(p)
	if err != nil {
		return "", errors.New("media attribute ice-ufrag")
	}
	return iceUfrag, nil
}

// ice-pwd:x9cml/YzichV2+XlhiMu8g
func parseAttributeIcePwd(p *parser) (string, error) {
	icePwd, err := parseString(p)
	if err != nil {
		return "", errors.New("media attribute ice-pwd")
	}
	return icePwd, nil
}

// fingerprint:sha-1 42:89:c5:c6(...)
func parseAtributeFingerprint(p *parser) (typ string, hash string, err error) {
	typ, err = parseWord(p)
	if err != nil {
		err = errors.New("media attribute fingerprint typ")
		return
	}
	hash, err = parseString(p)
	if err != nil {
		err = errors.New("media attribute fingerprint hash")
		return
	}
	return
}

func parseConnectionAddr(p *parser) (addr net.IP, v4 bool, ttl int, num int, err error) {
	addr, err = parseAddrIp(p)
	if err != nil {
		return
	}
	v4 = (addr.To4() != nil)
	// parsing optional fields ttl & num
	ttlOrNum, err := parseConnectionAddrTTLOrNum(p)
	if err != nil {
		// optional field => we ignore the error
		err = nil
		return
	}
	num, err = parseConnectionAddrTTLOrNum(p)
	if err != nil {
		// optional field => we ignore the error
		err = nil
		// single ttlOrNum param
		if v4 {
			ttl = ttlOrNum
		} else {
			num = ttlOrNum
		}
	} else {
		if !v4 {
			err = errors.New("unexpected TTL for IPv6")
		} else {
			ttl = ttlOrNum
		}
	}
	return
}

func parseConnectionAddrTTLOrNum(p *parser) (int, error) {
	err := parseSeparator(p, '/')
	if err != nil {
		return 0, err
	}
	return parseInt(p)
}

func parseAddrIp(p *parser) (net.IP, error) {
	word, err := parseWord(p)
	if err != nil {
		return nil, errors.New("addrIp")
	}
	return net.ParseIP(word), nil
}

func parseWord(p *parser) (string, error) {
	token := p.next()
	if token.typ != tokenAttributeWord {
		p.backup()
		return "", errors.New("word")
	}
	return token.val, nil
}

func parseInt(p *parser) (int, error) {
	token := p.next()
	if token.typ != tokenAttributeInteger {
		p.backup()
		return 0, errors.New("integer")
	}
	return strconv.Atoi(token.val)
}

func parseInt64(p *parser) (int64, error) {
	token := p.next()
	if token.typ != tokenAttributeInteger {
		p.backup()
		return 0, errors.New("integer 64")
	}
	return strconv.ParseInt(token.val, 10, 64)
}

func parseUint32(p *parser) (ui uint32, err error) {
	token := p.next()
	if token.typ != tokenAttributeInteger {
		p.backup()
		return 0, errors.New("uinteger 32")
	}
	u, err := strconv.ParseUint(token.val, 10, 32)
	ui = uint32(u)
	return
}

func parseUint64(p *parser) (uint64, error) {
	token := p.next()
	if token.typ != tokenAttributeInteger {
		p.backup()
		return 0, errors.New("uinteger 64")
	}
	return strconv.ParseUint(token.val, 10, 64)
}

func parseString(p *parser) (string, error) {
	token := p.next()
	if token.typ != tokenAttributeString {
		p.backup()
		return "", errors.New("string")
	}
	return token.val, nil
}

func parseSeparator(p *parser, r rune) error {
	token := p.next()
	if token.typ != tokenSeparator || token.val != string(r) {
		p.backup()
		return errors.New("separator")
	}
	return nil
}

func parseUntilEOL(p *parser) error {
	for {
		switch token := p.next(); {
		case token.typ == tokenEOF:
			return nil
		case token.typ == tokenEOL:
			return nil
		default:
			// skip token
		}
	}
}

func parseEOL(p *parser) error {
	token := p.next()
	if token.typ != tokenEOL {
		p.backup()
		return errors.New("EOL")
	}
	return nil
}
