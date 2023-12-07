package slurpeeth

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func readSlurpeethHeaderSize(conn net.Conn) (int, error) {
	var expectedMessageLen int

	headerBytes := make([]byte, MessageHeaderSize)

	headerBytesReadLen, err := conn.Read(headerBytes)
	if err != nil {
		return 0, err
	}

	if headerBytesReadLen != MessageHeaderSize {
		return expectedMessageLen, fmt.Errorf(
			"%w: read %d bytes when attempting to read message header which should be %d bytes",
			ErrMessage, headerBytesReadLen, MessageHeaderSize,
		)
	}

	headerTrimmedString := strings.TrimLeft(string(headerBytes), "0")

	expectedMessageLen, err = strconv.Atoi(headerTrimmedString)
	if err != nil {
		return expectedMessageLen, err
	}

	return expectedMessageLen, nil
}

// prepends the len of b to b by using the first 5 chars and padding the int len.
func prependReadCount(b []byte) []byte {
	return append([]byte(fmt.Sprintf("%05d", len(b))), b...)
}
