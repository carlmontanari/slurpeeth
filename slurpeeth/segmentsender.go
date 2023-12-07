package slurpeeth

import (
	"fmt"
	"log"
	"net"
	"syscall"
	"time"
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

	err := s.senderDial()
	if err != nil {
		s.senderErrChan <- err

		return
	}

	s.senderHandler()
}

func (s *SegmentWorker) senderDial() error {
	destination := fmt.Sprintf("%s:%d", s.Config.Send.Address, s.Config.Send.Port)

	log.Printf("dial remote send destination %s\n", destination)

	var retries int

	for {
		c, err := net.Dial(TCP, destination)
		if err == nil {
			log.Printf("dial remote succeeded on %d attempt, continuing...\n", retries)

			s.senderConn = c

			return nil
		}

		retries++

		if retries > MaxSenderRetries {
			return fmt.Errorf(
				"%w: maximum retries exceeeding attempting to dial destination %s",
				ErrConnectivity, destination,
			)
		}

		log.Printf(
			"dial remote send destination failed on attempt %d,"+
				" sleeping a bit before trying again...",
			retries,
		)

		time.Sleep(time.Second)
	}
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
