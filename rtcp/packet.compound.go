package rtcp

/*
   @see https://tools.ietf.org/html/rfc3550#section-6.1


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

  as stated in https://tools.ietf.org/html/rfc3711#section-3.4
   - the RTCP compound encryption prefix (random 32-bit) is omited
   - the authenticated portion of SRTCP packet is the whole compound

*/
type PacketCompound struct {
	Packet
}

// FIXME: writer funcs
