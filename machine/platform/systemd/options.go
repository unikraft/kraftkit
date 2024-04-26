package systemd

import "github.com/kardianos/service"

type ServiceConfigOption func(*ServiceConfig) error

// WithName sets the name of systemd service.
func WithName(name string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.Name = name
		return nil
	}
}

// WithDisplayName sets the display-name of systemd service.
func WithDisplayName(dName string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.DisplayName = dName
		return nil
	}
}

// WithDescription sets the description/heading of systemd service.
func WithDescription(desc string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.Description = desc
		return nil
	}
}

// WithDependencies sets the dependencies of systemd service.
func WithDependencies(deps []string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.Dependencies = deps
		return nil
	}
}

// WithEnvVars sets the environment variables for systemd service.
func WithEnvVars(envVars map[string]string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.EnvVars = envVars
		return nil
	}
}

// WithArguments sets the arguments to the command executed by systemd service.
func WithArguments(args []string) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.Arguments = args
		return nil
	}
}

// WithOptions sets the options of systemd service.
func WithOptions(opts service.KeyValue) ServiceConfigOption {
	return func(config *ServiceConfig) error {
		config.Option = opts
		return nil
	}
}
