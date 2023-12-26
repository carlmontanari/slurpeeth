package slurpeeth

import (
	"fmt"
	"log"
	"net"
)

// NewListener returns a new listener listening on 127.0.0.1 and the slurpeeth port.
func NewListener(
	port uint16,
	messageRelay func(id uint16, m *Message),
	errChan chan error,
	shutdownChan chan bool,
) (*Listener, error) {
	listener := &Listener{
		l:            nil,
		messageRelay: messageRelay,
		errChan:      errChan,
		shutdownChan: shutdownChan,
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	l, err := net.Listen(TCP, addr)
	if err != nil {
		return nil, err
	}

	listener.l = l

	return listener, nil
}

// Listener is the main process listening on the slurpeeth port -- it relays messages to workers
// based on tunnel id (via the manager).
type Listener struct {
	l            net.Listener
	messageRelay func(id uint16, m *Message)
	errChan      chan error
	shutdownChan chan bool
}

// Run runs the listener.
func (l *Listener) Run() {
	go func() {
		for {
			conn, err := l.l.Accept()
			if err != nil {
				l.errChan <- err
			} else {
				go l.handle(conn)
			}
		}
	}()

	<-l.shutdownChan

	_ = l.l.Close()
}

func (l *Listener) handle(conn net.Conn) {
	log.Printf("received new connection from %s", conn.RemoteAddr())

	for {
		m, err := NewMessageFromConn(conn)
		if err != nil {
			l.errChan <- err

			return
		}

		go l.messageRelay(m.Header.ID, &m)
	}
}
