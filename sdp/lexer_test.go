package sdp_test

import (
	"fmt"
	"testing"

	libsdp "github.com/heytribe/live-webrtcsignaling/sdp"
)

func TestLexerEmptySession(t *testing.T) {
	var s string = ""

	l := libsdp.Lex(s)
	tokens := 0
	for item := range l.Tokens {
		fmt.Printf(item.String())
		tokens++
	}
	if tokens != 0 {
		t.Fatal("should have parsed 0 lines")
	}
}

func TestLexerOK01(t *testing.T) {
	var s string = "v=0\r\n" +
		"o=carol 28908764872 28908764872 IN IP4 100.3.6.6\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"c=IN IP4 192.0.2.4\r\n" +
		"m=audio 0 RTP/AVP 0 1 3\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=rtpmap:1 1016/8000\r\n" +
		"a=rtpmap:3 GSM/8000\r\n" +
		"m=video 0 RTP/AVP 31 34\r\n" +
		"a=rtpmap:31 H261/90000\r\n" +
		"a=rtpmap:34 H263/90000\r\n"

	l := libsdp.Lex(s)
	tokens := 0
	for item := range l.Tokens {
		fmt.Printf(item.String())
		tokens++
	}
	if tokens != 78 {
		t.Fatal("should have parsed 78 tokens %d", tokens)
	}
}

func TestLexerOK02(t *testing.T) {
	var s string = "v=0\r\n" +
		"o=- 20518 0 IN IP4 203.0.113.1\r\n" +
		"s= \r\n" +
		"t=0 0\r\n" +
		"c=IN IP4 203.0.113.1/127/3\r\n" +
		"a=ice-ufrag:F7gI\r\n" +
		"a=ice-pwd:x9cml/YzichV2+XlhiMu8g\r\n" +
		"a=fingerprint:sha-1 42:89:c5:c6:55:9d:6e:c8:e8:83:55:2a:39:f9:b6:eb:e9:a3:a9:e7\r\n" +
		"m=audio 54400 RTP/SAVPF 0 96\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=rtpmap:96 opus/480000\r\n" +
		"a=ptime:20\r\n" +
		"a=sendrecv\r\n" +
		"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\r\n" +
		"a=candidate:1 2 UDP 2113667326 fd97:8b9:6ad2:a146:1c3e:58fd:cf9e:694e 54401 typ host\r\n" +
		"m=video 55400 RTP/SAVPF 97 98\r\n" +
		"a=rtpmap:97 H264/90000\r\n" +
		"a=fmtp:97 profile-level-id=4d0028;packetization-mode=1\r\n" +
		"a=rtpmap:98 VP8/90000\r\n" +
		"a=sendrecv\r\n" +
		"a=candidate:0 1 UDP 2113667327 203.0.113.1 55400 typ host\r\n" +
		"a=candidate:1 2 UDP 2113667326 203.0.113.1 55401 typ host\r\n"

	l := libsdp.Lex(s)
	tokens := 0
	for item := range l.Tokens {
		fmt.Printf(item.String())
		tokens++
	}
	if tokens != 154 {
		t.Fatal("should have parsed 154 tokens %d", tokens)
	}
}

func TestLexerMonkey(t *testing.T) {
	// FIXME: building random generated SDP
}

func TestLexerUnknownFields(t *testing.T) {
	// FIXME
}

func TestLexerUnknownEncoding(t *testing.T) {
	// FIXME
}

func TestLexerMalformedLine(t *testing.T) {
	// FIXME
}

func TestLexerMalformedField(t *testing.T) {
	// FIXME
}

func TestLexerMalformedAttributes(t *testing.T) {
	// FIXME
}
