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

func (d *DomainWorker) sendersRun() {
	for domainSegment, segmentConfig := range d.Config.Participants {
		go d.senderRun(domainSegment, segmentConfig)
	}
}

func (d *DomainWorker) senderRun(domainSegmentName string, segmentConfig Segment) {
	if d.senders[domainSegmentName] != nil {
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
	for data := range d.senders[domainSegmentName].sendChan {
		dataN := len(data)

		writeN, err := d.senders[domainSegmentName].conn.Write(prependReadCount(data))
		if err != nil {
			log.Printf("encountered error writing message to destination, err: %s\n", err)

			d.receiverErrChan <- err

			continue
		}

		if writeN != dataN+MessageHeaderSize {
			log.Printf("wrote %d bytes, but expected to write %d", writeN, dataN+MessageHeaderSize)
		}
	}
}
