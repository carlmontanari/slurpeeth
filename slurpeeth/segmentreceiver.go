package slurpeeth

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

func (s *SegmentWorker) receiverRun() {
	if s.receiverListener != nil {
		// listener was not nil, close it reset to nil
		err := s.receiverListener.Close()
		if err != nil {
			log.Printf("encountered error closing receiver listener, err: %s", err)
		}

		s.receiverListener = nil
	}

	err := s.receiverListen()
	if err != nil {
		s.receiverErrChan <- err

		return
	}

	for {
		var conn net.Conn

		conn, err = s.receiverListener.Accept()
		if err != nil {
			s.errChan <- err
		} else {
			go s.receiverHandleConnection(conn)
		}
	}
}

func (s *SegmentWorker) receiverListen() error {
	listen := fmt.Sprintf("%s:%d", s.Config.Listen.Address, s.Config.Listen.Port)

	log.Printf("starting listen on %s\n", listen)

	l, err := net.Listen(TCP, listen)
	if err != nil {
		return err
	}

	s.receiverListener = l

	return nil
}

func (s *SegmentWorker) receiverHandleConnection(conn net.Conn) {
	log.Printf("received a new connection from %s\n", conn.RemoteAddr())

	for {
		messageLen, err := readSlurpeethHeaderSize(conn)
		if err != nil {
			log.Printf("encountered error reading slurpeeth message header, err: %s\n", err)

			s.receiverErrChan <- err

			continue
		}

		messageB := make([]byte, messageLen)

		lenMessageRead, err := conn.Read(messageB)
		if err != nil {
			log.Printf("encountered error reading message contents, err: %s\n", err)

			s.receiverErrChan <- err

			continue
		}

		if lenMessageRead != messageLen {
			msg := fmt.Sprintf(
				"we read %d bytes, but thought we should read %d",
				lenMessageRead,
				messageLen,
			)

			log.Println(msg)

			s.receiverErrChan <- fmt.Errorf("%w: %s", ErrMessage, msg)

			continue
		}

		err = syscall.Sendto(s.Fd, messageB[:lenMessageRead], 0, s.InterfaceLinkAddr)
		if err != nil {
			log.Printf(
				"encountered error writing message to interface %q, err: %s\n",
				s.Config.Interface, err,
			)

			s.receiverErrChan <- err
		}

		log.Printf(
			"received and wrote %d bytes to interface %s", lenMessageRead, s.Config.Interface,
		)
	}
}
