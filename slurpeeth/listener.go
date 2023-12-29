package slurpeeth

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
)

// NewListener returns a new listener listening on 127.0.0.1 and the slurpeeth port.
func NewListener(
	address string,
	port uint16,
	messageRelay func(id uint16, m *Message),
	errChan chan error,
	shutdownChan chan bool,
) (*Listener, error) {
	listener := &Listener{
		addr:         fmt.Sprintf("%s:%d", address, port),
		l:            nil,
		messageRelay: messageRelay,
		errChan:      errChan,
		shutdownChan: shutdownChan,
	}

	return listener, nil
}

// Listener is the main process listening on the slurpeeth port -- it relays messages to workers
// based on tunnel id (via the manager).
type Listener struct {
	addr         string
	l            net.Listener
	messageRelay func(id uint16, m *Message)
	errChan      chan error
	shutdownChan chan bool
}

// Bind starts the listener/binds it to the address/port it was created with. It must be called
// before Run.
func (l *Listener) Bind() error {
	lis, err := net.Listen(TCP, l.addr)
	if err != nil {
		return err
	}

	l.l = lis

	return nil
}

// Run runs the listener.
func (l *Listener) Run() {
	if l.l != nil {
		err := l.l.Close()
		if err != nil {
			log.Printf(
				"encountered error closing listener prior to running, will ignore, err: %s",
				err,
			)
		}

		err = l.Bind()
		if err != nil {
			log.Printf(
				"encountered error re-binding listener prior to running, cannot continue, err: %s",
				err,
			)

			l.errChan <- err

			return
		}
	}

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
	log.Printf("received new connection from %q", conn.RemoteAddr())

	for {
		m, err := NewMessageFromConn(conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf(
					"encountered EOF error during listener handle," +
						" exiting handler for this connection...",
				)

				break
			}

			// something other than EOF we dunno how to handle for now and is probably bad
			l.errChan <- err

			break
		}

		go l.messageRelay(m.Header.ID, &m)
	}

	err := conn.Close()
	if err != nil {
		log.Printf(
			"encountered error closing connection from %q, will ignore. error: %s",
			conn.RemoteAddr(), err,
		)
	}
}
