package slurpeeth

import (
	"fmt"
	"net"
)

// NewMessageFromBody creates a new message from the given body bytes content.
func NewMessageFromBody(id uint16, sender string, body Bytes) Message {
	return Message{
		Header: NewHeaderFromBody(id, sender, body),
		Body:   body,
	}
}

// NewMessageFromConn returns the next message from the given connection.
func NewMessageFromConn(conn net.Conn) (Message, error) {
	rawHeaderBytes := make(Bytes, MessageHeaderSize)

	rawHeaderReadN, err := conn.Read(rawHeaderBytes)
	if err != nil {
		return Message{}, err
	}

	if rawHeaderReadN != MessageHeaderSize {
		return Message{}, fmt.Errorf(
			"%w: read %d bytes when attempting to read message header which should be %d bytes",
			ErrMessage, rawHeaderReadN, MessageHeaderSize,
		)
	}

	h, err := NewHeaderFromRaw(rawHeaderBytes)
	if err != nil {
		return Message{}, err
	}

	var totalBodyReadN int

	rawBodyBytes := make(Bytes, 0)

	for {
		var rawBodyReadN int

		rawBodyReadyBytes := make(Bytes, h.Size)

		rawBodyReadN, err = conn.Read(rawBodyReadyBytes)
		if err != nil {
			return Message{}, err
		}

		rawBodyBytes = append(rawBodyBytes, rawBodyReadyBytes[:rawBodyReadN]...)

		totalBodyReadN += rawBodyReadN

		totalBodyReadNUint16 := uint16(totalBodyReadN)

		if totalBodyReadNUint16 == h.Size {
			return Message{
				Header: h,
				Body:   rawBodyBytes,
			}, nil
		}

		if totalBodyReadNUint16 > h.Size {
			return Message{}, fmt.Errorf(
				"%w: read %d bytes, but thought we should read %d",
				ErrMessage,
				rawBodyReadN,
				h.Size,
			)
		}
	}
}

// Message is a message to/from slurpeeth endpoints.
type Message struct {
	Header Header
	Body   Bytes
}

// Output returns the full bytes of the Message including the header.
func (m *Message) Output() Bytes {
	return append(m.Header.Body, m.Body...)
}
