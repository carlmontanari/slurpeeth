package slurpeeth

import (
	"fmt"
	"log"
	"net"
	"time"
)

type destinationWorker struct {
	name           string
	idx            int
	dialRetryCount int
	sendChan       chan *Message
	shutdownChan   chan bool
	conn           net.Conn
}

func (w *Worker) restartDestination(idx int) {
	log.Printf(
		"destination %q for tunnel id %d received restart request",
		w.destinations[idx].name,
		w.segment.ID,
	)

	w.shutdownDestination(idx)
	w.runDestination(idx)
}

func (w *Worker) shutdownDestination(idx int) {
	log.Printf(
		"destination %q for tunnel id %d received shutdown request",
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
				"encountered error closing connection to %q for tunnel id %d prior to running,"+
					" will ignore, err: %s",
				w.destinations[idx].name,
				w.segment.ID,
				err,
			)
		}

		w.destinations[idx].conn = nil
	}

	c, err := w.runDestinationDialRetry(w.destinations[idx].name)
	if err != nil {
		w.destinations[idx].dialRetryCount++

		if w.retry {
			dialRetrySleepSeconds := w.destinations[idx].dialRetryCount
			if dialRetrySleepSeconds > maxDialRetrySleepSeconds {
				dialRetrySleepSeconds = maxDialRetrySleepSeconds
			}

			log.Printf(
				"sleeping %d seconds before attempting to dial destination %q again...",
				dialRetrySleepSeconds,
				w.destinations[idx].name,
			)

			time.Sleep(time.Duration(dialRetrySleepSeconds) * time.Second)

			w.runDestination(idx)
		} else {
			w.destinationErrChan <- destinationError{
				idx:  idx,
				name: w.destinations[idx].name,
				err:  err,
			}
		}

		return
	}

	w.destinations[idx].dialRetryCount = 0
	w.destinations[idx].conn = c

	w.runDestinationHandler(idx)
}

func (w *Worker) runDestinationDialRetry(destination string) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", destination, w.port)

	log.Printf("dial destination %q for tunnel id %d", addr, w.segment.ID)

	startTime := time.Now()
	deadline := startTime.Add(w.dialTimeout)

	var retries int

	for {
		c, err := net.Dial(TCP, addr)
		if err == nil {
			log.Printf(
				"dial destination %q succeeded on attempt %d for tunnel id %d",
				addr,
				retries,
				w.segment.ID,
			)

			return c, nil
		}

		retries++

		if time.Now().After(deadline) {
			return nil, fmt.Errorf(
				"%w: maximum retry duration exceeeding attempting to dial destination %q"+
					" for tunnel id %d",
				ErrConnectivity,
				addr,
				w.segment.ID,
			)
		}

		if w.debug {
			log.Printf(
				"dial remote send destination %q for tunnel id %d, failed on attempt %d,"+
					" sleeping a bit before trying again...",
				addr, w.segment.ID, retries,
			)
		} else if retries%5 == 0 {
			log.Printf(
				"dial remote send destination %q for tunnel id %d, failed on attempt %d,"+
					" sleeping a bit before trying again...",
				addr, w.segment.ID, retries,
			)
		}

		time.Sleep(dialRetryDelay)
	}
}

func (w *Worker) runDestinationHandler(idx int) {
	for {
		select {
		case <-w.destinations[idx].shutdownChan:
			w.shutdownDestination(idx)

			return
		case msg := <-w.destinations[idx].sendChan:
			if w.shutdownInProgress {
				w.shutdownDestination(idx)

				return
			}

			if w.destinations[idx].conn == nil {
				// connection was closed but the worker itself isnt shutting down -- this can
				// happen when/if a destination goes down, so in that case we just want to be done
				// with this particular handler loop -- a new one will be spawned when/if the
				// destination is once again reachable
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

				w.destinationErrChan <- destinationError{
					idx:  idx,
					name: w.destinations[idx].name,
					err:  err,
				}

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
