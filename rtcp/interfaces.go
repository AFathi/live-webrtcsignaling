package rtcp

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

type IPacketRTCP interface {
	GetData() []byte
	SetData([]byte)
	GetSize() int
	Slice(int, int)
	GetRAddr() *net.UDPAddr
	SetRAddr(*net.UDPAddr)
	GetCreatedAt() time.Time
	SetCreatedAt(time.Time)
}

type ILogger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

type IPLogger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Prefix(format string, args ...interface{}) IPLogger
	Tag(tag string) IPLogger
}
