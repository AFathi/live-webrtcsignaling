package sdp_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/heytribe/live-webrtcsignaling/sdp"
)

func TestSDP(t *testing.T) {
	var s string = "v=0\r\n" +
		"o=- 20518 0 IN IP4 203.0.113.1\r\n" +
		"s= \r\n" +
		"c=IN IP4 203.0.113.1/127/3\r\n" +
		"b=X-YZ:128\n" +
		"a=ice-ufrag:F7gI\r\n" +
		"a=ice-pwd:x9cml/YzichV2+XlhiMu8g\r\n" +
		"a=fingerprint:sha-1 42:89:c5:c6:55:9d:6e:c8:e8:83:55:2a:39:f9:b6:eb:e9:a3:a9:e7\r\n" +
		"t=0r 0\r\n" +
		"m=audio 54400 RTP/SAVPF 0 96\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=rtpmap:96 opus/48000\r\n" +
		"a=ptime:20\r\n" +
		"a=sendrecv\r\n" +
		"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\r\n" +
		"a=candidate:1 2 UDP 2113667326 203.0.113.1 54401 typ host\r\n" +
		"m=video 55400 RTP/SAVPF 97 98\r\n" +
		"a=rtpmap:97 H264/90000\r\n" +
		"a=fmtp:97 profile-level-id=4d0028;packetization-mode=1\r\n" +
		"a=rtpmap:98 VP8/90000\r\n" +
		"a=sendrecv\r\n" +
		"a=candidate:0 1 UDP 2113667327 203.0.113.1 55400 typ host\r\n" +
		"a=candidate:1 2 UDP 2113667326 203.0.113.1 55401 typ host\r\n"

	sdp := sdp.NewSDP(sdp.Dependencies{Logger: new(testLogger)})
	err := sdp.LoadBytes([]byte(s))
	if err != nil {
		fmt.Print("------------------------------------\n")
		fmt.Print("IN:\n")
		fmt.Print(s)
		fmt.Print("------------------------------------\n")
		fmt.Print("\n")
		fmt.Print("------------------------------------\n")
		fmt.Print("OUT:\n")
		fmt.Printf("%s\n", sdp.Write(context.Background()))
		fmt.Print("------------------------------------\n")

		t.Fatal(err)
	}
}

func TestSDPChromiumMarc(t *testing.T) {
	// marc: SDP envoyé par chromium depuis linux
	var s string = "v=0\r\n" +
		"o=- 9143854556127760863 2 IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"a=group:BUNDLE audio video\r\n" +
		"a=msid-semantic: WMS i4riEPmhHSOPK4y359LDJ9aNdIBggUV9F5Cr\r\n" +
		// RTP/SAVPF => rtp profile n°4
		// 96
		"m=audio 9 UDP/TLS/RTP/SAVPF 111 103 104 9 0 8 106 105 13 110 112 113 126\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"a=rtcp:9 IN IP4 0.0.0.0\r\n" +
		"a=ice-ufrag:wwVB\r\n" +
		"a=ice-pwd:i4CgvsPILtxv2xo2OaIdjkIP\r\n" +
		"a=ice-options:trickle\r\n" +
		"a=fingerprint:sha-256 4C:15:04:C7:F6:15:FC:4C:B4:11:DE:91:E7:D0:35:BE:52:47:A2:69:EC:D4:85:C6:9C:F0:EA:8C:A6:83:B2:E7\r\n" +
		"a=setup:actpass\r\n" +
		"a=mid:audio\r\n" +
		"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level\r\n" +
		"a=sendonly\r\n" +
		"a=rtcp-mux\r\n" +
		"a=rtpmap:111 opus/48000/2\r\n" +
		// immediate feedback mode:
		// option en gros pas attendre d'aggréger les packets rtcp-fb
		// renvoyer chaque info tout de suite.
		"a=rtcp-fb:111 transport-cc\r\n" +
		// parametres du format: qu'est ce que le codec permet d'avoir en extension
		"a=fmtp:111 minptime=10;useinbandfec=1\r\n" +
		"a=rtpmap:103 ISAC/16000\r\n" +
		"a=rtpmap:104 ISAC/32000\r\n" +
		"a=rtpmap:9 G722/8000\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n" +
		"a=rtpmap:106 CN/32000\r\n" +
		"a=rtpmap:105 CN/16000\r\n" +
		"a=rtpmap:13 CN/8000\r\n" +
		"a=rtpmap:110 telephone-event/48000\r\n" +
		"a=rtpmap:112 telephone-event/32000\r\n" +
		"a=rtpmap:113 telephone-event/16000\r\n" +
		"a=rtpmap:126 telephone-event/8000\r\n" +

		// SSRC: cname: / label:
		//  identifie le source ID qui est ce qu'on retrouve dans les packets RTP
		//  id unique par canal.
		"a=ssrc:2104746148 cname:IYdHJSLcZMxrFsxK\r\n" +
		"a=ssrc:2104746148 msid:i4riEPmhHSOPK4y359LDJ9aNdIBggUV9F5Cr 225baad5-9113-4701-8ab6-9d2ae1e7d8e7\r\n" +
		"a=ssrc:2104746148 mslabel:i4riEPmhHSOPK4y359LDJ9aNdIBggUV9F5Cr\r\n" +
		"a=ssrc:2104746148 label:225baad5-9113-4701-8ab6-9d2ae1e7d8e7\r\n" +

		"m=video 9 UDP/TLS/RTP/SAVPF 96 98 100 102 127 97 99 101 125\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"a=rtcp:9 IN IP4 0.0.0.0\r\n" +
		"a=ice-ufrag:wwVB\r\n" +
		"a=ice-pwd:i4CgvsPILtxv2xo2OaIdjkIP\r\n" +
		"a=ice-options:trickle\r\n" +
		"a=fingerprint:sha-256 4C:15:04:C7:F6:15:FC:4C:B4:11:DE:91:E7:D0:35:BE:52:47:A2:69:EC:D4:85:C6:9C:F0:EA:8C:A6:83:B2:E7\r\n" +
		"a=setup:actpass\r\n" +
		"a=mid:video\r\n" +
		"a=extmap:2 urn:ietf:params:rtp-hdrext:toffset\r\n" +
		"a=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\n" +
		"a=extmap:4 urn:3gpp:video-orientation\r\n" +
		"a=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\r\n" +
		"a=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay\r\n" +
		"a=sendonly\r\n" +
		"a=rtcp-mux\r\n" +
		"a=rtcp-rsize\r\n" +
		"a=rtpmap:96 VP8/90000\r\n" +
		"a=rtcp-fb:96 ccm fir\r\n" +
		"a=rtcp-fb:96 nack\r\n" +
		"a=rtcp-fb:96 nack pli\r\n" +
		"a=rtcp-fb:96 goog-remb\r\n" +
		"a=rtcp-fb:96 transport-cc\r\n" +
		"a=rtpmap:98 VP9/90000\r\n" +
		"a=rtcp-fb:98 ccm fir\r\n" +
		"a=rtcp-fb:98 nack\r\n" +
		"a=rtcp-fb:98 nack pli\r\n" +
		"a=rtcp-fb:98 goog-remb\r\n" +
		"a=rtcp-fb:98 transport-cc\r\n" +
		"a=rtpmap:100 H264/90000\r\n" +
		"a=rtcp-fb:100 ccm fir\r\n" +
		"a=rtcp-fb:100 nack\r\n" +
		"a=rtcp-fb:100 nack pli\r\n" +
		"a=rtcp-fb:100 goog-remb\r\n" +
		"a=rtcp-fb:100 transport-cc\r\n" +
		"a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\n" +
		"a=rtpmap:102 red/90000\r\n" +
		"a=rtpmap:127 ulpfec/90000\r\n" +
		"a=rtpmap:97 rtx/90000\r\n" +
		"a=fmtp:97 apt=96\r\n" +
		"a=rtpmap:99 rtx/90000\r\n" +
		"a=fmtp:99 apt=98\r\n" +
		"a=rtpmap:101 rtx/90000\r\n" +
		"a=fmtp:101 apt=100\r\n" +
		"a=rtpmap:125 rtx/90000\r\n" +
		"a=fmtp:125 apt=102\r\n" +

		"a=ssrc-group:FID 723811301 1712662157\r\n" +
		"a=ssrc:723811301 cname:IYdHJSLcZMxrFsxK\r\n" +
		"a=ssrc:723811301 msid:i4riEPmhHSOPK4y359LDJ9aNdIBggUV9F5Cr df8de0ad-df16-4f0e-8cf7-d56822d8c9b5\r\n" +
		"a=ssrc:723811301 mslabel:i4riEPmhHSOPK4y359LDJ9aNdIBggUV9F5Cr\r\n" +
		"a=ssrc:723811301 label:df8de0ad-df16-4f0e-8cf7-d56822d8c9b5\r\n" +
		"a=ssrc:1712662157 cname:IYdHJSLcZMxrFsxK\r\n" +
		"a=ssrc:1712662157 msid:i4riEPmhHSOPK4y359LDJ9aNdIBggUV9F5Cr df8de0ad-df16-4f0e-8cf7-d56822d8c9b5\r\n" +
		"a=ssrc:1712662157 mslabel:i4riEPmhHSOPK4y359LDJ9aNdIBggUV9F5Cr\r\n" +
		"a=ssrc:1712662157 label:df8de0ad-df16-4f0e-8cf7-d56822d8c9b5\r\n"

	sdp := sdp.NewSDP(sdp.Dependencies{Logger: new(testLogger)})
	err := sdp.LoadBytes([]byte(s))
	if err != nil {
		fmt.Print("------------------------------------\n")
		fmt.Print("IN:\n")
		fmt.Print(s)
		fmt.Print("------------------------------------\n")
		fmt.Print("\n")
		fmt.Print("------------------------------------\n")
		fmt.Print("OUT:\n")
		//fmt.Printf("%+v\n", sdp.Data)
		fmt.Printf("%s\n", sdp.Write(context.Background()))
		//fmt.Print("\n")
		//fmt.Printf("%# v\n", pretty.Formatter(sdp.Data))
		fmt.Print("------------------------------------\n")

		t.Fatal(err)
	}
}

func TestSDPChromiumMarc2(t *testing.T) {
	// marc: SDP envoyé par chromium depuis linux
	var s string = "v=0\r\n" +
		"o=- 2409965372822901634 2 IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n" +
		"a=group:BUNDLE audio video\r\n" +
		"a=msid-semantic: WMS beHX0fvWjJjSQBLgaBsh5lNAN2h5lGfOdLoH\r\n" +
		"m=audio 49128 UDP/TLS/RTP/SAVPF 111 103 104 9 0 8 106 105 13 110 112 113 126\r\n" +
		"c=IN IP4 172.18.0.1\r\n" +
		"a=rtcp:9 IN IP4 0.0.0.0\r\n" +
		"a=candidate:1051995033 1 udp 2122260223 172.18.0.1 49128 typ host generation 0 network-id 1 network-cost 50\r\n" +
		"a=candidate:316526476 1 udp 2122194687 192.168.0.20 51289 typ host generation 0 network-id 2 network-cost 10\r\n" +
		"a=ice-ufrag:ecwp\r\n" +
		"a=ice-pwd:8echQd9jHqownT1n4pg0owO1\r\n" +
		"a=ice-options:trickle\r\n" +
		"a=fingerprint:sha-256 76:2D:BC:64:9E:93:46:16:1C:4B:0E:0C:DC:79:CE:48:40:5F:34:CB:56:38:18:AA:59:8E:C5:5D:C3:5D:F6:EF\r\n" +
		"a=setup:actpass\r\n" +
		"a=mid:audio\r\n" +
		"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level\r\n" +
		"a=sendonly\r\n" +
		"a=rtcp-mux\r\n" +
		"a=rtpmap:111 opus/48000/2\r\n" +
		"a=rtcp-fb:111 transport-cc\r\n" +
		"a=fmtp:111 minptime=10;useinbandfec=1\r\n" +
		"a=rtpmap:103 ISAC/16000\r\n" +
		"a=rtpmap:104 ISAC/32000\r\n" +
		"a=rtpmap:9 G722/8000\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n" +
		"a=rtpmap:106 CN/32000\r\n" +
		"a=rtpmap:105 CN/16000\r\n" +
		"a=rtpmap:13 CN/8000\r\n" +
		"a=rtpmap:110 telephone-event/48000\r\n" +
		"a=rtpmap:112 telephone-event/32000\r\n" +
		"a=rtpmap:113 telephone-event/16000\r\n" +
		"a=rtpmap:126 telephone-event/8000\r\n" +
		"a=ssrc:3971311055 cname:3ZLITCJ2AWaaOEsx\r\n" +
		"a=ssrc:3971311055 msid:beHX0fvWjJjSQBLgaBsh5lNAN2h5lGfOdLoH bc4ec4ce-8fb6-4a19-8f5e-1ec9df596460\r\n" +
		"a=ssrc:3971311055 mslabel:beHX0fvWjJjSQBLgaBsh5lNAN2h5lGfOdLoH\r\n" +
		"a=ssrc:3971311055 label:bc4ec4ce-8fb6-4a19-8f5e-1ec9df596460\r\n" +
		"m=video 39574 UDP/TLS/RTP/SAVPF 96 98 100 102 127 97 99 101 125\r\n" +
		"c=IN IP4 172.18.0.1\r\n" +
		"a=rtcp:9 IN IP4 0.0.0.0\r\n" +
		"a=candidate:1051995033 1 udp 2122260223 172.18.0.1 39574 typ host generation 0 network-id 1 network-cost 50\r\n" +
		"a=candidate:316526476 1 udp 2122194687 192.168.0.20 48656 typ host generation 0 network-id 2 network-cost 10\r\n" +
		"a=ice-ufrag:ecwp\r\n" +
		"a=ice-pwd:8echQd9jHqownT1n4pg0owO1\r\n" +
		"a=ice-options:trickle\r\n" +
		"a=fingerprint:sha-256 76:2D:BC:64:9E:93:46:16:1C:4B:0E:0C:DC:79:CE:48:40:5F:34:CB:56:38:18:AA:59:8E:C5:5D:C3:5D:F6:EF\r\n" +
		"a=setup:actpass\r\n" +
		"a=mid:video\r\n" +
		"a=extmap:2 urn:ietf:params:rtp-hdrext:toffset\r\n" +
		"a=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\n" +
		"a=extmap:4 urn:3gpp:video-orientation\r\n" +
		"a=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\r\n" +
		"a=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay\r\n" +
		"a=sendonly\r\n" +
		"a=rtcp-mux\r\n" +
		"a=rtcp-rsize\r\n" +
		"a=rtcp-rsize\r\n" +
		"a=rtpmap:96 VP8/90000\r\n" +
		"a=rtcp-fb:96 ccm fir\r\n" +
		"a=rtcp-fb:96 nack\r\n" +
		"a=rtcp-fb:96 nack pli\r\n" +
		"a=rtcp-fb:96 goog-remb\r\n" +
		"a=rtcp-fb:96 transport-cc\r\n" +
		"a=rtpmap:98 VP9/90000\r\n" +
		"a=rtcp-fb:98 ccm fir\r\n" +
		"a=rtcp-fb:98 nack\r\n" +
		"a=rtcp-fb:98 nack pli\r\n" +
		"a=rtcp-fb:98 goog-remb\r\n" +
		"a=rtcp-fb:98 transport-cc\r\n" +
		"a=rtpmap:100 H264/90000\r\n" +
		"a=rtcp-fb:100 ccm fir\r\n" +
		"a=rtcp-fb:100 nack\r\n" +
		"a=rtcp-fb:100 nack pli\r\n" +
		"a=rtcp-fb:100 goog-remb\r\n" +
		"a=rtcp-fb:100 transport-cc\r\n" +
		"a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\n" +
		"a=rtpmap:102 red/90000\r\n" +
		"a=rtpmap:127 ulpfec/90000\r\n" +
		"a=rtpmap:97 rtx/90000\r\n" +
		"a=fmtp:97 apt=96\r\n" +
		"a=rtpmap:99 rtx/90000\r\n" +
		"a=fmtp:99 apt=98\r\n" +
		"a=rtpmap:101 rtx/90000\r\n" +
		"a=fmtp:101 apt=100\r\n" +
		"a=rtpmap:125 rtx/90000\r\n" +
		"a=fmtp:125 apt=102\r\n" +
		"a=ssrc-group:FID 2662865155 3954252979\r\n" +
		"a=ssrc:2662865155 cname:3ZLITCJ2AWaaOEsx\r\n" +
		"a=ssrc:2662865155 msid:beHX0fvWjJjSQBLgaBsh5lNAN2h5lGfOdLoH cf83321f-674f-489d-bc94-5e8e9324a106\r\n" +
		"a=ssrc:2662865155 mslabel:beHX0fvWjJjSQBLgaBsh5lNAN2h5lGfOdLoH\r\n" +
		"a=ssrc:2662865155 label:cf83321f-674f-489d-bc94-5e8e9324a106\r\n" +
		"a=ssrc:3954252979 cname:3ZLITCJ2AWaaOEsx\r\n" +
		"a=ssrc:3954252979 msid:beHX0fvWjJjSQBLgaBsh5lNAN2h5lGfOdLoH cf83321f-674f-489d-bc94-5e8e9324a106\r\n" +
		"a=ssrc:3954252979 mslabel:beHX0fvWjJjSQBLgaBsh5lNAN2h5lGfOdLoH\r\n" +
		"a=ssrc:3954252979 label:cf83321f-674f-489d-bc94-5e8e9324a106\r\n"

	sdp := sdp.NewSDP(sdp.Dependencies{Logger: new(testLogger)})
	err := sdp.LoadBytes([]byte(s))
	if err != nil {
		fmt.Print("------------------------------------\n")
		fmt.Print("IN:\n")
		fmt.Print(s)
		fmt.Print("------------------------------------\n")
		fmt.Print("\n")
		fmt.Print("------------------------------------\n")
		fmt.Print("OUT:\n")
		//fmt.Printf("%+v\n", sdp.Data)
		fmt.Printf("%s\n", sdp.Write(context.Background()))
		//fmt.Print("\n")
		//fmt.Printf("%# v\n", pretty.Formatter(sdp.Data))
		fmt.Print("------------------------------------\n")

		t.Fatal(err)
	}
}

func TestSDPFirefox(t *testing.T) {
	var s string = `v=0
o=mozilla...THIS_IS_SDPARTA-57.0.4 2942585659596354419 0 IN IP4 0.0.0.0
s=-
t=0 0
a=sendrecv
a=fingerprint:sha-256 9E:49:3F:D2:8F:CB:A4:7B:5D:77:08:D7:EE:09:95:69:06:5C:30:17:78:67:C8:BD:4A:19:8D:9C:B9:4C:C9:B7
a=group:BUNDLE sdparta_0 sdparta_1
a=ice-options:trickle
a=msid-semantic:WMS *
m=audio 9 UDP/TLS/RTP/SAVPF 109 9 0 8 101
c=IN IP4 0.0.0.0
a=sendrecv
a=extmap:1/sendonly urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=fmtp:109 maxplaybackrate=48000;stereo=1;useinbandfec=1
a=fmtp:101 0-15
a=ice-pwd:7d812845a4cc9184007589785e536eda
a=ice-ufrag:53e03b42
a=mid:sdparta_0
a=msid:{0d4ff97f-c714-5c44-816f-44c3932bfa17} {217002c2-5196-a847-a527-3b8db40d2451}
a=rtcp-mux
a=rtpmap:109 opus/48000/2
a=rtpmap:9 G722/8000/1
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:101 telephone-event/8000
a=setup:actpass
a=ssrc:1961433925 cname:{31d59695-f309-ac45-8b9e-c0b054f9014e}
m=video 9 UDP/TLS/RTP/SAVPF 120 121 126 97
c=IN IP4 0.0.0.0
a=sendrecv
a=extmap:1 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
a=extmap:2 urn:ietf:params:rtp-hdrext:toffset
a=fmtp:126 profile-level-id=42e01f;level-asymmetry-allowed=1;packetization-mode=1
a=fmtp:97 profile-level-id=42e01f;level-asymmetry-allowed=1
a=fmtp:120 max-fs=12288;max-fr=60
a=fmtp:121 max-fs=12288;max-fr=60
a=ice-pwd:7d812845a4cc9184007589785e536eda
a=ice-ufrag:53e03b42
a=mid:sdparta_1
a=msid:{0d4ff97f-c714-5c44-816f-44c3932bfa17} {ce28efba-fb7c-1f44-9b13-764dd1f4ced1}
a=rtcp-fb:120 nack
a=rtcp-fb:120 nack pli
a=rtcp-fb:120 ccm fir
a=rtcp-fb:120 goog-remb
a=rtcp-fb:121 nack
a=rtcp-fb:121 nack pli
a=rtcp-fb:121 ccm fir
a=rtcp-fb:121 goog-remb
a=rtcp-fb:126 nack
a=rtcp-fb:126 nack pli
a=rtcp-fb:126 ccm fir
a=rtcp-fb:126 goog-remb
a=rtcp-fb:97 nack
a=rtcp-fb:97 nack pli
a=rtcp-fb:97 ccm fir
a=rtcp-fb:97 goog-remb
a=rtcp-mux
a=rtpmap:120 VP8/90000
a=rtpmap:121 VP9/90000
a=rtpmap:126 H264/90000
a=rtpmap:97 H264/90000
a=setup:actpass
a=ssrc:3263075240 cname:{31d59695-f309-ac45-8b9e-c0b054f9014e}
`

	sdp := sdp.NewSDP(sdp.Dependencies{Logger: new(testLogger)})
	err := sdp.LoadBytes([]byte(s))
	if err != nil {
		fmt.Print("------------------------------------\n")
		fmt.Print("IN:\n")
		fmt.Print(s)
		fmt.Print("------------------------------------\n")
		fmt.Print("\n")
		fmt.Print("------------------------------------\n")
		fmt.Print("OUT:\n")
		//fmt.Printf("%+v\n", sdp.Data)
		fmt.Printf("%s\n", sdp.Write(context.Background()))
		//fmt.Print("\n")
		//fmt.Printf("%# v\n", pretty.Formatter(sdp.Data))
		fmt.Print("------------------------------------\n")

		t.Fatal(err)
	}
}

func TestSDPIOS(t *testing.T) {
	var s string = `v=0
o=- 8098683594627032581 2 IN IP4 127.0.0.1
s=-
t=0 0
a=group:BUNDLE audio video
a=msid-semantic: WMS ViewControllerSTREAM
m=audio 9 UDP/TLS/RTP/SAVPF 111 103 104 9 102 0 8 106 105 13 110 112 113 126
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:azHt
a=ice-pwd:xPEtVFx4soq8hLL7cpQp5Iox
a=ice-options:trickle renomination
a=fingerprint:sha-256 CE:E2:91:A2:39:16:4F:7B:09:9A:0E:D1:94:33:1F:F2:5A:DC:D5:4E:BD:F6:9B:37:6A:90:75:14:49:DB:20:1B
a=setup:actpass
a=mid:audio
a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level
a=sendrecv
a=rtcp-mux
a=rtpmap:111 opus/48000/2
a=rtcp-fb:111 transport-cc
a=fmtp:111 maxplaybackrate=16000; sprop-maxcapturerate=16000; maxaveragebitrate=20000; stereo=1; useinbandfec=1; usedtx=0
a=ptime:40
a=maxptime:40
a=rtpmap:103 ISAC/16000
a=rtpmap:104 ISAC/32000
a=rtpmap:9 G722/8000
a=rtpmap:102 ILBC/8000
a=rtpmap:0 PCMU/8000
a=rtpmap:8 PCMA/8000
a=rtpmap:106 CN/32000
a=rtpmap:105 CN/16000
a=rtpmap:13 CN/8000
a=rtpmap:110 telephone-event/48000
a=rtpmap:112 telephone-event/32000
a=rtpmap:113 telephone-event/16000
a=rtpmap:126 telephone-event/8000
a=ssrc:4026163009 cname:N2GDmT1I5MhGsjSB
a=ssrc:4026163009 msid:ViewControllerSTREAM ViewControllerAUDIO
a=ssrc:4026163009 mslabel:ViewControllerSTREAM
a=ssrc:4026163009 label:ViewControllerAUDIO
m=video 9 UDP/TLS/RTP/SAVPF 98 96 97 99 100 101 127
c=IN IP4 0.0.0.0
a=rtcp:9 IN IP4 0.0.0.0
a=ice-ufrag:azHt
a=ice-pwd:xPEtVFx4soq8hLL7cpQp5Iox
a=ice-options:trickle renomination
a=fingerprint:sha-256 CE:E2:91:A2:39:16:4F:7B:09:9A:0E:D1:94:33:1F:F2:5A:DC:D5:4E:BD:F6:9B:37:6A:90:75:14:49:DB:20:1B
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
a=rtpmap:98 H264/90000
a=rtcp-fb:98 goog-remb
a=rtcp-fb:98 transport-cc
a=rtcp-fb:98 ccm fir
a=rtcp-fb:98 nack
a=rtcp-fb:98 nack pli
a=fmtp:98 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
a=rtpmap:99 rtx/90000
a=fmtp:99 apt=98
a=rtpmap:100 red/90000
a=rtpmap:101 rtx/90000
a=fmtp:101 apt=100
a=rtpmap:127 ulpfec/90000
a=ssrc-group:FID 4029573333 4197005919
a=ssrc:4029573333 cname:N2GDmT1I5MhGsjSB
a=ssrc:4029573333 msid:ViewControllerSTREAM ViewControllerVIDEO
a=ssrc:4029573333 mslabel:ViewControllerSTREAM
a=ssrc:4029573333 label:ViewControllerVIDEO
a=ssrc:4197005919 cname:N2GDmT1I5MhGsjSB
a=ssrc:4197005919 msid:ViewControllerSTREAM ViewControllerVIDEO
a=ssrc:4197005919 mslabel:ViewControllerSTREAM
a=ssrc:4197005919 label:ViewControllerVIDEO
`

	sdp := sdp.NewSDP(sdp.Dependencies{Logger: new(testLogger)})
	err := sdp.LoadBytes([]byte(s))
	if err != nil {
		fmt.Print("------------------------------------\n")
		fmt.Print("IN:\n")
		fmt.Print(s)
		fmt.Print("------------------------------------\n")
		fmt.Print("\n")
		fmt.Print("------------------------------------\n")
		fmt.Print("OUT:\n")
		//fmt.Printf("%+v\n", sdp.Data)
		fmt.Printf("%s\n", sdp.Write(context.Background()))
		//fmt.Print("\n")
		//fmt.Printf("%# v\n", pretty.Formatter(sdp.Data))
		fmt.Print("------------------------------------\n")

		t.Fatal(err)
	}
}
