package slurpeeth

import (
	"log"
	"net"
	"sync"
	"syscall"
	"time"
)

// NewSegmentWorker returns a new (p2p) segment worker.
func NewSegmentWorker(config Segment, errChan chan error) (*SegmentWorker, error) {
	s := &SegmentWorker{
		Config:          config,
		errChan:         errChan,
		receiverErrChan: make(chan error),
		senderErrChan:   make(chan error),
		shutdownChan:    make(chan bool),
		// needs to be buffered since we most likely wont be receiving a new connection at the time
		// of shutdown, we dont have this problem on the sender so it can be unbuffered
		receiverShutdownChan: make(chan bool, 1),
		senderShutdownChan:   make(chan bool),
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
	Config               Segment
	InterfaceLinkAddr    *syscall.SockaddrLinklayer
	Fd                   int
	errChan              chan error
	receiverErrChan      chan error
	senderErrChan        chan error
	receiverListener     net.Listener
	senderConn           net.Conn
	shutdownInProgress   bool
	shutdownChan         chan bool
	receiverShutdownChan chan bool
	senderShutdownChan   chan bool
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

	go s.propagateErrors()
	go s.shutdownFanout()

	return nil
}

func (s *SegmentWorker) shutdownFanout() {
	for <-s.shutdownChan {
		log.Printf(
			"received shutdown signal, sending to receiver and sender for interface %q",
			s.Config.Interface,
		)

		s.receiverShutdownChan <- true

		s.senderShutdownChan <- true
	}
}

func (s *SegmentWorker) propagateErrors() {
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
}

// Run runs the worker forever. The worker should manage the connection, restarting things if
// needed. Any errors should be returned on the error channel the worker was created with.
func (s *SegmentWorker) Run() {
	log.Printf(
		"begin sgement worker run for interface %s\n", s.Config.Interface,
	)

	s.shutdownInProgress = false

	go s.senderRun()
	go s.receiverRun()
}

// Shutdown shuts down SegmentWorker sender and receiver.
func (s *SegmentWorker) Shutdown(wg *sync.WaitGroup) {
	log.Printf(
		"begin sgement worker shutdown for interface %s\n", s.Config.Interface,
	)

	// send the shutdown signal to stop things
	s.shutdownInProgress = true
	s.shutdownChan <- true

	// wait until things have closed and the conn/listener are nil'd
	for {
		if s.senderConn == nil && s.receiverListener == nil {
			break
		}

		time.Sleep(shutdownCheckDelay)
	}

	log.Printf("shutdown complete for interface %s\n", s.Config.Interface)

	wg.Done()
}
