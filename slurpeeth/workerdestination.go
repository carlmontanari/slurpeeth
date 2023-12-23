package slurpeeth

import (
	"fmt"
	"log"
	"net"
	"time"
)

type destinationWorker struct {
	name         string
	sendChan     chan *Message
	shutdownChan chan bool
	conn         net.Conn
}

func (w *Worker) shutdownDestination(idx int) {
	log.Printf(
		"destination %q for tunnel id %d received shutdown",
		w.destinations[idx].name,
		w.segment.ID,
	)

	err := w.destinations[idx].conn.Close()
	if err != nil {
		log.Printf(
			"ignoring error closing conn for destination %q for tunnel id %d, err: %s",
			w.destinations[idx].name,
			w.segment.ID,
			err,
		)
	}

	w.destinations[idx].conn = nil
}

func (w *Worker) runDestinations() {
	for idx := range w.destinations {
		go w.runDestination(idx)
	}
}

func (w *Worker) runDestination(idx int) {
	if w.destinations[idx].conn != nil {
		// conn was not nil, close it, reset to nil, this probably shouldnt happen anyway
		err := w.destinations[idx].conn.Close()
		if err != nil {
			log.Printf(
				"encountered error closing connection to %q for tunnel id %d, err: %s",
				w.destinations[idx].name,
				w.segment.ID,
				err,
			)
		}

		w.destinations[idx].conn = nil
	}

	c, err := w.runDestinationDialRetry(w.destinations[idx].name)
	if err != nil {
		w.destinationErrChan <- err

		return
	}

	w.destinations[idx].conn = c

	w.runDestinationHandler(idx)
}

func (w *Worker) runDestinationDialRetry(destination string) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", destination, w.port)

	log.Printf("dial destination %q for tunnel id %d", addr, w.segment.ID)

	var retries int

	for {
		c, err := net.Dial(TCP, addr)
		if err == nil {
			log.Printf(
				"dial destination %q succeeded on %d attempt for tunnel id %d, continuing...",
				addr, w.segment.ID,
				retries,
			)

			return c, nil
		}

		retries++

		if retries > MaxSenderRetries {
			return nil, fmt.Errorf(
				"%w: maximum retries exceeeding attempting to dial destination %q for tunnel id %d",
				ErrConnectivity, addr, w.segment.ID,
			)
		}

		log.Printf(
			"dial remote send destination %q for tunnel id %d, failed on attempt %d,"+
				" sleeping a bit before trying again...",
			addr, w.segment.ID, retries,
		)

		time.Sleep(dialRetryDelay)
	}
}

func (w *Worker) runDestinationHandler(idx int) {
	for {
		select {
		case <-w.destinationShutdownChan:
			w.shutdownDestination(idx)

			return
		case msg := <-w.destinations[idx].sendChan:
			if w.shutdownInProgress {
				w.shutdownDestination(idx)

				return
			}

			n, err := w.destinations[idx].conn.Write(msg.Output())
			if err != nil {
				log.Printf(
					"encountered error writing message to destination %q for tunnel id %d, err: %s",
					w.destinations[idx].name,
					w.segment.ID,
					err,
				)

				w.destinationErrChan <- err

				continue
			}

			if uint16(n) != msg.Header.TotalSize {
				log.Printf(
					"wrote %d bytes to destination %q for tunnel id %d, but expected to write %d",
					n,
					w.destinations[idx].name,
					w.segment.ID,
					msg.Header.TotalSize,
				)
			}
		}
	}
}
