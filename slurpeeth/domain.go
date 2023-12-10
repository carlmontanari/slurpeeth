package slurpeeth

import (
	"log"
	"net"
)

// NewDomainWorker returns a new (broadcast) domain worker.
func NewDomainWorker(config Domain, errChan chan error) (*DomainWorker, error) {
	s := &DomainWorker{
		Config:          config,
		errChan:         errChan,
		receiverErrChan: make(chan error),
		senderErrChan:   make(chan error),
		receivers:       make(map[string]net.Listener),
		senders:         make(map[string]*domainSender),
	}

	return s, nil
}

// DomainWorker is an object that works for a given domain -- a broadcast domain.
type DomainWorker struct {
	Config          Domain
	errChan         chan error
	receiverErrChan chan error
	senderErrChan   chan error
	receivers       map[string]net.Listener
	senders         map[string]*domainSender
}

// broadcast sends the given bytes b to all receivers other than the provided one.
func (d *DomainWorker) broadcast(receiverName string, b []byte) {
	for senderName, sender := range d.senders {
		if senderName == receiverName {
			continue
		}

		select {
		case sender.sendChan <- b:
		default:
			log.Printf(
				"failed writing message from receiver %q to sender %q, no consumer listening",
				receiverName, senderName,
			)
		}
	}
}

// Bind opens any sockets/listeners for the worker. For now a non-op for domain worker, keeping for
// consistency i guess.
func (d *DomainWorker) Bind() error {
	log.Println("begin domain worker bind, currently a noop...")

	return nil
}

// Run runs the worker forever. The worker should manage the connection, restarting things if
// needed. Any errors should be returned on the error channel the worker was created with.
func (d *DomainWorker) Run() {
	log.Println("begin domain worker run")

	go d.sendersRun()
	go d.receiversRun()

	go func() {
		for {
			select {
			case err := <-d.senderErrChan:
				log.Printf("received error on sender error channel, err: %s", err)

				d.errChan <- err
			case err := <-d.receiverErrChan:
				log.Printf("received error on receiver error channel, err: %s", err)

				d.errChan <- err
			}
		}
	}()
}
