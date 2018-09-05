package packet

import (
	"net"
	"time"
)

type UDP struct {
	data      []byte
	rAddr     *net.UDPAddr
	createdAt time.Time
}

func NewUDP() *UDP {
	p := new(UDP)
	p.data = make([]byte, 1600)
	p.createdAt = time.Now()
	return p
}

func NewUDPFromData(data []byte, rAddr *net.UDPAddr) *UDP {
	p := new(UDP)
	p.data = data
	p.rAddr = rAddr
	p.createdAt = time.Now()
	return p
}

/*
 * Implementing IPacket
 */
func (p *UDP) GetData() []byte {
	return p.data
}

func (p *UDP) SetData(data []byte) {
	p.data = data
}

func (p *UDP) GetSize() int {
	return len(p.data)
}

func (p *UDP) Slice(b int, e int) {
	p.data = p.data[b:e]
}

/*
 * Implementing IPacketUdp
 */
func (p *UDP) GetRAddr() *net.UDPAddr {
	return p.rAddr
}

func (p *UDP) SetRAddr(rAddr *net.UDPAddr) {
	p.rAddr = rAddr
}

func (p *UDP) GetCreatedAt() time.Time {
	return p.createdAt
}

func (p *UDP) SetCreatedAt(t time.Time) {
	p.createdAt = t
}

func (p *UDP) IsEmpty() bool {
	return len(p.data) == 0
}

/*
	@see RFC DTLS : https://tools.ietf.org/html/rfc7983#section-7
	  (update of RFC https://tools.ietf.org/html/rfc5764#section-5.1.2)

						+----------------+
						|        [0..3] -+--> forward to STUN
						|                |
						|      [16..19] -+--> forward to ZRTP
						|                |
packet -->  |      [20..63] -+--> forward to DTLS
						|                |
						|      [64..79] -+--> forward to TURN Channel
						|                |
						|    [128..191] -+--> forward to RTP/RTCP
						+----------------+
*/
func (p *UDP) IsSTUN() bool {
	return p.data[0] >= 0 && p.data[0] <= 3
}
func (p *UDP) IsDTLS() bool {
	return p.data[0] >= 20 && p.data[0] <= 63
}
func (p *UDP) IsSRTPorSRTCP() bool {
	return p.data[0] >= 128 && p.data[0] <= 191
}
