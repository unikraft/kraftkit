package systemd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
)

type ServiceConfig struct {
	Name         string            `json:"Name"`
	DisplayName  string            `json:"DisplayName,omitempty"`
	Description  string            `json:"Description"`
	Dependencies []string          `json:"Dependencies,omitempty"`
	Arguments    []string          `json:"Arguments"`
	Option       service.KeyValue  `json:"Option,omitempty"`
	EnvVars      map[string]string `json:"EnvVars,omitempty"`

	service service.Service
	logger  service.Logger
}

// NewMachineV1alpha1ServiceSystemdWrapper creates a new systemd service.
func NewMachineV1alpha1ServiceSystemdWrapper(ctx context.Context, opts ...ServiceConfigOption) (ServiceConfig, error) {
	svcConfig := ServiceConfig{}

	if uid := os.Getuid(); uid != 0 {
		return svcConfig, fmt.Errorf("requires root permission")
	}

	for _, opt := range opts {
		if err := opt(&svcConfig); err != nil {
			return svcConfig, err
		}
	}

	// Creates service config file at `$HOME/.local/share/kraftkit/runtime/systemd/`.
	byteJson, err := json.Marshal(svcConfig)
	if err != nil {
		return svcConfig, err
	}

	systemdDir := filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "systemd")
	if err = os.MkdirAll(systemdDir, 0700); err != nil {
		return svcConfig, err
	}

	if err = os.WriteFile(
		filepath.Join(systemdDir, svcConfig.Name+".json"),
		byteJson,
		0644,
	); err != nil {
		return svcConfig, err
	}

	err = svcConfig.initService()
	return svcConfig, err
}

// GetMachineV1alpha1ServiceSystemdWrapper returns existing systemd service.
func GetMachineV1alpha1ServiceSystemdWrapper(ctx context.Context, name string) (ServiceConfig, error) {
	svcConfig := ServiceConfig{}

	if uid := os.Getuid(); uid != 0 {
		return svcConfig, fmt.Errorf("requires root permission")
	}

	byteJson, err := os.ReadFile(filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "systemd", name+".json"))
	if err != nil {
		// Return `Service is not pre-configured`
		return svcConfig, err
	}

	if err = json.Unmarshal(byteJson, &svcConfig); err != nil {
		return svcConfig, err
	}

	err = svcConfig.initService()
	return svcConfig, err
}

// initService create systemd service.
func (sc *ServiceConfig) initService() error {
	var err error
	sys := service.ChosenSystem()
	sc.service, err = sys.New(&startStop{}, &service.Config{
		Name:         sc.Name,
		DisplayName:  sc.DisplayName,
		Description:  sc.Description,
		Dependencies: sc.Dependencies,
		Arguments:    sc.Arguments,
		EnvVars:      sc.EnvVars,
		Option:       sc.Option,
	})
	if err != nil {
		return err
	}

	errs := make(chan error, 5)
	sc.logger, err = sc.service.SystemLogger(errs)
	if err != nil {
		return err
	}

	return nil
}

// Create Install setups up the given service in the OS service manager.
// This may require greater rights. Will return an error if it is already installed.
func (sc ServiceConfig) Create(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if uid := os.Getuid(); uid != 0 {
		return machine, fmt.Errorf("requires root permission")
	}

	err := sc.service.Install()
	if err != nil {
		return machine, err
	}

	return machine, nil
}

// Start signals to the OS service manager the given service should start.
func (sc ServiceConfig) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Start()` -> to start running systemd process, It also checks for the user permission same as above first.
	if uid := os.Getuid(); uid != 0 {
		return machine, fmt.Errorf("requires root permission")
	}

	if err := sc.service.Start(); err != nil {
		return machine, nil
	}

	return machine, nil
}

func (sc ServiceConfig) Pause(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// This fuction can be upgraded in future as per need
	// But right now it does same as Stop().
	return sc.Stop(ctx, machine)
}

// Stop signals to the OS service manager the given service should stop.
func (sc ServiceConfig) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Stop()` -> to stop running systemd process, It also checks for the user permission same as above first.
	if uid := os.Getuid(); uid != 0 {
		return machine, fmt.Errorf("requires root permission")
	}

	if err := sc.service.Stop(); err != nil {
		return machine, nil
	}
	return machine, nil
}

func (sc ServiceConfig) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Stop & Delete the existing service if possible and Create & Start (in case service was running)
	// using Stop(), Delete(), Create() & Start() respectively.
	return machine, nil
}

// Delete removes the given service from the OS service manager.
// This may require greater rights. Will return an error if the service is not present.
func (sc ServiceConfig) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Delete()` -> to uninstall systemd process,
	// It also checks for the user permission same as above first & stop the process if it's in running state.
	if uid := os.Getuid(); uid != 0 {
		return machine, fmt.Errorf("requires root permission")
	}

	if err := sc.service.Uninstall(); err != nil {
		return machine, nil
	}
	return machine, nil
}

func (sc ServiceConfig) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Get()` -> to return the systemd process with following info: `name`, `status` & etc.
	return machine, nil
}

func (sc ServiceConfig) List(ctx context.Context, machineList *machinev1alpha1.MachineList) (*machinev1alpha1.MachineList, error) {
	return machineList, nil
}

func (sc ServiceConfig) Watch(ctx context.Context, machine *machinev1alpha1.Machine) (chan *machinev1alpha1.Machine, chan error, error) {
	events := make(chan *machinev1alpha1.Machine)
	return events, nil, nil
}

func (sc ServiceConfig) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	logs := make(chan string)
	return logs, nil, nil
}
