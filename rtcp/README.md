# Description

this package gives functions to parse or write RTCP packets  

# RFC

https://tools.ietf.org/html/rfc3550 RTP  
https://tools.ietf.org/html/rfc3551 RTP/AVP  
This RFC defines a "profil" (named AVP) for RTP rfc3550 <=> it defines a usage for unspecified RTP rfc3550 fields
https://tools.ietf.org/html/rfc4585 RTP/AVPF  
RTP/AVPF is an extension of AVP definition, it defines early Feedback messages from the receiver.  
https://tools.ietf.org/html/rfc5104 RTP/AVPF codec control message  
https://tools.ietf.org/html/rfc3611.html RTCP XR (extended reports)  

https://tools.ietf.org/html/rfc6642 RTCP Extension for a Third-Party Loss Report  

# Messages types

The RTP control protocol (RTCP) is based on the periodic transmission of control packets to all participants in the session, using the same distribution mechanism as the data packets. https://tools.ietf.org/html/rfc3550#section-6

RTP messages :  
- SR: Sender Report  
- RR: Receiver Report  
- SDES: Source Description RTCP packet (CNAME, name/email/phone/loc/tool/..)  
- BYE: Goodbye  
- APP: Application defined RTCP packet  

These messages are sent grouped inside a "compound" RTCP packet:

```
if encrypted: random 32-bit integer
|
|[--------- packet --------][---------- packet ----------][-packet-]
|
|                receiver            chunk        chunk
V                reports           item  item   item  item
--------------------------------------------------------------------
R[SR #sendinfo #site1#site2][SDES #CNAME PHONE #CNAME LOC][BYE##why]
--------------------------------------------------------------------
|                                                                  |
|<-----------------------  compound packet ----------------------->|
|<--------------------------  UDP packet ------------------------->|

#: SSRC/CSRC identifier

           Figure 1: Example of an RTCP compound packet
```

These messages have a common header :

```
0                   1                   2                   3
0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|V=2|P|    RC   |       PT      |             length            |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

the message type is encoded inside PT & RC fields  
PT=200 <=> Sender Report  
PT=201 <=> Receiver Report  
(...)

Later, they build RTP/AVPF extension, providing a new protocol to AVP enabling immediate feedback to the senders : https://tools.ietf.org/html/rfc4585

This extension adds "Feedback Messages" to RTCP.  

we can distinguish 3 categories of feedback messages @see https://tools.ietf.org/html/rfc4585#section-6  
- Transport layer FB messages  
- Payload-specific FB messages  
- Application layer FB messages  

Transport Layer Feedback messages :  
- NACK: Negative Acknowledgements  (@see https://tools.ietf.org/html/rfc4585#section-6.2.1)  

Payload-specific Feedback messages :  
- PLI : Picture Loss Indication (@see https://tools.ietf.org/html/rfc4585#section-6.3.1)  
- SLI : Slice Loss Indication (@see https://tools.ietf.org/html/rfc4585#section-6.3.2)  
- RPSI : Reference Picture Selection Indication (@see https://tools.ietf.org/html/rfc4585#section-6.3.3)  

Also, some payload-feedback messages (codec) extensions are defined in @see https://tools.ietf.org/html/rfc5104

Transport Layer Feedback messages extension :
- TMMBR : Temporary Maximum Media Stream Bit Rate (@see https://tools.ietf.org/html/rfc5104#section-4.2.1)  
- TMMBN : Temporary Maximum Media Stream Bit Rate Notification (@see https://tools.ietf.org/html/rfc5104#section-4.2.2)  
- TLLEI : Transport-Layer Third-Party Loss Early Indication


Payload-specific Feedback messages extension :
- FIR : Full Intra Request (@see https://tools.ietf.org/html/rfc5104#section-4.3.1)  
- TSTR : Temporal-Spatial Trade-off Request (@see https://tools.ietf.org/html/rfc5104#section-4.3.2)  
- TSTN : Temporal-Spatial Trade-off Notification (@see https://tools.ietf.org/html/rfc5104#section-4.3.3)  
- VBCM : Video Back Channel Message (@see https://tools.ietf.org/html/rfc5104#section-4.3.4)  

The Application layer FB messages, from a protocol point of view, are treated as a special case of payload-specific FB message.  

- goog-remb : Receiver Estimated Max Bitrate (@see https://tools.ietf.org/html/draft-alvestrand-rmcat-remb-00#section-2)  

# RTCP Objects

Packet is an abstract minimalist common ground for all RTCP packet.  
Packet is embed in Packet{SR/SR/SDES/BYE} & PacketRTPFB  
PacketRTPFB is embed in PacketRTPFB{Nack/TMMBR/TMMBN/...}  

base packets :
```
Packet{SR/SR/SDES/BYE}{
  Packet{
    Header
    Data
  }
  <custom fields>
}
```

feedback transport (RTPFB) packets :
```
PacketFTPFB{Nack/TMMBR/TMMBN/...}{
  PacketRTPFB(
    Packet(
      Header()
      Data
    )
    SenderSSRC
    MediaSSRC
  )
  <custom fields>
}
```

# Usage

## Parser

```go
// packet must implement GetData/SetData/GetSize/Slice
var packet UdpPacket
parser := rtcp.NewParser(rtcp.Dependencies{Logger:log})
packets := parser.Parse(packet)
range _, packet in packets {
  switch v := packet.(type) {
  case *rtcp.PacketSR:
    // ...
  case *rtcp.PacketRR:
    // ...
  default:
    // ...
  }
}
```

# API

FIXME

# TODO

- review all tests len(data) < ... to ensure that   p.Header.GetFullPacketSize() < ... triggers an error.  
- maybe think of an API in Packet object to encapsulate offset & size manipulation  
   & ensure we can fetch data[offset:size]  
   & to bump offset & size triggering appropriate errors.  
