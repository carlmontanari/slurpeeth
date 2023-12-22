package slurpeeth

import "time"

const (
	// Version is the version of slurpeeth, set w/ build flags in ci; only useful/relevant for cli.
	Version = "0.0.0"
)

const (
	// EthPAll is the already bit-shifted value of syscall.ETH_P_ALL.
	EthPAll = 768

	// TCP is a const for... TCP!
	TCP = "tcp"

	// ReadSize is the size of chunks we read from the interface a sender consumes from.
	ReadSize = 65_500

	// MessageHeaderSize is the size of the "header" we prepend to messages sent from a Sender --
	// this "field" specifies the size of the remaining message.
	MessageHeaderSize = 5
)

const (
	// MaxSenderRetries is the maximum amount of retry attempts to connect to a sender destination.
	MaxSenderRetries = 60
)

const (
	shutdownCheckDelay = 10 * time.Millisecond
)
