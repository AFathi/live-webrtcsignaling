package srtp

import (
	"net"
	"time"
)

type IPacket interface {
	GetData() []byte
	SetData([]byte)
	GetSize() int
	Slice(int, int)
}

type IPacketUDP interface {
	GetData() []byte
	SetData([]byte)
	GetSize() int
	Slice(int, int)
	GetRAddr() *net.UDPAddr
	SetRAddr(*net.UDPAddr)
	GetCreatedAt() time.Time
	SetCreatedAt(time.Time)
}
