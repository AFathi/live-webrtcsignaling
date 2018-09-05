package rtcp

import (
	"errors"
	"fmt"
)

/*
  @see https://tools.ietf.org/html/rfc3550#section-6.5

				0                   1                   2                   3
				0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
 			 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
header |V=2|P|    SC   |  PT=SDES=202  |             length            |
			 +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
chunk  |                          SSRC/CSRC_1                          |
  1    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
			 |                           SDES items                          |
			 |                              ...                              |
			 +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
chunk  |                          SSRC/CSRC_2                          |
  2    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
			 |                           SDES items                          |
			 |                              ...                              |
			 +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
*/
type PacketSDES struct {
	PacketRTCP
	Chunks SDESChunks
	// private
	size int
}

func NewPacketSDES() *PacketSDES {
	return new(PacketSDES)
}

func (p *PacketSDES) ParsePacketRTCP(packet *PacketRTCP) error {
	var err error

	// load packet
	p.PacketRTCP = *packet
	// setup offset
	offset := packet.GetOffset()
	// chunks
	for nbChunks := p.Header.ReceptionCount; nbChunks > 0; nbChunks-- {
		chunk := NewSDESChunk()
		if p.GetSize() < offset {
			return errors.New("PacketSDES chcunk size")
		}
		if err = chunk.Parse(p.GetData()[offset:]); err != nil {
			return err
		}
		p.Chunks = append(p.Chunks, *chunk)
		offset += chunk.GetSize()
	}
	p.size = offset
	return nil
}

func (p *PacketSDES) String() string {
	return fmt.Sprintf(
		"[RTCP-SDES %s %s]",
		p.PacketRTCP.String(),
		p.Chunks.String(),
	)
}
