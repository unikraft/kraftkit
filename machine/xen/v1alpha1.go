// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package xen

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	ose "os/exec"
	"path/filepath"
	"strings"
	"time"

	zip "api.zip"
	"github.com/shirou/gopsutil/v3/process"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/strings/slices"
	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/internal/logtail"
	"kraftkit.sh/machine/network/macaddr"
	"kraftkit.sh/unikraft/export/v0/ukargparse"
	"kraftkit.sh/unikraft/export/v0/vfscore"
)

const (
	XenLogsDefaultPath = "/var/log/xen"
)

type machineV1alpha1Service struct{}

func NewMachineV1alpha1Service(ctx context.Context) (machinev1alpha1.MachineService, error) {
	return &machineV1alpha1Service{}, nil
}
func (service *machineV1alpha1Service) Create(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.KernelPath == "" {
		return machine, fmt.Errorf("cannot create xen instace without a kernel")
	}

	if !slices.Contains([]string{"arm64", "x86_64", "amd64", "arm"}, machine.Spec.Architecture) {
		return machine, fmt.Errorf("unsupported architecture %s", machine.Spec.Architecture)
	}

	if _, err := os.Stat(machine.Status.KernelPath); err != nil && os.IsNotExist(err) {
		return machine, fmt.Errorf("supplied kernel path does not exist: %s", machine.Status.KernelPath)
	}

	if _, err := ose.LookPath(XenToolsBin); err != nil {
		return machine, fmt.Errorf("xl not found: %w", err)
	}

	if machine.ObjectMeta.UID == "" {
		machine.ObjectMeta.UID = uuid.NewUUID()
	}

	machine.Status.State = machinev1alpha1.MachineStateUnknown

	if len(machine.Status.StateDir) == 0 {
		machine.Status.StateDir = filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, string(machine.ObjectMeta.UID))
	}

	if err := os.MkdirAll(machine.Status.StateDir, fs.ModeSetgid|0o775); err != nil {
		return machine, err
	}

	if len(machine.Status.LogFile) == 0 {
		machine.Status.LogFile = filepath.Join(machine.Status.StateDir, "machine.log")
	}

	if machine.Spec.Resources.Requests == nil {
		machine.Spec.Resources.Requests = make(corev1.ResourceList, 2)
	}

	if machine.Spec.Resources.Requests.Memory().Value() == 0 {
		quantity, err := resource.ParseQuantity(string(XenMemoryDefault) + "Mi")
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}

		machine.Spec.Resources.Requests[corev1.ResourceMemory] = quantity
	}

	if machine.Spec.Resources.Requests.Cpu().Value() == 0 {
		quantity, err := resource.ParseQuantity(string(XenCPUsDefault))
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}

		machine.Spec.Resources.Requests[corev1.ResourceCPU] = quantity
	}

	// TODO(andreistan26): Check if the name is already in use as a xen domain
	xenOpts := []XenOption{
		WithCpu(int(machine.Spec.Resources.Requests.Cpu().Value())),
		WithMemory(int(machine.Spec.Resources.Requests.Memory().Value() / XenMemoryScale)),
		WithKernel(machine.Status.KernelPath),
		// Check if UID is used by another xen domain
		WithUuid(string(machine.ObjectMeta.UID)),
		WithName(string(machine.ObjectMeta.Name)),
	}

	if len(machine.Status.InitrdPath) > 0 {
		xenOpts = append(xenOpts, WithRamdisk(machine.Status.InitrdPath))
	}

	if len(machine.Spec.Ports) > 0 {
		// TODO(andreistan26): Add port mapping
		// make sure that net.ipv4.ip_forward = 1 is set, will need interaction with sysctl
		return machine, fmt.Errorf("mapping ports is not supported for xen")
	}

	// TODO(andreistan26): Add args
	kernelArgs, err := ukargparse.Parse(machine.Spec.KernelArgs...)
	if err != nil {
		return machine, err
	}

	if len(machine.Spec.Networks) > 0 {
		startMac, err := macaddr.GenerateMacAddress(true)
		if err != nil {
			return machine, err
		}

		i := 0
		for _, network := range machine.Spec.Networks {
			for _, iface := range network.Interfaces {
				mac := iface.Spec.MacAddress
				if mac == "" {
					startMac = macaddr.IncrementMacAddress(startMac)
					mac = startMac.String()
				}

				xenOpts = append(xenOpts,
					WithNetwork(NetworkSpec{
						Mac:    mac,
						Ip:     fmt.Sprintf("%s %s %s", iface.Spec.IP, network.Netmask, network.Gateway),
						Bridge: network.IfName,
					}),
				)
				i++
			}
		}
	}

	var fstab []string

	// TODO(andreistan26): Check if installed xen supports 9pfs
	for i, vol := range machine.Spec.Volumes {
		switch vol.Spec.Driver {
		case "9pfs":
			mounttag := fmt.Sprintf("fs%d", i+1)
			xenOpts = append(xenOpts,
				WithP9(P9Spec{
					Tag:  mounttag,
					Path: vol.Spec.Source,
				}),
			)

			fstab = append(fstab, vfscore.NewFstabEntry(
				mounttag,
				vol.Spec.Destination,
				vol.Spec.Driver,
				"",
				"",
			).String())
		default:
			return machine, fmt.Errorf("unsupported Xen volume driver: %v", vol.Spec.Driver)
		}
	}

	if len(fstab) > 0 {
		kernelArgs = append(kernelArgs, vfscore.ParamVfsFstab.WithValue(fstab))
	}

	args := kernelArgs.Strings()
	if len(args) > 0 {
		args = append(args, "--")
	}

	args = append(args, machine.Spec.ApplicationArgs...)
	xenOpts = append(xenOpts, WithArgs(strings.Join(args, " ")))

	xenLogPath := filepath.Join(XenLogsDefaultPath, fmt.Sprintf("xl-%s.log", machine.Name))
	if err != nil {
		return machine, err
	}

	fi, err := os.Open(machine.Status.LogFile)
	if err != nil {
		return machine, err
	}

	defer fi.Close()

	config, err := NewXenConfig(xenOpts...)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not create xen config: %w", err)
	}

	machine.Status.PlatformConfig = config

	if err := config.WriteConfigFile(filepath.Join(machine.Status.StateDir, "xen.cfg")); err != nil {
		return machine, fmt.Errorf("could not write xen config file: %w", err)
	}

	e, err := XenCreate(
		ctx,
		XenCreateExecConfig{
			StartPaused: true,
		},
		filepath.Join(machine.Status.StateDir, "xen.cfg"),
	)
	if err != nil {
		return machine, err
	}

	process, err := exec.NewProcessFromExecutable(e,
		exec.WithStdout(fi),
		exec.WithDetach(true),
	)
	if err != nil {
		return machine, err
	}

	machine.CreationTimestamp = metav1.Now()

	if err = process.Start(ctx); err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed

		if errLog, err2 := os.ReadFile(xenLogPath); err2 == nil {
			err = errors.Join(fmt.Errorf(strings.TrimSpace(string(errLog))), err)
		}

		return machine, fmt.Errorf("could not create xen domain: %w", err)
	}

	// pid of the `xl create` process, vm will not die if this process dies
	// probably not needed
	pid, err := process.Pid()
	if err != nil {
		return nil, fmt.Errorf("could not get xen pid: %v", err)
	}

	domID, err := XenID(ctx, machine.Name)
	if domID < 0 || err != nil {
		return machine, fmt.Errorf("could not get domain id: %w", err)
	}

	machine.Status.PlatformConfig.(*XenConfig).DomID = domID

	machine.Status.Pid = int32(pid)
	machine.Status.State = machinev1alpha1.MachineStateCreated

	return machine, nil
}
func (service *machineV1alpha1Service) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.PlatformConfig == nil {
		return machine, fmt.Errorf("machine has no platform config")
	}

	config := machine.Status.PlatformConfig.(*XenConfig)

	if err := XenUnpause(ctx, config.DomID); err != nil {
		return machine, fmt.Errorf("could not start xen domain: %w", err)
	}

	machine.Status.State = machinev1alpha1.MachineStateRunning
	machine.Status.StartedAt = time.Now()

	return machine, nil
}
func (service *machineV1alpha1Service) Pause(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.PlatformConfig == nil {
		return machine, fmt.Errorf("machine has no platform config")
	}

	config, ok := machine.Status.PlatformConfig.(*XenConfig)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	if err := XenPause(ctx, config.DomID); err != nil {
		return machine, fmt.Errorf("could not start xen domain: %w", err)
	}

	machine.Status.State = machinev1alpha1.MachineStatePaused

	return machine, nil
}
func (service *machineV1alpha1Service) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.State == machinev1alpha1.MachineStateExited {
		return machine, nil
	}

	// TODO might not need this
	process, err := process.NewProcess(machine.Status.Pid)
	if err != nil {
		return machine, fmt.Errorf("could not get process: %w", err)
	}

	if err := process.Kill(); err != nil {
		return machine, fmt.Errorf("could not kill process: %w", err)
	}

	cfg, ok := machine.Status.PlatformConfig.(*XenConfig)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	// TODO(andreistan26): replace with shutdown after adding support on unikraft's side
	err = XenDestroy(ctx, cfg.DomID)
	if err != nil {
		return machine, fmt.Errorf("could not destroy xen domain: %w", err)
	}

	machine.Status.State = machinev1alpha1.MachineStateExited
	machine.Status.ExitedAt = time.Now()
	machine.Status.Pid = 0
	cfg.DomID = -1

	return machine, nil
}
func (service *machineV1alpha1Service) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	panic("not implemented: kraftkit.sh/machine/qemu.machineV1alpha1Service.Update")
}
func (service *machineV1alpha1Service) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	cfg := machine.Status.PlatformConfig.(*XenConfig)
	if cfg != nil {
		return machine, fmt.Errorf("machine has no platform config")
	}

	err := os.RemoveAll(machine.Status.StateDir)

	return nil, err
}
func (service *machineV1alpha1Service) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	cfg, ok := machine.Status.PlatformConfig.(*XenConfig)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	currentXenState, err := XenGetState(ctx, cfg.DomID)
	if err != nil {
		return machine, fmt.Errorf("could not get xen state: %w", err)
	}

	exitCode := -1

	switch currentXenState {
	case XenStateRunning, XenStateBlocked:
		machine.Status.State = machinev1alpha1.MachineStateRunning
		machine.Status.ExitCode = -1
	case XenStatePaused:
		machine.Status.State = machinev1alpha1.MachineStatePaused
		machine.Status.ExitCode = -1
	case XenStateCrashed:
		machine.Status.State = machinev1alpha1.MachineStateErrored
		machine.Status.ExitCode = 1
	case XenStateShutdown, XenStateDying:
		machine.Status.State = machinev1alpha1.MachineStateExited
		machine.Status.ExitCode = 0
	default:
		machine.Status.State = machinev1alpha1.MachineStateUnknown
		machine.Status.ExitCode = -1
	}

	if machine.Status.ExitCode != -1 && machine.Status.ExitedAt.IsZero() {
		machine.Status.ExitedAt = time.Now()
	}

	return machine, nil

}

func (service *machineV1alpha1Service) List(ctx context.Context, machines *machinev1alpha1.MachineList) (*machinev1alpha1.MachineList, error) {
	cached := machines.Items
	machines.Items = []zip.Object[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus]{}

	for _, machine := range cached {
		machine, err := service.Get(ctx, &machine)
		if err != nil {
			machines.Items = cached
			return machines, err
		}

		machines.Items = append(machines.Items, *machine)
	}

	return machines, nil
}

func (service *machineV1alpha1Service) Watch(ctx context.Context, machine *machinev1alpha1.Machine) (chan *machinev1alpha1.Machine, chan error, error) {
	events := make(chan *machinev1alpha1.Machine)
	errs := make(chan error)

	cfg, ok := machine.Status.PlatformConfig.(*XenConfig)
	if !ok {
		return events, errs, fmt.Errorf("machine has no platform config")
	}

	cfgCopy := *cfg
	client, err := XenCreateClient()
	if err != nil {
		return events, errs, fmt.Errorf("could not create xen watch client: %w", err)
	}

	client, err := client.Watch()
	defer func() {
		if err := client.UnWatch(); err != nil {
			errs <- err
		}
		if err := client.Close(); err != nil {
			errs <- err
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				errs <- ctx.Err()
				return

			default:
			}
		}
	}()

	return events, errs, nil
}
func (service *machineV1alpha1Service) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	return logtail.NewLogTail(ctx, machine.Status.LogFile)
}
