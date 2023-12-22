package slurpeeth

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
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
		shutdownChan:    make(chan bool),
		// needs to be buffered since we most likely wont be receiving a new connection at the time
		// of shutdown, we dont have this problem on the sender so it can be unbuffered
		receiverShutdownChans: make(map[string]chan bool, 1),
		senderShutdownChans:   make(map[string]chan bool, 1),
	}

	return s, nil
}

// DomainWorker is an object that works for a given domain -- a broadcast domain.
type DomainWorker struct {
	Config                Domain
	errChan               chan error
	receiverErrChan       chan error
	senderErrChan         chan error
	receivers             map[string]net.Listener
	senders               map[string]*domainSender
	shutdownInProgress    bool
	shutdownChan          chan bool
	receiverShutdownChans map[string]chan bool
	senderShutdownChans   map[string]chan bool
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
	log.Println("begin domain worker bind...")

	go d.propagateErrors()
	go d.shutdownFanout()

	return nil
}

func (d *DomainWorker) shutdownFanout() {
	for <-d.shutdownChan {
		log.Printf(
			"received shutdown signal, sending to receivers and senders",
		)

		for _, receiverShutdownChan := range d.receiverShutdownChans {
			receiverShutdownChan <- true
		}

		for _, senderShutdownChan := range d.senderShutdownChans {
			senderShutdownChan <- true
		}
	}
}

func (d *DomainWorker) propagateErrors() {
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
}

// Run runs the worker forever. The worker should manage the connection, restarting things if
// needed. Any errors should be returned on the error channel the worker was created with.
func (d *DomainWorker) Run() {
	log.Println("begin domain worker run")

	go d.sendersRun()
	go d.receiversRun()
}

// Shutdown shuts down DomainWorker sender and receiver.
func (d *DomainWorker) Shutdown(wg *sync.WaitGroup) {
	log.Print(
		"begin domain worker shutdown",
	)

	// send the shutdown signal to stop things
	d.shutdownInProgress = true
	d.shutdownChan <- true

	for {
		var sendersNotNil bool

		for _, sender := range d.senders {
			if sender.conn != nil {
				sendersNotNil = true

				break
			}
		}

		if sendersNotNil {
			fmt.Println("SENDER NOT NIL")
			time.Sleep(shutdownCheckDelay)

			continue
		}

		var receiversNotNil bool

		for _, receiver := range d.receivers {
			if receiver != nil {
				receiversNotNil = true

				break
			}
		}

		if receiversNotNil {
			fmt.Println("RECEIVER NOT NIL")
			time.Sleep(shutdownCheckDelay)

			continue
		}

		break
	}

	log.Print("shutdown complete")

	wg.Done()
}
