package packet

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

type IPacketRTP interface {
	GetData() []byte
	SetData([]byte)
	GetSize() int
	Slice(int, int)
	GetRAddr() *net.UDPAddr
	SetRAddr(*net.UDPAddr)
	GetCreatedAt() time.Time
	SetCreatedAt(time.Time)
	GetSSRC() string
	GetPT() int
	GetTimestamp() uint32
	GetSeqNumber() uint16
}
