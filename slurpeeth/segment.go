package slurpeeth

import (
	"log"
	"net"
	"syscall"
)

// NewSegmentWorker returns a new (p2p) segment worker.
func NewSegmentWorker(config Segment, errChan chan error) (*SegmentWorker, error) {
	s := &SegmentWorker{
		Config:          config,
		errChan:         errChan,
		receiverErrChan: make(chan error),
		senderErrChan:   make(chan error),
	}

	namedInterface, err := net.InterfaceByName(config.Interface)
	if err != nil {
		return nil, err
	}

	s.InterfaceLinkAddr = &syscall.SockaddrLinklayer{
		Ifindex: namedInterface.Index,
	}

	return s, nil
}

// SegmentWorker is an object that works for a given segment -- a p2p connection.
type SegmentWorker struct {
	Config            Segment
	InterfaceLinkAddr *syscall.SockaddrLinklayer
	Fd                int
	errChan           chan error
	receiverErrChan   chan error
	senderErrChan     chan error
	receiverListener  net.Listener
	senderConn        net.Conn
}

// Bind opens any sockets/listeners for the worker.
func (s *SegmentWorker) Bind() error {
	log.Printf(
		"begin segment worker bind for interface %s\n", s.Config.Interface,
	)

	fd, err := syscall.Socket(
		syscall.AF_PACKET,
		syscall.SOCK_RAW,
		EthPAll,
	)
	if err != nil {
		closeErr := syscall.Close(fd)
		if closeErr != nil {
			log.Printf(
				"encountered initial error %q, and subsequent error %q attempting to"+
					" close file descriptor\n",
				err, closeErr,
			)
		}

		return err
	}

	err = syscall.Bind(fd, s.InterfaceLinkAddr)
	if err != nil {
		closeErr := syscall.Close(fd)
		if closeErr != nil {
			log.Printf(
				"encountered error %q binding to interface %s, and subsequent error %q"+
					" attempting to close file descriptor\n",
				err, s.Config.Interface, closeErr,
			)
		}

		return err
	}

	s.Fd = fd

	return nil
}

// Run runs the worker forever. The worker should manage the connection, restarting things if
// needed. Any errors should be returned on the error channel the worker was created with.
func (s *SegmentWorker) Run() {
	log.Printf(
		"begin sgement worker run for interface %s\n", s.Config.Interface,
	)

	go s.senderRun()
	go s.receiverRun()

	go func() {
		for {
			select {
			case err := <-s.senderErrChan:
				log.Printf("received error on sender error channel, err: %s", err)

				s.errChan <- err
			case err := <-s.receiverErrChan:
				log.Printf("received error on receiver error channel, err: %s", err)

				s.errChan <- err
			}
		}
	}()
}
