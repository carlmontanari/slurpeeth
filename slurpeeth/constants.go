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

	// Address is the default Slurpeeth listen address.
	Address = "0.0.0.0"

	// Port is the default Slurpeeth port.
	Port = 4799

	// DialTimeout is the default max dial time for workers dialing destinations.
	DialTimeout = time.Minute

	// ReadSize is the size of chunks we read from the interface a sender consumes from.
	ReadSize = 65_500

	// MessageHeaderSize is the size of the "header" we prepend to messages sent from a Sender --
	// this header contains the tunnel ID, size of the message, and some reserved space.
	MessageHeaderSize = 32
)

const (
	dialRetryDelay           = 500 * time.Millisecond
	shutdownCheckDelay       = 10 * time.Millisecond
	maxDialRetrySleepSeconds = 60
)
