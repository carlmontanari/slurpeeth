package slurpeeth

import "time"

// Option defines an option for the slurpeeth Manager.
type Option func(m *manager) error

// WithConfigFile provides a config filepath to the manager.
func WithConfigFile(s string) Option {
	return func(m *manager) error {
		m.configPath = s

		return nil
	}
}

// WithLiveReload instructs the manager to watch the config file for changes and "live reload" the
// slurpeeth tunnels.
func WithLiveReload(b bool) Option {
	return func(m *manager) error {
		m.liveReload = b

		return nil
	}
}

// WithListenAddress sets up a slurpeeth manager with a custom listen address.
func WithListenAddress(s string) Option {
	return func(m *manager) error {
		m.address = s

		return nil
	}
}

// WithPort sets up a slurpeeth manager with a custom listen port.
func WithPort(i uint16) Option {
	return func(m *manager) error {
		m.port = i

		return nil
	}
}

// WithDialTimeout sets the maximum timeout for dial attempts for slurpeeth workers -- this is the
// maximum amount of time a worker will continue to attempt to dial a destination. 0 indicates that
// there is no timeout and we'll continually try to dial the connection.
func WithDialTimeout(d time.Duration) Option {
	return func(m *manager) error {
		m.dialTimeout = d

		return nil
	}
}

// WithDebug runs slurpeeth with debug mode.
func WithDebug(b bool) Option {
	return func(m *manager) error {
		m.debug = b

		return nil
	}
}

// WithWorkerRetry sets the retry flavor for workers -- especially with respect to dialing their
// destinations. When worker retry is true, the worker will handle any errors by closing and
// re-dialing a destination; conversely, when false, the worker will simply propagate the errors up
// to the manager which will result in a panic. The default behavior is to retry connections which
// can lead to slurpeeth never failing and just retrying over and over (which is maybe what you
// want, or maybe not!).
func WithWorkerRetry(b bool) Option {
	return func(m *manager) error {
		m.workerRetry = b

		return nil
	}
}
