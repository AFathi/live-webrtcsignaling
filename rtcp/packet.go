package rtcp

type Packet struct {
	data []byte
}

func NewPacket() *Packet {
	p := new(Packet)
	p.data = make([]byte, 1600)
	return p
}

func (p *Packet) GetData() []byte {
	return p.data
}

func (p *Packet) SetData(data []byte) {
	p.data = data
}

func (p *Packet) GetSize() int {
	return len(p.data)
}

func (p *Packet) Slice(b int, e int) {
	p.data = p.data[b:e]
}
