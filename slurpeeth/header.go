package slurpeeth

import (
	"fmt"
	"strconv"
	"strings"
)

// Header is an object that is created from the first MessageHeaderSize bytes of a slurpeeth message
// it can be decoded to indicate the size of the message, and the id of the tunnel the message
// corresponds to.
type Header struct {
	Body   Bytes
	ID     uint16
	Size   uint16
	Sender string
	// TotalSize is the message size *and* the header size
	TotalSize uint16
}

func (h *Header) parse() error {
	id, err := paddedBytesToUint16(h.Body[0:5])
	if err != nil {
		return fmt.Errorf("%w: failed to parse tunnel id from header", ErrMessage)
	}

	size, err := paddedBytesToUint16(h.Body[5:10])
	if err != nil {
		return fmt.Errorf("%w: failed to parse payload size from header", ErrMessage)
	}

	h.ID = id

	h.Size = size

	h.TotalSize = h.Size + MessageHeaderSize

	h.Sender = string(h.Body[10:20])

	return nil
}

// NewHeaderFromRaw returns a new Header object from the bytes b -- it can fail if b is not
// exactly MessageHeaderSize in length.
func NewHeaderFromRaw(b Bytes) (Header, error) {
	l := len(b)

	if l != MessageHeaderSize {
		return Header{}, fmt.Errorf(
			"%w: header must be exactly %d bytes, got %d bytes", ErrMessage, MessageHeaderSize, l,
		)
	}

	h := Header{
		Body: b,
	}

	err := h.parse()
	if err != nil {
		return Header{}, err
	}

	return h, nil
}

func paddedBytesToUint16(b Bytes) (uint16, error) {
	s := strings.TrimLeft(string(b), "0")

	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}

	return uint16(i), nil
}

// NewHeaderFromBody returns a new Header object from the body bytes b. It cannot fail as we are
// constructing this from bytes so we are ourselves creating the header data.
func NewHeaderFromBody(id uint16, sender string, b Bytes) Header {
	l := uint16(len(b))

	return Header{
		// for now only id (5), size(5), and sender(10) are encoded in the header, then we pad 12
		// more 0s for future use
		Body:      []byte(fmt.Sprintf("%05d%05d%s%012d", id, l, sender, 0)),
		ID:        id,
		Size:      l,
		TotalSize: MessageHeaderSize + l,
		Sender:    sender,
	}
}
