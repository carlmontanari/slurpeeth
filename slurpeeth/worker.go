package slurpeeth

import (
	"log"
	"sync"
	"time"
)

// NewWorker returns a new worker.
func NewWorker(
	port uint16,
	dialTimeout time.Duration,
	segment Segment,
	errChan chan error,
	retry,
	debug bool,
) (*Worker, error) {
	s := &Worker{
		retry: retry,
		debug: debug,

		port: port,

		dialTimeout: dialTimeout,

		segment: segment,

		errChan: errChan,

		interfaceErrChan:      make(chan error),
		interfaceShutdownChan: make(chan bool),
		interfaces:            make([]interfaceWorker, len(segment.Interfaces)),

		destinationFanoutChan:   make(chan *Message),
		destinationErrChan:      make(chan destinationError),
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
			idx:          idx,
			sendChan:     make(chan *Message),
			shutdownChan: make(chan bool),
		}
	}

	return s, nil
}

// Worker is an object that works for a given segment -- a p2p connection.
type Worker struct {
	retry bool
	debug bool

	port uint16

	dialTimeout time.Duration

	segment Segment

	// errChan is the handle to the error chanel in the manager process, this is how we propagate
	// errors up to the manager
	errChan chan error

	interfaceErrChan      chan error
	interfaceShutdownChan chan bool
	interfaces            []interfaceWorker

	// senders to destinations we represent
	destinationFanoutChan   chan *Message
	destinationErrChan      chan destinationError
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
		case destErr := <-w.destinationErrChan:
			log.Printf(
				"received error on worker -> destination %q error channel for tunnel id %d, "+
					"err: %s",
				destErr.name,
				w.segment.ID,
				destErr.err,
			)

			if w.retry {
				// we absolutely cannot block in this method, so... don't :)
				go w.restartDestination(destErr.idx)
			} else {
				w.errChan <- destErr.err
			}
		case err := <-w.interfaceErrChan:
			log.Printf(
				"received error on worker <-> interface error channel for tunnel id %d, err: %s",
				w.segment.ID,
				err,
			)

			// we dont do the retry business here because if there is some issue with the interface
			// we very likely wont solve it by just re-dialing/binding over and over!
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
	for range w.interfaceShutdownChan {
		for idx := range w.interfaces {
			w.interfaces[idx].shutdownChan <- true
		}
	}
}

func (w *Worker) destinationFanout() {
	for {
		select {
		case <-w.destinationShutdownChan:
			for idx := range w.destinations {
				w.destinations[idx].shutdownChan <- true
			}
		case msg := <-w.destinationFanoutChan:
			for idx := range w.destinations {
				// future: should this or this for loop be in a goroutine? so we dont block sending?
				// and/or should the channels be buffered for some amount?
				w.destinations[idx].sendChan <- msg
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

// Shutdown shuts down the Worker interfaces and destinations.
func (w *Worker) Shutdown(wg *sync.WaitGroup) {
	log.Printf("begin worker shutdown for tunnel id %d", w.segment.ID)

	// send the shutdown signal to stop things
	w.shutdownInProgress = true
	w.shutdownChan <- true

	// wait until things have closed and the conn/listener are nil'd
	for {
		var destinationConnsNotNil bool

		for idx := range w.destinations {
			if w.destinations[idx].conn != nil {
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

		var interfaceFdsNotZero bool

		for idx := range w.interfaces {
			if w.interfaces[idx].fd != 0 {
				interfaceFdsNotZero = true

				break
			}
		}

		if interfaceFdsNotZero {
			log.Printf(
				"interface fds for worker for tunnel id %d are not closed yet",
				w.segment.ID,
			)

			time.Sleep(shutdownCheckDelay)

			continue
		}

		break
	}

	log.Printf("worker shutdown complete for tunnel id %d", w.segment.ID)

	wg.Done()
}
