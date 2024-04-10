package systemd

import "github.com/kardianos/service"

type ServiceConfigOption func(*ServiceConfig) error

// WithName sets the name of the systemd process.
func WithName(name string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.name = name
		return nil
	}
}

// WithDisplayName sets the display-name of the systemd process.
func WithDisplayName(dName string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.displayName = dName
		return nil
	}
}

// WithDescription sets the description of the systemd process.
func WithDescription(desc string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.description = desc
		return nil
	}
}

// WithDependencies sets the dependencies of the systemd process.
func WithDependencies(deps []string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.dependencies = deps
		return nil
	}
}

// WithOptions sets the options of the systemd process.
func WithOptions(opts service.KeyValue) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.option = opts
		return nil
	}
}
