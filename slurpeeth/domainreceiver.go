package slurpeeth

import (
	"fmt"
	"log"
	"net"
)

func (d *DomainWorker) receiversRun() {
	for domainName, segmentConfig := range d.Config.Participants {
		go d.receiverRun(domainName, segmentConfig)
	}
}

func (d *DomainWorker) receiverRun(domainSegmentName string, segmentConfig Segment) {
	if d.receivers[domainSegmentName] != nil {
		// listener was not nil, close it reset to nil
		err := d.receivers[domainSegmentName].Close()
		if err != nil {
			log.Printf(
				"encountered error closing receiver %q listener, err: %s",
				domainSegmentName,
				err,
			)
		}

		d.receivers[domainSegmentName] = nil
	}

	err := d.receiverListen(domainSegmentName, segmentConfig)
	if err != nil {
		d.receiverErrChan <- err

		return
	}

	for {
		var conn net.Conn

		conn, err = d.receivers[domainSegmentName].Accept()
		if err != nil {
			d.errChan <- err
		} else {
			go d.receiverHandleConnection(domainSegmentName, conn)
		}
	}
}

func (d *DomainWorker) receiverListen(
	domainSegmentName string,
	segmentConfig Segment,
) error {
	listen := fmt.Sprintf(
		"%s:%d",
		segmentConfig.Listen.Address,
		segmentConfig.Listen.Port,
	)

	log.Printf("starting listen on %s\n", listen)

	l, err := net.Listen(TCP, listen)
	if err != nil {
		return err
	}

	d.receivers[domainSegmentName] = l

	return nil
}

func (d *DomainWorker) receiverHandleConnection(domainName string, conn net.Conn) {
	log.Printf("received a new connection from %s\n", conn.RemoteAddr())

	for {
		messageLen, err := readSlurpeethHeaderSize(conn)
		if err != nil {
			log.Printf("encountered error reading slurpeeth message header, err: %s\n", err)

			d.receiverErrChan <- err

			continue
		}

		messageB := make([]byte, messageLen)

		lenMessageRead, err := conn.Read(messageB)
		if err != nil {
			log.Printf("encountered error reading message contents, err: %s\n", err)

			d.receiverErrChan <- err

			continue
		}

		if lenMessageRead != messageLen {
			msg := fmt.Sprintf(
				"we read %d bytes, but thought we should read %d",
				lenMessageRead,
				messageLen,
			)

			log.Println(msg)

			d.receiverErrChan <- fmt.Errorf("%w: %s", ErrMessage, msg)

			continue
		}

		d.broadcast(domainName, messageB)
	}
}
