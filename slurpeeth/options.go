package slurpeeth

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

// WithPort sets up a slurpeeth manager with a custom listen port.
func WithPort(i uint16) Option {
	return func(m *manager) error {
		m.port = i

		return nil
	}
}
