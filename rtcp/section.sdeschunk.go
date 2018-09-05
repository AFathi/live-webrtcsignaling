package rtcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

/*
	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1

 @see https://tools.ietf.org/html/rfc3550#section-6.5
 			 +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
chunk  |                          SSRC/CSRC_i                          |
  i    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
			 |                           SDES items                          |
			 |                              ...                              |
			 +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+

 Items are contiguous, i.e., items are not individually padded to a
    32-bit boundary.  Text is not null terminated because some multi-
    octet encodings include null octets.  The list of items in each chunk
    MUST be terminated by one or more null octets, the first of which is
    interpreted as an item type of zero to denote the end of the list.
    No length octet follows the null item type octet, but additional null
    octets MUST be included if needed to pad until the next 32-bit
    boundary.  Note that this padding is separate from that indicated by
    the P bit in the RTCP SDESChunk.  A chunk with zero items (four null
    octets) is valid but useless.
*/
type SDESChunk struct {
	// public
	SSRC  uint32
	Items SDESItems
	// private
	size int
}

func NewSDESChunk() *SDESChunk {
	return new(SDESChunk)
}

func (c *SDESChunk) Parse(data []byte) error {
	if len(data) < 4 {
		return errors.New("sdes chunk size")
	}
	// parsing the SDESChunk
	c.SSRC = binary.BigEndian.Uint32(data[0:4])
	offset := 4
	// parsing items
	for {
		item := NewSDESItem()
		if len(data) < offset {
			return errors.New("sdes item size")
		}
		err := item.Parse(data[offset:])
		if err != nil {
			return err
		}
		c.Items = append(c.Items, *item)
		offset += item.GetSize()
		if item.Typ == SDES_NULL {
			break
		}
	}
	// chunk is padded to 32bit boundary
	c.size = offset + (offset % 4)
	return nil
}

// return the byte size of the SDESChunk
func (c *SDESChunk) GetSize() int {
	return c.size
}

func (c *SDESChunk) String() string {
	return fmt.Sprintf(
		"Ck(ssrc=%d %s)",
		c.SSRC,
		c.Items.String(),
	)
}

type SDESChunks []SDESChunk

func (l *SDESChunks) String() string {
	var chunks []string

	for _, chunk := range *l {
		chunks = append(chunks, chunk.String())
	}
	return "Cks=[" + strings.Join(chunks, ", ") + "]"
}
