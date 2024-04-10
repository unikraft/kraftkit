package systemd

import (
	"context"
	"fmt"
	"os"

	"github.com/kardianos/service"
	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
)

type ServiceConfig struct {
	name         string
	displayName  string
	description  string
	dependencies []string
	option       service.KeyValue

	service service.Service
	logger  service.Logger
}

func NewMachineV1alpha1ServiceSystemdWrapper(ctx context.Context, opts ...ServiceConfigOption) (ServiceConfig, error) {
	config := ServiceConfig{}

	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return config, err
		}
	}

	return config, nil
}

func (sc ServiceConfig) Create(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var err error
	uid := os.Getuid()
	if uid != 0 {
		return machine, fmt.Errorf("requires root permission")
	}

	svcConfig := &service.Config{
		Name:         sc.name,
		DisplayName:  sc.displayName,
		Description:  sc.description,
		Dependencies: sc.dependencies,
		Option:       sc.option,
	}
	sys := service.ChosenSystem()
	sc.service, err = sys.New(&startStop{}, svcConfig)
	if err != nil {
		return machine, err
	}

	errs := make(chan error, 5)
	sc.logger, err = sc.service.Logger(errs)
	if err != nil {
		return machine, err
	}

	err = sc.service.Install()
	if err != nil {
		return machine, err
	}

	return machine, nil
}

func (sc ServiceConfig) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Start()` -> to start running systemd process, It also checks for the user permission same as above first.
	return &machinev1alpha1.Machine{}, nil
}

func (sc ServiceConfig) Pause(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	return &machinev1alpha1.Machine{}, nil
}

func (sc ServiceConfig) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Stop()` -> to stop running systemd process, It also checks for the user permission same as above first.
	return &machinev1alpha1.Machine{}, nil
}

func (sc ServiceConfig) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	return &machinev1alpha1.Machine{}, nil
}

func (sc ServiceConfig) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Delete()` -> to uninstall systemd process,
	// It also checks for the user permission same as above first & stop the process if it's in running state.
	return &machinev1alpha1.Machine{}, nil
}

func (sc ServiceConfig) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	// Implement `Get()` -> to return the systemd process with following info: `name`, `status` & etc.
	return &machinev1alpha1.Machine{}, nil
}

func (sc ServiceConfig) List(ctx context.Context, machineList *machinev1alpha1.MachineList) (*machinev1alpha1.MachineList, error) {
	return &machinev1alpha1.MachineList{}, nil
}

func (sc ServiceConfig) Watch(ctx context.Context, machine *machinev1alpha1.Machine) (chan *machinev1alpha1.Machine, chan error, error) {
	events := make(chan *machinev1alpha1.Machine)
	return events, nil, nil
}

func (sc ServiceConfig) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	logs := make(chan string)
	return logs, nil, nil
}
