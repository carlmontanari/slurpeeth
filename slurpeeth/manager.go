package slurpeeth

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Manager is an interface representing the manager singleton's methods.
type Manager interface {
	Run() error
	RunDaemon(exitErr, exitDone chan bool) error
}

type manager struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	errored   bool

	debug bool

	configPath  string
	config      *Config
	liveReload  bool
	workerRetry bool

	// listen address -- defaults to 0.0.0.0.
	address string
	// listen port -- defaults to 4799.
	port uint16

	// maximum duration workers will try to dial a destination -- defaults to 1 minute.
	dialTimeout time.Duration

	// channel to receiver errors from the workers on.
	errChan chan error

	// listener is the object that handles incoming connections -- messages are received, the header
	// is parsed, and then the message content is dispatched to the necessary worker.
	listenerShutdownChan chan bool
	listener             *Listener

	// workers is a mapping of Worker -- the key is the uint16 tunnel id.
	workers map[uint16]*Worker
}

var managerInst *manager //nolint:gochecknoglobals

// GetManager returns the singleton implementation of Manager.
func GetManager(opts ...Option) (Manager, error) {
	if managerInst != nil {
		return managerInst, nil
	}

	ctx, ctxCancel := SignalHandledContext(log.Printf)

	m := &manager{
		ctx:        ctx,
		ctxCancel:  ctxCancel,
		configPath: "slurpeeth.yaml",
		config: &Config{
			Segments: []Segment{},
		},
		workerRetry:          true,
		address:              Address,
		port:                 Port,
		dialTimeout:          DialTimeout,
		errChan:              make(chan error),
		listenerShutdownChan: make(chan bool),
		workers:              map[uint16]*Worker{},
	}

	for _, opt := range opts {
		err := opt(m)
		if err != nil {
			log.Printf("failed applying manager config option, err: %s\n", err)

			return nil, err
		}
	}

	qualifiedConfigPath, err := filepath.Abs(m.configPath)
	if err != nil {
		log.Printf("failed determining absolute path to config, err: %s\n", err)

		return nil, err
	}

	m.configPath = qualifiedConfigPath

	configBytes, err := os.ReadFile(m.configPath)
	if err != nil {
		log.Printf("failed reading config file at path %q, err: %s\n", m.configPath, err)

		return nil, err
	}

	err = yaml.Unmarshal(configBytes, m.config)
	if err != nil {
		log.Printf("failed unmarshlaing config file, err: %s\n", err)

		return nil, err
	}

	managerInst = m

	return managerInst, nil
}

func (m *manager) run() error {
	log.Println("manager run started...")

	log.Println("starting error receiver...")

	go m.listenErrors()

	log.Println("setting up workers...")

	err := m.setupWorkers()
	if err != nil {
		log.Printf("error creating workers: %s\n", err)

		return err
	}

	log.Println("setting up listener...")

	err = m.setupListener()
	if err != nil {
		log.Printf("error creating listener: %s\n", err)

		return err
	}

	log.Println("starting workers...")

	m.startWorkers()

	log.Println("starting listener...")

	m.startListener()

	log.Println("processing watch config...")

	err = m.watchConfig()
	if err != nil {
		log.Printf("error setting up config watch: %s\n", err)

		return err
	}

	return nil
}

// Run starts all connections in the configuration and runs until sigint or failure.
func (m *manager) Run() error {
	err := m.run()
	if err != nil {
		return err
	}

	log.Println("running forever or until error...")

	<-m.ctx.Done()

	if m.errored {
		log.Println("root context signaled done with error, exiting...")

		return fmt.Errorf("%w: exiting due to worker signaling error", ErrConnectivity)
	}

	log.Println("root context signaled done without error, exiting...")

	return nil
}

// RunDaemon runs the "normal" run things but accepts channels that it can send a signal on if it
// exits (rather than blocking like "normal" Run does).
func (m *manager) RunDaemon(exitErr, exitDone chan bool) error {
	err := m.run()
	if err != nil {
		return err
	}

	log.Println("running in background forever or until error...")

	go func() {
		<-m.ctx.Done()

		if m.errored {
			log.Println("root context signaled done with error, exiting...")

			exitErr <- true

			return
		}

		log.Println("root context signaled done without error, exiting...")

		exitDone <- true
	}()

	return nil
}

func (m *manager) listenErrors() {
	for err := range m.errChan {
		log.Printf("received error during run, err: %s\n", err)

		m.errored = true

		m.ctxCancel()
	}
}

func (m *manager) setupWorkers() error {
	for _, segmentConfig := range m.config.Segments {
		worker, err := NewWorker(
			m.port,
			m.dialTimeout,
			segmentConfig,
			m.errChan,
			m.workerRetry,
			m.debug,
		)
		if err != nil {
			return err
		}

		err = worker.Bind()
		if err != nil {
			return err
		}

		m.workers[segmentConfig.ID] = worker
	}

	return nil
}

func (m *manager) startWorkers() {
	for _, segment := range m.workers {
		segment.Run()
	}
}

func (m *manager) shutdownWorkers() {
	wg := &sync.WaitGroup{}

	wg.Add(len(m.workers))

	for _, segment := range m.workers {
		go segment.Shutdown(wg)
	}

	wg.Wait()
}

func (m *manager) setupListener() error {
	l, err := NewListener(m.address, m.port, m.messageRelay, m.errChan, m.listenerShutdownChan)
	if err != nil {
		return err
	}

	err = l.Bind()
	if err != nil {
		return err
	}

	m.listener = l

	return nil
}

func (m *manager) startListener() {
	go m.listener.Run()
}

func (m *manager) messageRelay(id uint16, msg *Message) {
	worker, ok := m.workers[id]
	if !ok {
		log.Printf("message received for tunnel id %d, but no worker present for this tunnel", id)

		return
	}

	for idx := range worker.interfaces {
		if msg.Header.Sender == worker.interfaces[idx].sender {
			// message came from this worker, dont send it back to them
			continue
		}

		worker.interfaces[idx].sendChan <- msg
	}
}
