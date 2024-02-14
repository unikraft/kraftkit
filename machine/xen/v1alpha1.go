// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package xen

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	zip "api.zip"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/logtail"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network/macaddr"
	"kraftkit.sh/unikraft/export/v0/ukargparse"
	"kraftkit.sh/unikraft/export/v0/vfscore"
	"xenbits.xenproject.org/git-http/xen.git/tools/golang/xenlight"
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
		quantity, err := resource.ParseQuantity(fmt.Sprintf("%d", XenMemoryDefault) + "Mi")
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}

		machine.Spec.Resources.Requests[corev1.ResourceMemory] = quantity
	}

	if machine.Spec.Resources.Requests.Cpu().Value() == 0 {
		quantity, err := resource.ParseQuantity(fmt.Sprintf("%d", XenCPUsDefault))
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}

		machine.Spec.Resources.Requests[corev1.ResourceCPU] = quantity
	}

	// TODO(andreistan26): Check if the name is already in use as a xen domain
	xenOpts := []XenOption{
		WithCpu(int(machine.Spec.Resources.Requests.Cpu().Value())),
		WithMemoryKb(uint64(machine.Spec.Resources.Requests.Memory().Value() / XenMemoryScale)),
		WithKernel(machine.Status.KernelPath),
		WithUuid(string(machine.ObjectMeta.UID)),
		WithName(string(machine.ObjectMeta.Name)),
	}

	if len(machine.Status.InitrdPath) > 0 {
		xenOpts = append(xenOpts, WithRamdisk(machine.Status.InitrdPath))
	}

	// TODO(andreistan26): Add port mapping
	if len(machine.Spec.Ports) > 0 {
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

				//TODO(andreistan26): refactor this
				nic, err := xenlight.NewDeviceNic()
				if err != nil {
					return nil, err
				}

				nic.Ip = fmt.Sprintf("%s %s %s", iface.Spec.IP, network.Netmask, network.Gateway)
				nic.Bridge = network.IfName
				nic.Mac = xenlight.Mac([]byte(mac))

				xenOpts = append(xenOpts, WithNetwork(*nic))
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
			// TODO(andreistan26): refactor this
			p9Dev, err := xenlight.NewDeviceP9()
			if err != nil {
				return nil, err
			}

			p9Dev.Tag = mounttag
			p9Dev.Path = vol.Spec.Source

			xenOpts = append(xenOpts,
				WithP9(*p9Dev),
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
	xenOpts = append(xenOpts, WithArgs(args))

	xenLogPath := filepath.Join(XenLogsDefaultPath, fmt.Sprintf("xl-%s.log", machine.Name))
	if err != nil {
		return machine, err
	}
	machine.Status.LogFile = xenLogPath

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

	xenCtx, err := xenlight.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create xen context: %w", err)
	}

	domID, err := xenCtx.DomainCreateNew(config)
	if err != nil {
		return machine, fmt.Errorf("could not create xen domain: %w", err)
	}

	// TODO(andreistan26): Check if create-pause can be made as a transation/atomic
	if err = xenCtx.DomainPause(domID); err != nil {
		return machine, fmt.Errorf("could not pause xen domain: %w", err)
	}

	config.CInfo.Domid = domID

	machine.CreationTimestamp = metav1.Now()
	machine.Status.PlatformConfig = &XenConfig{
		DomainConfig: config,
		XenCtx:       xenCtx,
	}
	machine.Status.State = machinev1alpha1.MachineStateCreated

	return machine, nil
}
func (service *machineV1alpha1Service) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.PlatformConfig == nil {
		return machine, fmt.Errorf("machine has no platform config")
	}

	config := machine.Status.PlatformConfig.(*XenConfig)

	config.XenCtx.DomainUnpause(config.DomainConfig.CInfo.Domid)

	machine.Status.State = machinev1alpha1.MachineStateRunning
	machine.Status.StartedAt = time.Now()

	// Start appending pts output to logfile: pts -> chan -> log file
	go func() {
		pts, err := config.XenCtx.PrimaryConsoleGetTty(uint32(config.DomainConfig.CInfo.Domid))
		if err != nil {
			log.G(ctx).Errorf("could not get xen domain pts: %v", err)
			return
		}
		ptsChan, errChan, err := logtail.NewLogTail(ctx, pts)
		if err != nil {
			log.G(ctx).Errorf("could not tail xen domain pts: %v", err)
			return
		}

		fd, err := os.OpenFile(machine.Status.LogFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.G(ctx).Errorf("log file not found after create: %v", err)
			return
		}

		for {
			select {
			case err := <-errChan:
				log.G(ctx).Errorf("could not tail log file: %v", err)
			case line := <-ptsChan:
				_, err := fd.Write([]byte(line))
				if err != nil {
					log.G(ctx).Errorf("could not write to log file: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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

	if err := config.XenCtx.DomainUnpause(config.DomainConfig.CInfo.Domid); err != nil {
		return machine, fmt.Errorf("could not unpause xen domain: %w", err)
	}

	machine.Status.State = machinev1alpha1.MachineStatePaused

	return machine, nil
}
func (service *machineV1alpha1Service) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.State == machinev1alpha1.MachineStateExited {
		return machine, nil
	}

	config, ok := machine.Status.PlatformConfig.(*XenConfig)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	if err := config.XenCtx.DomainDestroy(config.DomainConfig.CInfo.Domid); err != nil {
		return machine, fmt.Errorf("could not destroy xen domain: %w", err)
	}

	machine.Status.State = machinev1alpha1.MachineStateExited
	machine.Status.ExitedAt = time.Now()

	return machine, nil
}
func (service *machineV1alpha1Service) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	panic("not implemented: kraftkit.sh/machine/qemu.machineV1alpha1Service.Update")
}
func (service *machineV1alpha1Service) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	err := os.RemoveAll(machine.Status.StateDir)

	return nil, err
}
func (service *machineV1alpha1Service) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	config, ok := machine.Status.PlatformConfig.(*XenConfig)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	machine.Status.State = machinev1alpha1.MachineStateUnknown
	machine.Status.ExitCode = -1

	dominfo, err := config.XenCtx.DomainInfo(config.DomainConfig.CInfo.Domid)
	if err != nil {
		return machine, fmt.Errorf("could not get xen domain info: %w", err)
	}

	machine.Status.ExitCode, machine.Status.State = getXenState(dominfo)

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
	config, ok := machine.Status.PlatformConfig.(*XenConfig)
	if !ok {
		return nil, nil, fmt.Errorf("machine has no platform config")
	}

	w, err := NewWatcher(config.DomainConfig.CInfo.Domid)
	if err != nil {
		return nil, nil, err
	}

	watch, err := w.Watch(ctx)
	if err != nil {
		return nil, nil, err
	}

	events := make(chan *machinev1alpha1.Machine)
	errs := make(chan error)

	go func() {
		intialMachine, err := service.Get(ctx, machine)
		if err != nil {
			errs <- err
		}
		events <- intialMachine

		for {
			select {
			case <-ctx.Done():
				w.Close()
				return
			case <-watch:
				machine, err := service.Get(ctx, machine)
				if err != nil {
					errs <- err
					continue
				}

				events <- machine
			}
		}
	}()

	return events, errs, nil
}
func (service *machineV1alpha1Service) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	return logtail.NewLogTail(ctx, machine.Status.LogFile)
}

func getXenState(domInfo *xenlight.Dominfo) (int, machinev1alpha1.MachineState) {
	if domInfo.Blocked || domInfo.Running {
		return -1, machinev1alpha1.MachineStateRunning
	} else if domInfo.Paused {
		return -1, machinev1alpha1.MachineStatePaused
	} else if domInfo.Dying {
		return 0, machinev1alpha1.MachineStateExited
	} else if domInfo.Shutdown {
		switch domInfo.ShutdownReason {
		case xenlight.ShutdownReasonCrash:
			return 1, machinev1alpha1.MachineStateErrored
		case xenlight.ShutdownReasonPoweroff:
			return 0, machinev1alpha1.MachineStateExited
		}
	}

	return -1, machinev1alpha1.MachineStateUnknown
}
