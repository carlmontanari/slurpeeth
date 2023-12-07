package slurpeeth

// NewDomainWorker returns a new (broadcast) domain worker.
func NewDomainWorker(config Domain, errChan chan error) (*DomainWorker, error) {
	s := &DomainWorker{
		Config:  config,
		errChan: errChan,
	}

	return s, nil
}

// DomainWorker is an object that works for a given domain -- a broadcast domain.
type DomainWorker struct {
	Config  Domain
	errChan chan error
}

// Bind opens any sockets/listeners for the worker.
func (d *DomainWorker) Bind() error {
	return nil
}

// Run runs the worker forever. The worker should manage the connection, restarting things if
// needed. Any errors should be returned on the error channel the worker was created with.
func (d *DomainWorker) Run() {}
