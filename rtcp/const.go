package rtcp

/*
  PT=packet types (RTCP header)
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|V=2|P|    RC   |        PT     |             length            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

Feedback messages: @see https://tools.ietf.org/html/rfc4585#section-6.1
Payload type (PT): 8 bits
		This is the RTCP packet type that identifies the packet as being
		an RTCP FB message.  Two values are defined by the IANA:

					Name   | Value | Brief Description
			 ----------+-------+------------------------------------
					RTPFB  |  205  | Transport layer FB message
					PSFB   |  206  | Payload-specific FB message
*/
const (
	PT_SR uint8 = 200 + iota
	PT_RR
	PT_SDES
	PT_BYE
	PT_APP
	PT_RTPFB // feedback transport layer (RTP)
	PT_PSFB  // feedback payload specific (codec)
	//
	PT_FIR  uint8 = 192
	PT_NACK uint8 = 193
)

/*
@see https://tools.ietf.org/html/rfc3550
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|    CNAME=1    |     length    | user and domain name        ...
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
const (
	SDES_NULL int = iota
	SDES_CNAME
	SDES_NAME
	SDES_EMAIL
	SDES_PHONE
	SDES_LOC
	SDES_TOOL
	SDES_NOTE
	SDES_PRIV
)

/*
  Feedback TL (Transport Layer)
	FMT values for PT=205 ( PT_RTPFB )

  @see https://tools.ietf.org/html/rfc4585#section-6.2 (base)
  @see https://tools.ietf.org/html/rfc5104#section-4.2 (extension)


  0                   1                   2                   3
  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |V=2|P|   FMT   |       PT      |          length               |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                  SSRC of packet sender                        |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |                  SSRC of media source                         |
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 :            Feedback Control Information (FCI)                 :
 :                                                               :

 FMT=
 (...)
  1:    Generic NACK
  2:    reserved (see note below)
  3:    Temporary Maximum Media Stream Bit Rate Request (TMMBR)
  4:    Temporary Maximum Media Stream Bit Rate Notification (TMMBN)
  31:   reserved for future expansion of the identifier number space
*/
const (
	_ uint8 = iota
	FMT_RTPFB_NACK
	FMT_RTPFB_RESERVED
	FMT_RTPFB_TMMBR
	FMT_RTPFB_TMMBN
	FMT_RTPFB_SR_REQ // 5: FIXME
	FMT_RTPFB_RAMS
	FMT_RTPFB_TLLEI
	FMT_RTPFB_ECN
	FMT_RTPFB_PS
	FMT_RTPFB_EXT uint8 = 31
)

/*
FMT values for PT=206 ( PT_PSFB )

@see https://tools.ietf.org/html/rfc4585#section-6.1
0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|V=2|P|   FMT   |       PT      |          length               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  SSRC of packet sender                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                  SSRC of media source                         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
:            Feedback Control Information (FCI)                 :
:                                                               :

@see https://tools.ietf.org/html/rfc4585#section-6.3
FMT=
0:     unassigned
1:     Picture Loss Indication (PLI)
2:     Slice Loss Indication (SLI)
3:     Reference Picture Selection Indication (RPSI)
4-14:  unassigned
15:    Application layer FB (AFB) message
16-30: unassigned
31:    reserved for future expansion of the sequence number space
@see https://tools.ietf.org/html/rfc5104#section-4.3
4:     Full Intra Request (FIR) Command
5:     Temporal-Spatial Trade-off Request (TSTR)
6:     Temporal-Spatial Trade-off Notification (TSTN)
7:     Video Back Channel Message (VBCM)
*/
const (
	_ uint8 = iota
	FMT_PSFB_PLI
	FMT_PSFB_SLI
	FMT_PSFB_RPSI
	FMT_PSFB_FIR
	FMT_PSFB_TSTR
	FMT_PSFB_TSTN
	FMT_PSFB_VBCM
	FMT_PSFB_PSLEI
	FMT_PSFB_ROI
	FMT_PSFB_AFB uint8 = 15 // Application Layer Feedback, ex: goog-remb
	FMT_PSFB_EXT uint8 = 31
)
