package sdp

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Pos int

// token represents a token or text string returned from the scanner.
type token struct {
	typ  tokenType // The type of this token.
	pos  Pos       // The starting position, in bytes, of this token in the input string.
	val  string    // The value of this token.
	line int       // The line number at the start of this token.
}

func (i token) String() string {
	switch {
	case i.typ == tokenError:
		return fmt.Sprintf("[E:%s]", i.val)
	case i.typ == tokenEOL:
		return "[EOL]"
	case i.typ == tokenField:
		return fmt.Sprintf("[F:%s]", i.val)
	case i.typ == tokenAttributeInteger:
		return fmt.Sprintf("[A.i:%s]", i.val)
	case i.typ == tokenAttributeWord:
		return fmt.Sprintf("[A.w:%s]", i.val)
	case i.typ == tokenAttributeString:
		return fmt.Sprintf("[A.s:%s]", i.val)
	case i.typ == tokenSeparator:
		return fmt.Sprintf("[S:<%s>]", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

// tokenType identifies the type of lex tokens.
type tokenType int

const (
	none tokenType = iota
	tokenError
	tokenEOF
	tokenEOL
	tokenField            // ex: <v>
	tokenAttributeInteger // ex: <12345>
	tokenAttributeWord    // ex: <fooBAR-_/:.=42>
	tokenAttributeString  // ex: <foo BAR-_/:.=42 w00t!>
	tokenSeparator        // ex: <=> <:> </>
)

type sectionType int

const (
	sectionSession sectionType = iota
	sectionTime
	sectionMedia
)

// lexerStateFn represents the state of the scanner as a function that returns the next state.
type lexerStateFn func(*Lexer) lexerStateFn

// lexer holds the state of the scanner.
type Lexer struct {
	input     string       // the string being scanned
	state     lexerStateFn // the next lexing function to enter
	pos       Pos          // current position in the input
	start     Pos          // start position of this token
	width     Pos          // width of last rune read from input
	Tokens    chan token   // channel of scanned tokens
	line      int          // 1+number of newlines seen
	lastToken token        // last token type
	section   sectionType  // current section
	log       Logger
}

const eof = -1

// next returns the next rune in the input.
func (l *Lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}

	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = Pos(w)
	l.pos += l.width
	if r == '\n' {
		l.line++
	}
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *Lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *Lexer) backup() {
	l.pos -= l.width
	// Correct newline count.
	if l.width == 1 && l.input[l.pos] == '\n' {
		l.line--
	}
}

// emit passes an token back to the client.
func (l *Lexer) emit(t tokenType) {
	l.lastToken = token{t, l.start, l.current(), l.line}
	l.Tokens <- l.lastToken
	l.start = l.pos
}

// return the curent string
func (l *Lexer) current() string {
	return l.input[l.start:l.pos]
}

// ignore skips over the pending input before this point.
func (l *Lexer) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *Lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *Lexer) acceptRun(valid string) int {
	var i int

	for strings.ContainsRune(valid, l.next()) {
		i++
	}
	l.backup()
	return i
}

// emitting an error is not an error :)
func (l *Lexer) emitError(format string, args ...interface{}) {
	a := append([]interface{}{l.line, l.getCurrentLine(), string(l.input[l.pos])}, args...)
	l.log.Warnf("lexer: line(%d)=[%s] token=[%s] err="+format, a...)
	l.lastToken = token{tokenError, l.start, fmt.Sprintf(format, args...), l.line}
	l.Tokens <- l.lastToken
}

func (l *Lexer) getCurrentLine() string {
	var begin, pos Pos

	// search the begining
	for begin = l.pos; begin >= 0 && l.input[begin] != '\n'; begin-- {
	}
	begin++
	// search the end
	for pos = begin; ; {
		if int(pos) > len(l.input) {
			break
		}
		r, w := utf8.DecodeRuneInString(l.input[pos:])
		if r == '\n' || r == '\r' {
			return l.input[begin:pos]
		}
		pos += Pos(w)
	}
	return ""
}

// this function should be called by the parser
func (l *Lexer) NextToken() token {
	token, alive := <-l.Tokens
	if !alive {
		return l.NewTokenEOF()
	}
	return token
}

func (l *Lexer) NewTokenEOF() token {
	return token{tokenEOF, 0, "", 0}
}

func Lex(input string) *Lexer {
	l := &Lexer{
		input:  input,
		Tokens: make(chan token),
		line:   1,
		log:    NewConsoleLogger(),
	}
	go l.run()
	return l
}

// run runs the state machine for the Lexer.
func (l *Lexer) run() {
	for l.state = lexLine; l.state != nil; {
		l.state = l.state(l)
	}
	close(l.Tokens)
}

//
// This function "lex" a single line,
// line format is  <field> = <attributes>
//   ex: v=0 => emit token field v, token integer 0, token EOL
//   ex: o=carol 4224 4242 IN IP4 127.0.0.1
//         => emit token field o, token word carol, token integer 4224
//                 token integer 4242, token word IN, token word IP4
//                 token word 127.0.0.1
//
// how does it work ?
// l.next() get the next rune (utf8 letter) from the input
//  if EOF => exit
//  if EOL => strip EOL characters
//  if other => "lex" field & "lex" attributes
//     depending on the field, we "lex" the attributes
//     with a different function (contextual Lexer)
//
func lexLine(l *Lexer) lexerStateFn {
	for {
		r := l.next()
		switch {
		case r == eof:
			return nil
		case isEndOfLine(r):
			return lexEOL
		case l.lastToken.typ != 0 && l.lastToken.typ != tokenEOL:
			// no need to backup, we are greedy :)
			l.emitError("can't lex middle of line, skipping line")
			return lexSkipLine
		default:
			// scan the field
			l.backup()
			if !l.scanField() {
				l.emitError("malformed field, skipping line")
				return lexSkipLine
			}
			l.emit(tokenField)
			// check the equal symbol
			if !l.scanSeparator('=') {
				l.emitError("expecting '=', skipping line")
				return lexSkipLine
			}
			l.ignore()
			// scan the attributes
			switch r {
			case 'v':
				// Protocol Version ("v=")
				// v=0
				l.scanIntegerAndEmit()
			case 'o':
				// Origin ("o=")
				// o=<username> <sess-id> <sess-version> <nettype> <addrtype> <unicast-address>
				l.scanWordAndEmit()
				l.eatSpaces()
				l.scanIntegerAndEmit()
				l.eatSpaces()
				l.scanIntegerAndEmit()
				l.eatSpaces()
				l.scanWordsAndEmit()
			case 's':
				// Session Name ("s=")
				// s=<session name>
				l.scanStringAndEmit()
			case 'i':
				// Session Information ("i=")
				// i=<session information>  (section session)
				// i=<media title>          (section media)
				l.scanStringAndEmit()
			case 'u':
				// URI ("u=")
				// u=<uri>
				l.scanStringAndEmit()
			case 'e':
				// Email Address
				// e=<email-address>
				l.scanStringAndEmit()
			case 'p':
				// Phone Number ("e=" and "p=")
				// p=<phone-number>
				l.scanStringAndEmit()
			case 'c':
				// Connection Data ("c=")
				// c=<nettype> <addrtype> <connection-address>
				//  & connection-address = <base multicast address>[/<ttl>]/<number of addresses>
				l.scanWordAndEmit()
				l.eatSpaces()
				l.scanWordAndEmit()
				l.eatSpaces()
				l.scanConnectionAddressAndEmit()
			case 'b':
				// Bandwidth ("b=")
				// b=<bwtype>:<bandwidth>   (section session & section media)
				l.scanAlphaNumAndEmit("-")
				l.scanSeparatorAndEmit(':')
				l.scanIntegerAndEmit()
			case 't':
				// Timing ("t=")
				// t=<start-time> <stop-time>
				l.scanIntegerAndEmit()
				l.eatSpaces()
				l.scanIntegerAndEmit()
			case 'r':
				// Repeat Times ("r=")
				// r=<repeat interval> <active duration> <offsets from start-time>
				// FIXME: improve the parsing !
				l.scanStringAndEmit()
			case 'z':
				// Time Zones ("z=")
				// z=<adjustment time> <offset> <adjustment time> <offset> ....
				// FIXME: improve the parsing !
				l.scanStringAndEmit()
			case 'k':
				// Encryption Keys ("k=")
				// k=<method>                   (section session & section media)
				// k=<method>:<encryption key>  (section session & section media)
				l.scanAlphaNumAndEmit("-")
				switch n := l.next(); {
				case n == ':':
					l.emit(tokenSeparator)
					l.scanStringAndEmit()
				default:
					l.backup()
				}
			case 'a':
				// session attributes
				// Attributes ("a=")
				// a=<attribute>                 (section session & section media)
				// a=<attribute>:<value>         (section session & section media)
				att := l.scanAlphaNumAndEmit("-")
				switch n := l.next(); {
				case n == ':':
					l.emit(tokenSeparator)
					switch att {
					case "ice-ufrag":
						// ice-ufrag:F7gI
						l.scanStringAndEmit()
					case "ice-pwd":
						// ice-pwd:x9cml/YzichV2+XlhiMu8g
						l.scanStringAndEmit()
					case "fingerprint":
						// fingerprint:sha-1 42:89:c5:c6(...)
						l.scanWordAndEmit()
						l.eatSpaces()
						l.scanStringAndEmit()
					case "rtpmap":
						// rtpmap:<payload type> <encoding name>/<clock rate> [/<encoding parameters>]
						l.scanIntegerAndEmit()
						l.eatSpaces()
						l.scanAlphaNumAndEmit("-")
						l.scanSeparatorAndEmit('/')
						l.scanIntegerAndEmit()
						switch o := l.next(); {
						case o == '/':
							l.emit(tokenSeparator)
							l.scanStringAndEmit()
						default:
							l.backup()
						}
					case "fmtp":
						l.scanFmtpAndEmit()
					case "rtcp-fb":
						l.scanRtcpFbAndEmit()
					case "candidate":
						l.scanCandidateAndEmit()
					case "ssrc":
						l.scanIntegerAndEmit()
						l.eatSpaces()
						l.scanAlphaNumAndEmit("")
						switch o := l.next(); {
						case o == ':':
							l.emit(tokenSeparator)
							l.scanStringAndEmit()
						default:
							l.backup()
						}
					case "ssrc-group":
						l.scanWordAndEmit()
						l.eatSpaces()
						l.scanIntegersAndEmit()
					default:
						l.scanStringAndEmit()
					}
				default:
					l.backup()
				}
			case 'm':
				// Media Descriptions ("m=")
				// m=<media> <port> <proto> <fmt> ...
				l.scanWordAndEmit()
				l.eatSpaces()
				l.scanIntegerAndEmit()
				switch n := l.next(); {
				case n == '/':
					l.emit(tokenSeparator)
					l.scanIntegerAndEmit()
				default:
					l.backup()
				}
				l.eatSpaces()
				l.scanWordAndEmit()
				proto := l.lastToken.val
				l.eatSpaces()
				if strings.Contains(proto, "RTP/AVP") ||
					strings.Contains(proto, "RTP/SAVP") {
					l.scanIntegersAndEmit()
				} else {
					l.scanStringAndEmit()
				}
			default:
				l.scanStringAndEmit()
			}
			return lexLine
		}
	}
}

func lexEOL(l *Lexer) lexerStateFn {
	l.scanEOL()
	l.ignore()
	l.emit(tokenEOL)
	return lexLine
}

func lexSkipLine(l *Lexer) lexerStateFn {
	for {
		switch r := l.next(); {
		case r == eof:
			return nil
		case isEndOfLine(r):
			return lexLine
		}
	}
}

//
// SCANNER
//

// scan* funcs :
// - move the end cursor according to rule
// - keep the start cursor in place
// - return true  : >1 caracter scanned & cursor start<end
// - return false :  0 caracter scanned & cursor start=end
func (l *Lexer) scanInteger() bool {
	return l.acceptRun("0123456789") > 0
}

func (l *Lexer) scanField() bool {
	return l.accept("vosiuepcbzkatrmicbka")
}

func (l *Lexer) scanSeparator(separator rune) bool {
	return l.accept(string(separator))
}

func (l *Lexer) scanSpaces() {
	for isSpace(l.next()) {
	}
	l.backup()
}

func (l *Lexer) scanEOL() {
	for isEndOfLine(l.next()) {
	}
	l.backup()
}

func (l *Lexer) scanAlphaNum(valid string) bool {
	var i int

	for {
		r := l.next()
		if !isAlphaNumeric(r) && !strings.ContainsRune(valid, r) {
			break
		}
		i++
	}
	l.backup()
	return i != 0
}

func (l *Lexer) scanWord() bool {
	return l.scanAlphaNum("-_/:.=")
}

func (l *Lexer) scanString() bool {
	var i int

Loop:
	for {
		switch r := l.next(); {
		case r == eof:
			break Loop
		case isEndOfLine(r):
			break Loop
		}
		i++
	}
	l.backup()
	return i != 0
}

// eat funcs :
// - move the end cursor
// - place the start cursor at the end cursor position
func (l *Lexer) eatSpaces() {
	l.scanSpaces()
	l.ignore()
}

// scan & emit funcs :
// - scan (move end cursor)
// - emit token if scan was successful
// - emit error if scan failed
// - after return, start cursor = end cursor
func (l *Lexer) scanSeparatorAndEmit(r rune) {
	if l.scanSeparator(r) {
		l.emit(tokenSeparator)
	} else {
		l.emitError("await separator <%s>", string(r))
	}
}

func (l *Lexer) scanIntegerAndEmit() {
	if l.scanInteger() {
		l.emit(tokenAttributeInteger)
	} else {
		l.emitError("integer")
	}
}

func (l *Lexer) scanIntegersAndEmit() {
Loop:
	for {
		l.scanIntegerAndEmit()
		switch r := l.peek(); {
		case isSpace(r):
			l.eatSpaces()
		case isEndOfLine(r):
			break Loop
		case r == eof:
			break Loop
		default:
			l.emitError("cannot parse '%s' (not an integer)", string(r))
			break Loop
		}
	}
}

func (l *Lexer) scanAlphaNumAndEmit(additional string) string {
	var c string

	if l.scanAlphaNum(additional) {
		c = l.current()
		l.emit(tokenAttributeWord)
	} else {
		l.emitError("alphaNum")
	}
	return c
}

func (l *Lexer) scanWordAndEmit() {
	if l.scanWord() {
		l.emit(tokenAttributeWord)
	} else {
		l.emitError("word")
	}
}

func (l *Lexer) scanWordsAndEmit() {
Loop:
	for {
		l.scanWordAndEmit()
		switch r := l.peek(); {
		case isSpace(r):
			l.eatSpaces()
		case isEndOfLine(r):
			break Loop
		case r == eof:
			break Loop
		default:
			l.emitError("cannot parse '%s' (not a word)", string(r))
			break Loop
		}
	}
}

func (l *Lexer) scanStringAndEmit() {
	if l.scanString() {
		l.emit(tokenAttributeString)
	} else {
		l.emitError("string")
	}
}

// https://tools.ietf.org/html/rfc4566#section-9
// ABNF:
// connection-address =  multicast-address / unicast-address
// multicast-address =   IP4-multicast / IP6-multicast / FQDN / extn-addr
// IP4-multicast =       m1 3( "." decimal-uchar )
//                       "/" ttl [ "/" integer ]
// IP6-multicast =       hexpart [ "/" integer ]
//                       ; IPv6 address starting with FF
// hexpart =             hexseq / hexseq "::" [ hexseq ] /
//                       "::" [ hexseq ]
// hexseq  =             hex4 *( ":" hex4)
// hex4    =             1*4HEXDIG
// m1 =                  ("22" ("4"/"5"/"6"/"7"/"8"/"9")) /
//                       ("23" DIGIT )
// decimal-uchar =       DIGIT
//                       / POS-DIGIT DIGIT
//                       / ("1" 2*(DIGIT))
//                       / ("2" ("0"/"1"/"2"/"3"/"4") DIGIT)
//                       / ("2" "5" ("0"/"1"/"2"/"3"/"4"/"5"))
// ttl =                 (POS-DIGIT *2DIGIT) / "0"
// POS-DIGIT =           %x31-39 ; 1 - 9
func (l *Lexer) scanConnectionAddressAndEmit() {
	if l.scanAlphaNum(".:") {
		l.emit(tokenAttributeWord)
	} else {
		l.emitError("connection-address")
		return
	}
	switch n := l.next(); {
	case n == '/':
		l.emit(tokenSeparator)
		l.scanIntegerAndEmit()
	default:
		l.backup()
	}
	switch n := l.next(); {
	case n == '/':
		l.emit(tokenSeparator)
		l.scanIntegerAndEmit()
	default:
		l.backup()
	}
}

// https://tools.ietf.org/html/rfc4585
// rtcp-fb-syntax = "a=rtcp-fb:" rtcp-fb-pt SP rtcp-fb-val CRLF
//
// rtcp-fb-pt         = "*"   ; wildcard: applies to all formats
//                    / fmt   ; as defined in SDP spec
//
// rtcp-fb-val        = "ack" rtcp-fb-ack-param
//                    / "nack" rtcp-fb-nack-param
//                    / "trr-int" SP 1*DIGIT
//                    / rtcp-fb-id rtcp-fb-param
//
// rtcp-fb-id         = 1*(alpha-numeric / "-" / "_")
//
// rtcp-fb-param      = SP "app" [SP byte-string]
//                    / SP token [SP byte-string]
//                    / ; empty
//
// rtcp-fb-ack-param  = SP "rpsi"
//                    / SP "app" [SP byte-string]
//                    / SP token [SP byte-string]
//                    / ; empty
//
// rtcp-fb-nack-param = SP "pli"
//                    / SP "sli"
//                    / SP "rpsi"
//                    / SP "app" [SP byte-string]
//                    / SP token [SP byte-string]
//                    / ; empty
func (l *Lexer) scanRtcpFbAndEmit() {
	switch l.peek() {
	case '*':
		l.scanWordAndEmit() // all format (payloads) !
	default:
		l.scanIntegerAndEmit()
	}
	l.eatSpaces()
	l.scanStringAndEmit() // FIXME: we might need to dig this in the future.
}

// https://tools.ietf.org/html/rfc5245#section-15.1
// candidate-attribute   = "candidate" ":" foundation SP component-id SP
// 												transport SP
// 												priority SP
// 												connection-address SP     ;from RFC 4566
// 												port         ;port from RFC 4566
// 												SP cand-type
// 												[SP rel-addr]
// 												[SP rel-port]
// 												*(SP extension-att-name SP
// 														 extension-att-value)
//
// foundation            = 1*32ice-char
// component-id          = 1*5DIGIT
// transport             = "UDP" / transport-extension
// transport-extension   = token              ; from RFC 3261
// priority              = 1*10DIGIT
// cand-type             = "typ" SP candidate-types
// candidate-types       = "host" / "srflx" / "prflx" / "relay" / token
// rel-addr              = "raddr" SP connection-address
// rel-port              = "rport" SP port
// extension-att-name    = byte-string    ;from RFC 4566
// extension-att-value   = byte-string
// ice-char              = ALPHA / DIGIT / "+" / "/"
//
// from RFC 4566
// byte-string =         1*(%x01-09/%x0B-0C/%x0E-FF)
//                       ;any byte except NUL, CR, or LF
//
// We will ignore extension-att-name & extension-att-value
// because byte-string definition include separator SP !!
//
// Also, there are problem in this grammar :
// https://www.ietf.org/mail-archive/web/mmusic/current/msg17220.html
// a candidate must either have both rel-addr and rel-port (if it’s reflexive or relay), or neither (if it’s host).
// the grammar should be : [SP rel-addr SP rel-port].
//
func (l *Lexer) scanCandidateAndEmit() {
	// mandatory
	l.scanAlphaNumAndEmit("+/")      // foundation, ice-char = ALPHA / DIGIT / "+" / "/"
	l.eatSpaces()                    //
	l.scanIntegerAndEmit()           // component-id
	l.eatSpaces()                    //
	l.scanWordAndEmit()              // transport: UDP|token from RFC 3261
	l.eatSpaces()                    //
	l.scanIntegerAndEmit()           // priority
	l.eatSpaces()                    //
	l.scanConnectionAddressAndEmit() // connection-address from RFC 4566
	l.eatSpaces()                    //
	l.scanIntegerAndEmit()           // port
	l.eatSpaces()                    //
	l.scanAlphaNumAndEmit("")        // candidate-types: "typ"
	l.eatSpaces()                    //
	l.scanAlphaNumAndEmit("")        // candidate-types: "host" / "srflx" / "prflx" / "relay" / token
	// optional fields
	//
Loop:
	for {
		switch r := l.next(); {
		case isSpace(r):
			l.eatSpaces()
		case isEndOfLine(r):
			l.backup()
			break Loop
		case r == eof:
			l.backup()
			break Loop
		default:
			l.backup()
			l.scanWordAndEmit()
			w := l.current()
			switch w {
			case "raddr":
				l.eatSpaces()
				l.scanConnectionAddressAndEmit()
			case "rport":
				l.eatSpaces()
				l.scanIntegerAndEmit()
			default:
				l.eatSpaces()
				l.scanWordAndEmit()
			}
		}
	}
}

// https://tools.ietf.org/html/rfc4566#section-9
// a=fmtp:<format> <format specific parameters>
//  The <format> parameter MUST be one of
//   the media formats (i.e., RTP payload types) specified for the media
//   stream.  The meaning of the <format specific parameters> is unique
//   for each media type.  This parameter MUST only be used for media
//   types for which source-level format parameters have explicitly been
//   specified; media-level format parameters MUST NOT be carried over
//   blindly.
//
// FIXME: we only implement VP8 / Opus format ...
//  a=fmtp:111 minptime=10;useinbandfec=1
//  a=fmtp:97 apt=96
// others:
//  a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
// iOS:
//  a=fmtp:111 maxplaybackrate=16000; sprop-maxcapturerate=16000; maxaveragebitrate=20000; stereo=1; useinbandfec=1; usedtx=0
func (l *Lexer) scanFmtpAndEmit() {
	// mandatory
	l.scanIntegerAndEmit()
	l.eatSpaces()
	// loop
	n := 0
Loop:
	for {
		switch r := l.next(); {
		case isEndOfLine(r):
			l.backup()
			break Loop
		case r == eof:
			l.backup()
			break Loop
		default:
			l.backup()
			if n > 0 {
				l.scanSeparatorAndEmit(';')
			}
			n++
			l.eatSpaces()
			l.scanAlphaNumAndEmit("-_/:.")
			if l.scanSeparator('=') {
				l.emit(tokenSeparator)
				l.scanAlphaNumAndEmit("-_/:.")
			}
		}
	}
}

//
// UTILS
//
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

func isAlphaNumeric(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
