package slurpeeth

import (
	"log"
	"sync"
	"time"
)

// NewWorker returns a new worker.
func NewWorker(
	port uint16,
	segment Segment,
	errChan chan error,
	debug bool,
) (*Worker, error) {
	s := &Worker{
		debug: debug,

		port:    port,
		segment: segment,

		errChan: errChan,

		interfaceErrChan:      make(chan error),
		interfaceShutdownChan: make(chan bool),
		interfaces:            make([]interfaceWorker, len(segment.Interfaces)),

		destinationFanoutChan:   make(chan *Message),
		destinationErrChan:      make(chan error),
		destinationShutdownChan: make(chan bool),
		destinations:            make([]destinationWorker, len(segment.Destinations)),

		shutdownChan: make(chan bool),
	}

	for idx, segmentInterface := range segment.Interfaces {
		w, err := newInterfaceWorker(segment.Name, segmentInterface)
		if err != nil {
			return nil, err
		}

		s.interfaces[idx] = w
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
	debug bool

	port    uint16
	segment Segment

	// errChan is the handle to the error chanel in the manager process, this is how we propagate
	// errors up to the manager
	errChan chan error

	interfaceErrChan      chan error
	interfaceShutdownChan chan bool
	interfaces            []interfaceWorker

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

	for idx := range w.interfaces {
		log.Printf(
			"binding to interface %q for worker for tunnel id %d",
			w.interfaces[idx].name, w.segment.ID,
		)

		err := w.bindInterface(idx)
		if err != nil {
			return err
		}
	}

	log.Printf("starting worker background tasks for tunnel id %d...", w.segment.ID)

	go w.propagateErrors()
	go w.shutdownFanout()
	go w.interfaceFanout()
	go w.destinationFanout()

	log.Printf("binding for worker for tunnel id %d complete", w.segment.ID)

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

func (w *Worker) interfaceFanout() {
	for {
		select {
		case <-w.interfaceShutdownChan:
			for idx := range w.interfaces {
				// TODO maybe need to do a select on this
				w.interfaces[idx].shutdownChan <- true
			}
		}
	}
}

func (w *Worker) destinationFanout() {
	for {
		select {
		case <-w.destinationShutdownChan:
			for idx := range w.destinations {
				// TODO maybe need to do a select on this
				w.destinations[idx].shutdownChan <- true
			}
		case msg := <-w.destinationFanoutChan:
			for idx := range w.destinations {
				select {
				case w.destinations[idx].sendChan <- msg:
				default:
					log.Printf(
						"failed writing message from for tunnel id %d to destination %s",
						w.segment.ID, w.destinations[idx].name,
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

	w.runInterfaces()
	w.runDestinations()
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
