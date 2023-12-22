package slurpeeth

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/fsnotify/fsnotify"

	"gopkg.in/yaml.v3"
)

// Manager is an interface representing the manager singleton's methods.
type Manager interface {
	Run() error
}

// Worker is an interface representing a Segment or a Domain worker.
type Worker interface {
	// Bind opens any sockets/listeners for the worker.
	Bind() error
	// Run runs the worker forever. The worker should manage the connection, restarting things if
	// needed. Any errors should be returned on the error channel the worker was created with.
	Run()
	// Shutdown shuts down the sender and receiver for the given worker.
	Shutdown(wg *sync.WaitGroup)
}

type manager struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	configPath string
	config     *Config
	liveReload bool

	errChan chan error

	segments map[string]Worker
	domains  map[string]Worker
}

var managerInst *manager //nolint:gochecknoglobals

// GetManager returns the singleton implementation of Manager.
func GetManager(opts ...Option) (Manager, error) {
	if managerInst != nil {
		return managerInst, nil
	}

	ctx, ctxCancel := SignalHandledContext(log.Fatalf)

	m := &manager{
		ctx:        ctx,
		ctxCancel:  ctxCancel,
		configPath: "slurpeeth.yaml",
		config: &Config{
			Segments: make(map[string]Segment),
			Domains:  make(map[string]Domain),
		},
		errChan:  make(chan error),
		segments: map[string]Worker{},
		domains:  map[string]Worker{},
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

// Run starts all connections in the configuration and runs until sigint or failure.
func (m *manager) Run() error {
	log.Println("manager run started, setting up segments and domains...")

	err := m.setupSegments()
	if err != nil {
		log.Printf("error creating segments: %s\n", err)

		return err
	}

	err = m.setupDomains()
	if err != nil {
		log.Printf("error creating domains: %s\n", err)

		return err
	}

	m.runSegments()
	m.runDomains()

	err = m.watchConfig()
	if err != nil {
		log.Printf("error setting up config watch: %s\n", err)

		return err
	}

	for err = range m.errChan {
		log.Printf("got error while running things, err: %s\n", err)
	}

	return nil
}

func (m *manager) setupSegments() error {
	for segmentName, segmentConfig := range m.config.Segments {
		segment, err := NewSegmentWorker(segmentConfig, m.errChan)
		if err != nil {
			return err
		}

		err = segment.Bind()
		if err != nil {
			return err
		}

		m.segments[segmentName] = segment
	}

	return nil
}

func (m *manager) setupDomains() error {
	for domainName, domainConfig := range m.config.Domains {
		domain, err := NewDomainWorker(domainConfig, m.errChan)
		if err != nil {
			return err
		}

		err = domain.Bind()
		if err != nil {
			return err
		}

		m.domains[domainName] = domain
	}

	return nil
}

func (m *manager) runSegments() {
	for _, segment := range m.segments {
		segment.Run()
	}
}

func (m *manager) runDomains() {
	for _, domain := range m.domains {
		domain.Run()
	}
}

func (m *manager) shutdownSegments() {
	wg := &sync.WaitGroup{}

	wg.Add(len(m.segments))

	for _, segment := range m.segments {
		go segment.Shutdown(wg)
	}

	wg.Wait()
}

func (m *manager) shutdownDomains() {
	wg := &sync.WaitGroup{}

	wg.Add(len(m.domains))

	for _, domain := range m.domains {
		go domain.Shutdown(wg)
	}

	wg.Wait()
}

func (m *manager) watchConfig() error {
	if !m.liveReload {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					panic("unknown issue handling config watch event")
				}

				log.Printf("got config watch event %q\n", event)

				if event.Name == m.configPath && event.Has(fsnotify.Write) {
					m.reloadConfig()
				}
			case watchErr, ok := <-watcher.Errors:
				if !ok {
					panic("unknown issue handling config watch error")
				}

				m.errChan <- watchErr
			}
		}
	}()

	err = watcher.Add(filepath.Dir(m.configPath))
	if err != nil {
		return err
	}

	return nil
}

func (m *manager) reloadConfig() {
	log.Print("processing config update...")

	configBytes, err := os.ReadFile(m.configPath)
	if err != nil {
		panic(fmt.Sprintf("failed reading config file at path %q, err: %s\n", m.configPath, err))
	}

	newConfig := &Config{}

	err = yaml.Unmarshal(configBytes, newConfig)
	if err != nil {
		panic(fmt.Sprintf("failed unmarshlaing config file, err: %s\n", err))
	}

	if configsEqual(m.config, newConfig) {
		log.Print("previous and current parsed config are equal, nothing to do...")

		return
	}

	log.Print("config has changes, restarting workers...")

	// in the near(?) future we can update just the changed things instead of everything
	m.config = newConfig

	log.Printf("shutting down segments and domains after config update")
	m.shutdownSegments()
	m.shutdownDomains()

	log.Printf("restarting segments and domains after config update")
	m.runSegments()
	m.runDomains()
}

func configsEqual(existingConfig, newConfig *Config) bool {
	if len(existingConfig.Segments) != len(newConfig.Segments) {
		return false
	}

	if len(existingConfig.Domains) != len(newConfig.Domains) {
		return false
	}

	for existingSegmentName, existingSegmentData := range existingConfig.Segments {
		newSegmentData, ok := newConfig.Segments[existingSegmentName]
		if !ok {
			return false
		}

		if !reflect.DeepEqual(existingSegmentData, newSegmentData) {
			return false
		}
	}

	for existingDomainName, existingDomainData := range existingConfig.Domains {
		newDomainData, ok := newConfig.Domains[existingDomainName]
		if !ok {
			return false
		}

		if !reflect.DeepEqual(existingDomainData, newDomainData) {
			return false
		}
	}

	return true
}
