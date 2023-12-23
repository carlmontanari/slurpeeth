package slurpeeth

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// NewWorker returns a new worker.
func NewWorker(
	port uint16,
	segment Segment,
	errChan chan error,
) (*Worker, error) {
	s := &Worker{
		port:    port,
		segment: segment,

		messageChan: make(chan *Message),
		errChan:     errChan,

		interfaceErrChan:      make(chan error),
		interfaceShutdownChan: make(chan bool),

		destinationFanoutChan:   make(chan *Message),
		destinationErrChan:      make(chan error),
		destinationShutdownChan: make(chan bool),
		destinations:            make([]destinationWorker, len(segment.Destinations)),

		shutdownChan: make(chan bool),
	}

	segmentBytes, err := yaml.Marshal(segment)
	if err != nil {
		return nil, err
	}

	segmentHash := sha256.New()
	segmentHash.Write(segmentBytes)

	s.sender = hex.EncodeToString(segmentHash.Sum(nil))

	namedInterface, err := net.InterfaceByName(segment.Interface)
	if err != nil {
		return nil, err
	}

	s.interfaceDetails = &syscall.SockaddrLinklayer{
		Ifindex: namedInterface.Index,
	}

	for idx, destination := range segment.Destinations {
		s.destinations[idx] = destinationWorker{
			name:         destination,
			sendChan:     make(chan *Message),
			shutdownChan: make(chan bool),
		}
	}

	return s, nil
}

// Worker is an object that works for a given segment -- a p2p connection.
type Worker struct {
	port    uint16
	segment Segment

	// sender is a 10 character string that is a hash of the segment information -- meant to
	// uniquely represent this worker in a message header (so we don't send messages from this
	// worker back to itself).
	sender string

	// TODO maybe there needs to be a fanout channel here if we make interfaces a slice
	// messageChan is the channel that messages that this worker should write to the interface it
	// represents.
	messageChan chan *Message

	// errChan is the handle to the error chanel in the manager process, this is how we propagate
	// errors up to the manager
	errChan chan error

	// things related to the interface we may or may not need to write to (depending on the config)
	interfaceDetails      *syscall.SockaddrLinklayer
	interfaceFd           int
	interfaceErrChan      chan error
	interfaceShutdownChan chan bool

	// senders to destinations we represent
	destinationFanoutChan   chan *Message
	destinationErrChan      chan error
	destinationShutdownChan chan bool
	destinations            []destinationWorker

	shutdownInProgress bool
	shutdownChan       chan bool
}

// Bind opens any sockets/listeners for the worker.
func (w *Worker) Bind() error {
	log.Printf("binding for worker for tunnel id %d", w.segment.ID)

	if w.segment.Interface != "" {
		err := w.bindInterface()
		if err != nil {
			return err
		}
	}

	log.Printf("starting worker background tasks for tunnel id %d...", w.segment.ID)
	go w.propagateErrors()
	go w.shutdownFanout()
	go w.destinationFanout()

	log.Printf("binding for worker for tunnel id %d complete", w.segment.ID)

	return nil
}

func (w *Worker) bindInterface() error {
	log.Printf(
		"begin worker bind for interface %s for tunnel id %d", w.segment.Interface, w.segment.ID,
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
					" close file descriptor",
				err, closeErr,
			)
		}

		return err
	}

	err = syscall.Bind(fd, w.interfaceDetails)
	if err != nil {
		closeErr := syscall.Close(fd)
		if closeErr != nil {
			log.Printf(
				"encountered error %q binding to interface %s, and subsequent error %q"+
					" attempting to close file descriptor",
				err, w.segment.Interface, closeErr,
			)
		}

		return err
	}

	log.Printf(
		"begin worker bind for interface %s for tunnel id %d complete!",
		w.segment.Interface,
		w.segment.ID,
	)

	w.interfaceFd = fd

	return nil
}

func (w *Worker) propagateErrors() {
	for {
		select {
		case err := <-w.destinationErrChan:
			log.Printf(
				"received error on worker -> destination error channel for tunnel id %d, err: %s",
				w.segment.ID,
				err,
			)

			w.errChan <- err
		case err := <-w.interfaceErrChan:
			log.Printf(
				"received error on worker <-> interface error channel for tunnel id %d, err: %s",
				w.segment.ID,
				err,
			)

			w.errChan <- err
		}
	}
}

func (w *Worker) shutdownFanout() {
	for <-w.shutdownChan {
		log.Printf(
			"received shutdown signal for worker for tunnel id %d",
			w.segment.ID,
		)

		w.interfaceShutdownChan <- true
		w.destinationShutdownChan <- true
	}
}

func (w *Worker) destinationFanout() {
	for {
		select {
		case <-w.destinationShutdownChan:
			for _, destination := range w.destinations {
				// TODO maybe need to do a select on this
				destination.shutdownChan <- true
			}
		case msg := <-w.destinationFanoutChan:
			for _, destination := range w.destinations {
				select {
				case destination.sendChan <- msg:
				default:
					log.Printf(
						"failed writing message from for tunnel id %d to destination %s",
						w.segment.ID, destination.name,
					)
				}
			}
		}
	}
}

// Run runs the worker forever. The worker should manage the connection, restarting things if
// needed. Any errors should be returned on the error channel the worker was created with.
func (w *Worker) Run() {
	log.Printf("begin worker run for tunnel id %d", w.segment.ID)

	w.shutdownInProgress = false

	if w.interfaceFd != 0 {
		go w.runInterfaceRead()
		go w.runInterfaceWrite()
	}

	if len(w.segment.Destinations) > 0 {
		go w.runDestinations()
	}
}

// Shutdown shuts down SegmentWorker sender and receiver.
func (w *Worker) Shutdown(wg *sync.WaitGroup) {
	log.Printf("begin worker shutdown for tunnel id %d", w.segment.ID)

	// send the shutdown signal to stop things
	w.shutdownInProgress = true
	w.shutdownChan <- true

	// wait until things have closed and the conn/listener are nil'd
	for {
		var destinationConnsNotNil bool

		for _, destination := range w.destinations {
			if destination.conn != nil {
				destinationConnsNotNil = true

				break
			}
		}

		if destinationConnsNotNil {
			log.Printf(
				"destination connections for worker for tunnel id %d are not closed yet",
				w.segment.ID,
			)

			time.Sleep(shutdownCheckDelay)

			continue
		}

		// TODO -- probably need to check some fd shit?

		break
	}

	log.Printf("worker shutdown complete for tunnel id %d", w.segment.ID)

	wg.Done()
}
