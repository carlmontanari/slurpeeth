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
