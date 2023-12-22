package slurpeeth

import (
	"log"
	"syscall"
)

func (s *SegmentWorker) senderShutdown() {
	log.Printf(
		"sender for interface %q received shutdown",
		s.Config.Interface,
	)

	err := s.senderConn.Close()
	if err != nil {
		log.Printf(
			"ignoring error closing sender conn for itnerface %q, err: %s",
			s.Config.Interface,
			err,
		)
	}

	s.senderConn = nil
}

func (s *SegmentWorker) senderRun() {
	if s.senderConn != nil {
		// listener was not nil, close it, reset to nil, this probably shouldnt happen
		err := s.senderConn.Close()
		if err != nil {
			log.Printf(
				"encountered error closing sender connection for interface %q, err: %s",
				s.Config.Interface,
				err,
			)
		}

		s.senderConn = nil
	}

	c, err := senderDial(s.Config.Send)
	if err != nil {
		s.senderErrChan <- err

		return
	}

	s.senderConn = c

	s.senderHandler()
}

func (s *SegmentWorker) senderHandler() {
	for {
		select {
		case <-s.senderShutdownChan:
			go s.senderShutdown()

			return
		default:
			if s.shutdownInProgress {
				go s.senderShutdown()

				return
			}

			data := make([]byte, ReadSize)

			readN, _, err := syscall.Recvfrom(s.Fd, data, 0)
			if err != nil {
				s.senderErrChan <- err

				return
			}

			var writeN int

			writeN, err = s.senderConn.Write(prependReadCount(data[:readN]))
			if err != nil {
				log.Printf(
					"encountered error writing message to destination for interface %q, err: %s\n",
					s.Config.Interface,
					err,
				)

				s.receiverErrChan <- err

				continue
			}

			if writeN != readN+MessageHeaderSize {
				log.Printf(
					"wrote %d bytes, but expected to write %d",
					writeN,
					readN+MessageHeaderSize,
				)
			}
		}
	}
}
