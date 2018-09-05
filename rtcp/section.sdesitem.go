package rtcp

import (
	"errors"
	"fmt"
	"strings"
)

/*
	0                   1                   2                   3
	0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1

 @see https://tools.ietf.org/html/rfc3550#section-6.5.1
 +-+-+-+-+-+-+-+-+
 |       0       |
 +-+-+-+-+-+-+-+-+

 "null" item: The list of items in each chunk
   MUST be terminated by one or more null octets, the first of which is
   interpreted as an item type of zero to denote the end of the list.
   No length octet follows the null item type octet

 @see https://tools.ietf.org/html/rfc3550#section-6.5.1
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |    CNAME=1    |     length    | user and domain name        ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 @see https://tools.ietf.org/html/rfc3550#section-6.5.2
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |     NAME=2    |     length    | common name of source       ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 @see https://tools.ietf.org/html/rfc3550#section-6.5.3
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |    EMAIL=3    |     length    | email address of source     ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 @see https://tools.ietf.org/html/rfc3550#section-6.5.4
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |    PHONE=4    |     length    | phone number of source      ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 @see https://tools.ietf.org/html/rfc3550#section-6.5.5
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |     LOC=5     |     length    | geographic location of site ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 @see https://tools.ietf.org/html/rfc3550#section-6.5.6
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |     TOOL=6    |     length    |name/version of source appl. ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 @see https://tools.ietf.org/html/rfc3550#section-6.5.7
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |     NOTE=7    |     length    | note about the source       ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 @see https://tools.ietf.org/html/rfc3550#section-6.5.8
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 |     PRIV=8    |     length    | prefix length |prefix string...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 ...             |                  value string               ...
 +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

 (...) Each item consists of an 8-
   bit type field, an 8-bit octet count describing the length of the
   text (thus, not including this two-octet header), and the text
   itself.
*/
type SDESItem struct {
	// public
	Typ    int
	Length int
	Text   string
	// private
	size int
}

func NewSDESItem() *SDESItem {
	return new(SDESItem)
}

func (i *SDESItem) Parse(data []byte) error {
	if len(data) < 1 {
		return errors.New("sdes item size")
	}
	i.Typ = int(data[0])
	switch i.Typ {
	case SDES_NULL:
		i.size = 1
		return nil
	case SDES_CNAME:
		fallthrough
	case SDES_NAME:
		fallthrough
	case SDES_EMAIL:
		fallthrough
	case SDES_PHONE:
		fallthrough
	case SDES_LOC:
		fallthrough
	case SDES_TOOL:
		fallthrough
	case SDES_NOTE:
		if len(data) < 2 {
			return errors.New("sdes item size")
		}
		i.Length = int(data[1]) // length of text.
		offset := 2
		if len(data) < offset+i.Length {
			return errors.New("sdes length")
		}
		i.Text = string(data[offset : offset+i.Length])
		i.size = offset + i.Length
		return nil
	case SDES_PRIV:
		if len(data) < 2 {
			return errors.New("sdes item size")
		}
		i.Length = int(data[1]) // length of text.
		offset := 2
		if len(data) < offset+i.Length {
			return errors.New("sdes length")
		}
		// FIXME: we should parse the content
		i.size = offset + i.Length
		return nil
	default:
		return errors.New("sdes item typ")
	}
}

func (i *SDESItem) GetSize() int {
	return i.size
}

func (i *SDESItem) String() string {
	switch i.Typ {
	case SDES_NULL:
		return "I(t=null)"
	case SDES_PRIV:
		return "I(t=priv)"
	default:
		return fmt.Sprintf(
			"I(t=%d l=%d txt=%s)",
			i.Typ,
			i.Length,
			i.Text,
		)
	}
}

type SDESItems []SDESItem

func (l *SDESItems) String() string {
	var items []string

	for _, item := range *l {
		items = append(items, item.String())
	}
	return "Itms=[" + strings.Join(items, ", ") + "]"
}
