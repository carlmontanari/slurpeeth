package slurpeeth

import (
	"log"
	"syscall"
)

func (w *Worker) runInterfaceRead() {
	for {
		select {
		case <-w.interfaceShutdownChan:
			return
		default:
			if w.shutdownInProgress {
				return
			}

			data := make([]byte, ReadSize)

			readN, _, err := syscall.Recvfrom(w.interfaceFd, data, 0)
			if err != nil {
				log.Printf(
					"encountered error receiving from interface %q for tunnel id %d, err: %s",
					w.segment.Interface, w.segment.ID, err,
				)

				w.interfaceErrChan <- err

				return
			}

			message := NewMessageFromBody(w.segment.ID, w.sender, data[:readN])

			w.destinationFanoutChan <- &message
		}
	}
}

func (w *Worker) runInterfaceWrite() {
	for {
		select {
		case <-w.interfaceShutdownChan:
			return
		case msg := <-w.messageChan:
			err := syscall.Sendto(w.interfaceFd, msg.Body, 0, w.interfaceDetails)
			if err != nil {
				log.Printf(
					"encountered error writing message to interface %q for tunnel id %d, err: %s",
					w.segment.Interface, w.segment.ID, err,
				)

				w.interfaceErrChan <- err
			}
		}
	}
}
