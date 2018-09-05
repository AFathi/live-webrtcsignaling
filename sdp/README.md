# Description

SDP reader & writer

@see RFC4566: https://tools.ietf.org/html/rfc4566  
@see https://www.ietf.org/rfc/rfc3264.txt  
@see https://tools.ietf.org/html/rfc5245  (ICE)  
@see https://tools.ietf.org/html/draft-ietf-ice-rfc5245bis-11 (ICE, draft, remplace la RFC 5245)  
@see https://tools.ietf.org/html/draft-ietf-ice-trickle-14 (ICE trickle, draft)  
@see https://tools.ietf.org/html/rfc4585#section-4  
@see https://tools.ietf.org/html/draft-ietf-mmusic-sdp-bundle-negotiation-39  Param Bundle   

SDP anatomy :  
https://webrtchacks.com/anatomy-webrtc-sdp/  
https://webrtchacks.com/sdp-anatomy/  
https://www.w3.org/TR/webrtc/#call-flow-browser-to-browser  WebRTC workflow

ICE overview :
https://www.slideshare.net/saghul/ice-4414037

sdp overview :  

```
Session description
    v=  (protocol version)
    o=  (originator and session identifier)
    s=  (session name)
    i=* (session information)
    u=* (URI of description)
    e=* (email address)
    p=* (phone number)
    c=* (connection information -- not required if included in
         all media)
    b=* (zero or more bandwidth information lines)
    One or more time descriptions ("t=" and "r=" lines; see below)
    z=* (time zone adjustments)
    k=* (encryption key)
    a=* (zero or more session attribute lines)
    Zero or more media descriptions

 Time description
    t=  (time the session is active)
    r=* (zero or more repeat times)

 Media description, if present
    m=  (media name and transport address)
    i=* (media title)
    c=* (connection information -- optional if included at
         session level)
    b=* (zero or more bandwidth information lines)
    k=* (encryption key)
    a=* (zero or more media attribute lines)
```

SDP Grammar (ABNF) : https://tools.ietf.org/html/rfc4566#section-9  

Candidate Grammar (ABNF) : https://tools.ietf.org/html/rfc5245#section-15.1  

```
candidate-attribute   = "candidate" ":" foundation SP component-id SP
                        transport SP
                        priority SP
                        connection-address SP     ;from RFC 4566
                        port         ;port from RFC 4566
                        SP cand-type
                        [SP rel-addr]
                        [SP rel-port]
                        *(SP extension-att-name SP
                             extension-att-value)

foundation            = 1*32ice-char
component-id          = 1*5DIGIT
transport             = "UDP" / transport-extension
transport-extension   = token              ; from RFC 3261
priority              = 1*10DIGIT
cand-type             = "typ" SP candidate-types
candidate-types       = "host" / "srflx" / "prflx" / "relay" / token
rel-addr              = "raddr" SP connection-address
rel-port              = "rport" SP port
extension-att-name    = byte-string    ;from RFC 4566
extension-att-value   = byte-string
ice-char              = ALPHA / DIGIT / "+" / "/"
```

rtcp-fb Grammar (ABNF): https://tools.ietf.org/html/rfc4585#section-4

```
rtcp-fb-syntax = "a=rtcp-fb:" rtcp-fb-pt SP rtcp-fb-val CRLF

rtcp-fb-pt         = "*"   ; wildcard: applies to all formats
                   / fmt   ; as defined in SDP spec

rtcp-fb-val        = "ack" rtcp-fb-ack-param
                   / "nack" rtcp-fb-nack-param
                   / "trr-int" SP 1*DIGIT
                   / rtcp-fb-id rtcp-fb-param

rtcp-fb-id         = 1*(alpha-numeric / "-" / "_")

rtcp-fb-param      = SP "app" [SP byte-string]
                   / SP token [SP byte-string]
                   / ; empty

rtcp-fb-ack-param  = SP "rpsi"
                   / SP "app" [SP byte-string]
                   / SP token [SP byte-string]
                   / ; empty

rtcp-fb-nack-param = SP "pli"
                   / SP "sli"
                   / SP "rpsi"
                   / SP "app" [SP byte-string]
                   / SP token [SP byte-string]
                   / ; empty
```

ssrc-group grammar https://tools.ietf.org/html/rfc5576#section-4.2

```
ssrc-group-attr = "ssrc-group:" semantics *(SP ssrc-id)

semantics       = "FEC" / "FID" / token
                 ; Matches RFC 3388 definition and
                 ; IANA registration rules in this doc.
token           = 1*(token-char)
token-char      = %x21 / %x23-27 / %x2A-2B / %x2D-2E / %x30-39 / %x41-5A / %x5E-7E
```

# Usage

## Read

### LoadString

load the sdp from a string

```go
import ("github.com/heytribe/live-webrtcsignaling/sdp")

var offer string = "v=0\r\n"

sdp := sdp.NewSDP(sdp.Dependencies{Logger:logger})
err := sdp.LoadString(offer)
```

### FromBytes

load the sdp from a byte array

```go
import ("github.com/heytribe/live-webrtcsignaling/sdp")

sdp := sdp.NewSDP(sdp.Dependencies{Logger:logger})
err := sdp.LoadBytes(byteArray)
```

## Write

```go
var answer string = sdp.Write()
```

## Example

```go
import ("github.com/heytribe/live-webrtcsignaling/sdp")

sdp := sdp.NewSDP(sdp.Dependencies{Logger:logger})
err := sdp.FromBytes(byteArray)
if err != nil {
  // whatever
}
// we can read info
fmt.Printf(sdp.Data.Version)
fmt.Printf(sdp.Data.Name)
fmt.Printf(sdp.Data.Bandwidth.Bw)
// we can set info
sdp.Data.Bandwidth.Bwtype = "X-YZ"
sdp.Data.Bandwidth.Bw = 4096
// we can write info
answer := sdp.Write()
```

# Internals

The implementation is inspired by the talk "Lexical Scanning in Go" by Rob Pike:
 - https://www.youtube.com/watch?v=HxaD_trXwRE
 - https://golang.org/src/text/template/parse/lex.go
 - https://talks.golang.org/2011/lex.slide#1

The lexical scanner is composed of a Lexer & a Parser.

The Lexer is somehow contextual, because the syntax is mainly: field=attributes. Attributes are "lexed" depending on field & previous fields.

The lexer feeds "Tokens" to the parser.

The parser is a recursive descent parser that does not require backtracking. we assume the SDP attribute grammar is LL(1)

Note: because some of the logic is duplicate between the lexer and the parser, we maybe should have limited the code to a recursive descent parser without a lexer.

Additionnals info:  
https://softwareengineering.stackexchange.com/questions/337676/what-is-the-procedure-that-is-followed-when-writing-a-lexer-based-upon-a-grammar


# TODO

- improve logs
- beeing more "resilient"

- improve parsing of r=,z=,k=
- improve parsing of: session attributes
- improve parsing of: media attributes
- improve parsing of: repeat times, time zones

- implement tests
- add monkey test, shouldn't timeout

- pass metrics to constructor & implement metrics
- refactor: global encoding check on parsing : restrict to default ISO-10646 char set in utf-8.
- refactor: replace scanWord with scanToken & respect the ABNF grammar charcodes...

- defensive code: for loops: we should panic > 100 loop.

- check RFC's fields "MUST" conditions to generate correct errors & better Warning logs
- check session-level + media-level fields duplicates


# Examples

## chromium

### Publisher

offer
```
v=0
o=- 8864026617769031906 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE audio video
a=msid-semantic: WMS agzScspNJkaPW705c6ooKfyhrQNVe6c1B2Xq
m=audio 45579 UDP/TLS/RTP/SAVPF 111 103 104 9 0 8 106 105 13 110 112 113 126
c=IN IP4 172.18.0.1
a=rtcp:9 IN IP4 0.0.0.0
a=candidate:1051995033 1 udp 2122260223 172.18.0.1 45579 typ host generation 0 network-id 1 network-cost 50
a=candidate:678820566 1 udp 2122194687 192.168.0.19 39728 typ host generation 0 network-id 2 network-cost 10
a=ice-ufrag:3182
a=ice-pwd:edjxV2/H1VVclnmBfGJ0rpjS
a=ice-options:trickle
a=fingerprint:sha-256 91:0E:4B:C3:6E:CD:BF:61:32:7F:7A:99:34:C4:6E:AD:3A:20:E2:AD:80:64:3B:04:A8:00:A7:09:FF:19:F3:77
a=setup:actpass
a=mid:audio
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=sendrecv
a=rtcp-mux
a=rtpmap:111 opus/48000/2
a=rtcp-fb:111 transport-cc
a=fmtp:111 minptime=10;useinbandfec=1
a=rtpmap:103 ISAC/16000
a=rtpmap:104 ISAC/32000
a=rtpmap:9 G722/8000
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:106 CN/32000
a=rtpmap:105 CN/16000
a=rtpmap:13 CN/8000
a=rtpmap:110 telephone-event/48000
a=rtpmap:112 telephone-event/32000
a=rtpmap:113 telephone-event/16000
a=rtpmap:126 telephone-event/8000
a=ssrc:2644864907 cname:b2GNMqAR0r1p3MCv
a=ssrc:2644864907 msid:agzScspNJkaPW705c6ooKfyhrQNVe6c1B2Xq 54e0485b-b83d-4f77-9964-75b9fea5d7f7
a=ssrc:2644864907 mslabel:agzScspNJkaPW705c6ooKfyhrQNVe6c1B2Xq
a=ssrc:2644864907 label:54e0485b-b83d-4f77-9964-75b9fea5d7f7
m=video 39496 UDP/TLS/RTP/SAVPF 96 97 98 99 100 101 102 124 127 123 125
c=IN IP4 172.18.0.1
a=rtcp:9 IN IP4 0.0.0.0
a=candidate:1051995033 1 udp 2122260223 172.18.0.1 39496 typ host generation 0 network-id 1 network-cost 50
a=candidate:678820566 1 udp 2122194687 192.168.0.19 40415 typ host generation 0 network-id 2 network-cost 10
a=ice-ufrag:3182
a=ice-pwd:edjxV2/H1VVclnmBfGJ0rpjS
a=ice-options:trickle
a=fingerprint:sha-256 91:0E:4B:C3:6E:CD:BF:61:32:7F:7A:99:34:C4:6E:AD:3A:20:E2:AD:80:64:3B:04:A8:00:A7:09:FF:19:F3:77
a=setup:actpass
a=mid:video
a=extmap:2 urn:ietf:params:rtp-hdrext:toffset
a=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
a=extmap:4 urn:3gpp:video-orientation
a=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01
a=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay
a=extmap:7 http://www.webrtc.org/experiments/rtp-hdrext/video-content-type
a=extmap:8 http://www.webrtc.org/experiments/rtp-hdrext/video-timing
a=sendrecv
a=rtcp-mux
a=rtcp-rsize
a=rtpmap:96 VP8/90000
a=rtcp-fb:96 goog-remb
a=rtcp-fb:96 transport-cc
a=rtcp-fb:96 ccm fir
a=rtcp-fb:96 nack
a=rtcp-fb:96 nack pli
a=rtpmap:97 rtx/90000
a=fmtp:97 apt=96
a=rtpmap:98 VP9/90000
a=rtcp-fb:98 goog-remb
a=rtcp-fb:98 transport-cc
a=rtcp-fb:98 ccm fir
a=rtcp-fb:98 nack
a=rtcp-fb:98 nack pli
a=rtpmap:99 rtx/90000
a=fmtp:99 apt=98
a=rtpmap:100 H264/90000
a=rtcp-fb:100 goog-remb
a=rtcp-fb:100 transport-cc
a=rtcp-fb:100 ccm fir
a=rtcp-fb:100 nack
a=rtcp-fb:100 nack pli
a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f
a=rtpmap:101 rtx/90000
a=fmtp:101 apt=100
a=rtpmap:102 H264/90000
a=rtcp-fb:102 goog-remb
a=rtcp-fb:102 transport-cc
a=rtcp-fb:102 ccm fir
a=rtcp-fb:102 nack
a=rtcp-fb:102 nack pli
a=fmtp:102 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
a=rtpmap:124 rtx/90000
a=fmtp:124 apt=102
a=rtpmap:127 red/90000
a=rtpmap:123 rtx/90000
a=fmtp:123 apt=127
a=rtpmap:125 ulpfec/90000
a=ssrc-group:FID 1112045517 278547195
a=ssrc:1112045517 cname:b2GNMqAR0r1p3MCv
a=ssrc:1112045517 msid:agzScspNJkaPW705c6ooKfyhrQNVe6c1B2Xq 4a7c021b-5a5d-417b-863f-63d6c9b7b346
a=ssrc:1112045517 mslabel:agzScspNJkaPW705c6ooKfyhrQNVe6c1B2Xq
a=ssrc:1112045517 label:4a7c021b-5a5d-417b-863f-63d6c9b7b346
a=ssrc:278547195 cname:b2GNMqAR0r1p3MCv
a=ssrc:278547195 msid:agzScspNJkaPW705c6ooKfyhrQNVe6c1B2Xq 4a7c021b-5a5d-417b-863f-63d6c9b7b346
a=ssrc:278547195 mslabel:agzScspNJkaPW705c6ooKfyhrQNVe6c1B2Xq
a=ssrc:278547195 label:4a7c021b-5a5d-417b-863f-63d6c9b7b346
```

answer
```
v=0
o=- 2038416329560424690 2 IN IP4 192.168.1.35
s=Tribe MCU
i=Tribe MCU Server
t=0 0
a=group:BUNDLE audio video
m=audio 45579 UDP/TLS/RTP/SAVPF 111
c=IN IP4 192.168.1.35
b=AS:32
a=ice-ufrag:MJcx
a=ice-pwd:CXUlMxsFULoZHPyhCXUlMc
a=fingerprint:sha-256 22:AD:4A:A7:A8:BD:4C:E2:72:63:CC:C2:F0:78:CC:9D:B9:4E:09:C7:97:C8:F9:A1:03:AC:AA:F8:50:34:B7:02
a=rtpmap:111 opus/48000/2
a=fmtp:111 minptime=10;useinbandfec=1
a=rtcp-fb:111 transport-cc
a=recvonly
a=mid:audio
a=rtcp-mux
a=ice-options:trickle
a=setup:active
a=candidate:1 1 udp 2130706432 192.168.1.35 60687 typ host
a=end-of-candidates
m=video 39496 UDP/TLS/RTP/SAVPF 102 124
c=IN IP4 192.168.1.35
b=AS:128
a=ice-ufrag:MJcx
a=ice-pwd:CXUlMxsFULoZHPyhCXUlMc
a=fingerprint:sha-256 22:AD:4A:A7:A8:BD:4C:E2:72:63:CC:C2:F0:78:CC:9D:B9:4E:09:C7:97:C8:F9:A1:03:AC:AA:F8:50:34:B7:02
a=rtpmap:102 H264/90000
a=fmtp:102 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
a=rtcp-fb:102 goog-remb
a=rtcp-fb:102 ccm fir
a=rtcp-fb:102 nack
a=rtcp-fb:102 nack pli
a=rtpmap:124 rtx/90000
a=fmtp:124 apt=102
a=recvonly
a=mid:video
a=rtcp-mux
a=ice-options:trickle
a=setup:active
a=candidate:1 1 udp 2130706432 192.168.1.35 60687 typ host
a=end-of-candidates
```
