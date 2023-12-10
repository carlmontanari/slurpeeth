package slurpeeth

import (
	"log"
	"syscall"
)

func (s *SegmentWorker) senderRun() {
	if s.senderConn != nil {
		// listener was not nil, close it reset to nil
		err := s.senderConn.Close()
		if err != nil {
			log.Printf("encountered error closing sender connection, err: %s", err)
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
		data := make([]byte, ReadSize)

		readN, _, err := syscall.Recvfrom(s.Fd, data, 0)
		if err != nil {
			s.senderErrChan <- err

			return
		}

		var writeN int

		writeN, err = s.senderConn.Write(prependReadCount(data[:readN]))
		if err != nil {
			log.Printf("encountered error writing message to destination, err: %s\n", err)

			s.receiverErrChan <- err

			continue
		}

		if writeN != readN+MessageHeaderSize {
			log.Printf("wrote %d bytes, but expected to write %d", writeN, readN+MessageHeaderSize)
		}
	}
}
