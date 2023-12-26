package slurpeeth

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

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

				log.Printf("got config watch event %q", event)

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
		panic(fmt.Sprintf("failed reading config file at path %q, err: %s", m.configPath, err))
	}

	newConfig := &Config{}

	err = yaml.Unmarshal(configBytes, newConfig)
	if err != nil {
		panic(fmt.Sprintf("failed unmarshlaing config file, err: %s", err))
	}

	if configsEqual(m.config, newConfig) {
		log.Print("previous and current parsed config are equal, nothing to do...")

		return
	}

	log.Print("config has changes, restarting workers...")

	// in the near(?) future we can update just the changed things instead of everything
	m.config = newConfig

	m.shutdownWorkers()

	log.Print("deleting old workers...")

	m.workers = make(map[uint16]*Worker)

	log.Print("rebuilding workers...")

	err = m.setupWorkers()
	if err != nil {
		panic(fmt.Sprintf("failed re-setting up workers after config change, err: %s", err))
	}

	log.Printf("restarting workers after config update...")

	m.startWorkers()
}

func configsEqual(existingConfig, newConfig *Config) bool {
	if len(existingConfig.Segments) != len(newConfig.Segments) {
		return false
	}

	if reflect.DeepEqual(existingConfig, newConfig) {
		return true
	}

	return false
}
