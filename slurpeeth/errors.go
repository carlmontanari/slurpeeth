package slurpeeth

import (
	"errors"
)

// ErrConnectivity is a generic error for connectivity issues -- either between senders and
// receivers, or for connectivity to local interfaces.
var ErrConnectivity = errors.New("errConnectivity")

// ErrMessage is a generic error for issues with messages -- for example, when a message is received
// and the length is *not* what is specified in the message header.
var ErrMessage = errors.New("errMessage")

// ErrBind is a generic error for bind issues -- like finding a requested interface.
var ErrBind = errors.New("errBind")
