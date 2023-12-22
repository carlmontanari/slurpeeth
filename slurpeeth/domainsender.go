package slurpeeth

import (
	"log"
	"net"
)

type domainSender struct {
	send     Socket
	sendChan chan []byte
	conn     net.Conn
}

func (d *DomainWorker) senderShutdown(domainSegmentName string) {
	log.Printf(
		"sender for domain %q received shutdown",
		domainSegmentName,
	)

	err := d.senders[domainSegmentName].conn.Close()
	if err != nil {
		log.Printf(
			"ignoring error closing sender conn for domain %q, err: %s",
			domainSegmentName,
			err,
		)
	}

	d.senders[domainSegmentName].conn = nil
}

func (d *DomainWorker) sendersRun() {
	for domainSegment, segmentConfig := range d.Config.Participants {
		d.senderShutdownChans[domainSegment] = make(chan bool)

		go d.senderRun(domainSegment, segmentConfig)
	}
}

func (d *DomainWorker) senderRun(domainSegmentName string, segmentConfig Segment) {
	if d.senders[domainSegmentName] != nil && d.senders[domainSegmentName].conn != nil {
		// listener was not nil, close it reset to nil
		err := d.senders[domainSegmentName].conn.Close()
		if err != nil {
			log.Printf("encountered error closing sender connection, err: %s", err)
		}

		d.senders[domainSegmentName] = nil
	}

	d.senders[domainSegmentName] = &domainSender{
		send:     segmentConfig.Send,
		sendChan: make(chan []byte),
		conn:     nil,
	}

	c, err := senderDial(segmentConfig.Send)
	if err != nil {
		d.senderErrChan <- err

		return
	}

	d.senders[domainSegmentName].conn = c

	d.senderHandler(domainSegmentName)
}

func (d *DomainWorker) senderHandler(domainSegmentName string) {
	for {
		select {
		case <-d.senderShutdownChans[domainSegmentName]:
			go d.senderShutdown(domainSegmentName)

			return
		case data := <-d.senders[domainSegmentName].sendChan:
			if d.shutdownInProgress {
				go d.senderShutdown(domainSegmentName)

				return
			}

			dataN := len(data)

			writeN, err := d.senders[domainSegmentName].conn.Write(prependReadCount(data))
			if err != nil {
				log.Printf("encountered error writing message to destination, err: %s\n", err)

				d.receiverErrChan <- err

				continue
			}

			if writeN != dataN+MessageHeaderSize {
				log.Printf(
					"wrote %d bytes, but expected to write %d",
					writeN,
					dataN+MessageHeaderSize,
				)
			}
		}
	}
}
