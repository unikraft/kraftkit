// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

//go:build xen
// +build xen

package xen

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	zip "api.zip"
	"github.com/acorn-io/baaah/pkg/merr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/logtail"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network/macaddr"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/export/v0/ukargparse"
	"kraftkit.sh/unikraft/export/v0/uknetdev"
	"kraftkit.sh/unikraft/export/v0/vfscore"
	"xenbits.xenproject.org/git-http/xen.git/tools/golang/xenlight"
)

const (
	XenMemoryScale   = 1024
	XenMemoryDefault = 64
	XenCPUsDefault   = 1
)

type machineV1alpha1Service struct{}

func NewMachineV1alpha1Service(ctx context.Context) (machinev1alpha1.MachineService, error) {
	return &machineV1alpha1Service{}, nil
}

func (service *machineV1alpha1Service) Create(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	cfg, err := NewXenConfig()
	if err != nil {
		return nil, err
	}

	if machine.Status.KernelPath == "" {
		return machine, fmt.Errorf("cannot create xen instace without a kernel")
	}

	if arch.ArchitectureByName(machine.Spec.Architecture) == arch.ArchitectureUnknown {
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

	fd, err := os.Create(machine.Status.LogFile)
	if err != nil {
		return machine, err
	}
	defer fd.Close()

	if machine.Spec.Resources.Requests == nil {
		machine.Spec.Resources.Requests = make(corev1.ResourceList, 2)
	}

	if machine.Spec.Resources.Requests.Memory().Value() == 0 {
		quantity, err := resource.ParseQuantity(fmt.Sprintf("%dMi", XenMemoryDefault))
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

	if machine.Spec.Resources.Requests.Memory().Value() < XenMemoryScale {
		return machine, fmt.Errorf("memory must be greater than %d bytes", XenMemoryScale)
	}

	cfg.BInfo.MaxVcpus = int(machine.Spec.Resources.Requests.Cpu().Value())
	cfg.BInfo.MaxMemkb = uint64(machine.Spec.Resources.Requests.Memory().Value() / XenMemoryScale)
	cfg.BInfo.Kernel = machine.Status.KernelPath
	cfg.CInfo.Name = string(machine.ObjectMeta.Name)
	cfg.CInfo.Type = xenlight.DomainTypePv

	if machine.Status.InitrdPath != "" {
		cfg.BInfo.Ramdisk = machine.Status.InitrdPath
	}

	if len(machine.Spec.Ports) > 0 {
		return machine, fmt.Errorf("mapping ports is not supported for xen")
	}

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

				nic, err := xenlight.NewDeviceNic()
				if err != nil {
					return nil, fmt.Errorf("could not create xen nic: %v", err)
				}

				// TODO(andreistan26): Check if xen accepts CIDR notation
				nic.Ip = fmt.Sprintf("%s %s %s", strings.Split(iface.Spec.CIDR, "/")[0], network.Netmask, network.Gateway)
				nic.Bridge = network.IfName
				nic.Mac = xenlight.Mac([]byte(mac))

				cfg.Nics = append(cfg.Nics, *nic)
				kernelArgs = append(kernelArgs,
					uknetdev.NewParamIp().WithValue(uknetdev.NetdevIp{
						CIDR:     iface.Spec.CIDR,
						Gateway:  network.Gateway,
						DNS0:     iface.Spec.DNS0,
						DNS1:     iface.Spec.DNS1,
						Hostname: iface.Spec.Hostname,
						Domain:   iface.Spec.Domain,
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
		case "9p":
		case "9pfs":
			mounttag := fmt.Sprintf("fs%d", i+1)
			p9Dev, err := xenlight.NewDeviceP9()
			if err != nil {
				return nil, err
			}

			p9Dev.Tag = mounttag
			p9Dev.Path = vol.Spec.Source
			p9Dev.SecurityModel = "none"

			cfg.P9S = append(cfg.P9S, *p9Dev)

			fstab = append(fstab, vfscore.NewFstabEntry(
				mounttag,
				vol.Spec.Destination,
				vol.Spec.Driver,
				"",
				"",
				"mkmp",
			).String())
		case "initrd":
			fstab = append(fstab, vfscore.NewFstabEntry(
				"initrd0",
				vol.Spec.Destination,
				"extract",
				"",
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
	cfg.BInfo.Cmdline = strings.Join(args, " ")

	xenCtx, err := xenlight.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create xen context: %v", err)
	}
	defer xenCtx.Close()

	machine.CreationTimestamp = metav1.Now()

	domID, err := xenCtx.DomainCreateNew(cfg)
	if err != nil {
		return machine, fmt.Errorf("could not create xen domain: %v", err)
	}

	machine.Status.PlatformConfig = domID

	machine.Status.State = machinev1alpha1.MachineStateCreated

	return machine, nil
}

func (service *machineV1alpha1Service) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.PlatformConfig == nil {
		return machine, fmt.Errorf("machine has no platform config")
	}

	domId, ok := machine.Status.PlatformConfig.(xenlight.Domid)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	xenCtx, err := xenlight.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create xen context: %v", err)
	}
	defer xenCtx.Close()

	err = xenCtx.DomainUnpause(domId)
	if err != nil {
		return machine, fmt.Errorf("could not unpause xen domain: %v", err)
	}

	machine.Status.State = machinev1alpha1.MachineStateRunning
	machine.Status.StartedAt = time.Now()

	return machine, nil
}

func (service *machineV1alpha1Service) Pause(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.PlatformConfig == nil {
		return machine, fmt.Errorf("machine has no platform config")
	}

	domId, ok := machine.Status.PlatformConfig.(xenlight.Domid)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	xenCtx, err := xenlight.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create xen context: %v", err)
	}
	defer xenCtx.Close()

	if err := xenCtx.DomainPause(domId); err != nil {
		return machine, fmt.Errorf("could not unpause xen domain: %v", err)
	}

	machine.Status.State = machinev1alpha1.MachineStatePaused

	return machine, nil
}

func (service *machineV1alpha1Service) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.State == machinev1alpha1.MachineStateExited {
		return machine, nil
	}

	domId, ok := machine.Status.PlatformConfig.(xenlight.Domid)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	xenCtx, err := xenlight.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create xen context: %v", err)
	}
	defer xenCtx.Close()
	if err := xenCtx.DomainDestroy(domId); err != nil {
		return machine, fmt.Errorf("could not destroy xen domain: %v", err)
	}

	machine.Status.State = machinev1alpha1.MachineStateExited
	machine.Status.ExitedAt = time.Now()

	return machine, nil
}

func (service *machineV1alpha1Service) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	panic("not implemented: kraftkit.sh/machine/xen.machineV1alpha1Service.Update")
}

func (service *machineV1alpha1Service) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs merr.Errors

	errs = append(errs, os.RemoveAll(machine.Status.StateDir))
	errs = append(errs, os.RemoveAll(machine.Status.LogFile))

	return nil, errs.Err()
}

func (service *machineV1alpha1Service) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	domId, ok := machine.Status.PlatformConfig.(xenlight.Domid)
	if !ok {
		return machine, fmt.Errorf("machine has no platform config")
	}

	machine.Status.State = machinev1alpha1.MachineStateUnknown
	machine.Status.ExitCode = -1

	xenCtx, err := xenlight.NewContext()
	if err != nil {
		return nil, fmt.Errorf("could not create xen context: %v", err)
	}
	defer xenCtx.Close()

	dominfo := &xenlight.Dominfo{}

	// Should be done with xenCtx.DomainInfo, but it currently does not work
	doms := xenCtx.ListDomain()
	if err != nil {
		return machine, fmt.Errorf("could not list xen domains: %v", err)
	}

	index := slices.IndexFunc[[]xenlight.Dominfo, xenlight.Dominfo](doms, func(dominfo xenlight.Dominfo) bool {
		return dominfo.Domid == domId
	})

	// if index is not present in the list probably it crashed
	if index == -1 {
		dominfo = &xenlight.Dominfo{Shutdown: true, ShutdownReason: xenlight.ShutdownReasonPoweroff}
	} else {
		dominfo = &doms[index]
	}

	machine.Status.ExitCode, machine.Status.State = getXenState(dominfo)

	if machine.Status.ExitCode != -1 && machine.Status.ExitedAt.IsZero() {
		machine.Status.ExitedAt = time.Now()
	}

	return machine, nil
}

func (service *machineV1alpha1Service) List(ctx context.Context, machines *machinev1alpha1.MachineList) (*machinev1alpha1.MachineList, error) {
	cached := machines.Items
	machines.Items = make([]zip.Object[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus], len(cached))
	for i, machine := range cached {
		machine, err := service.Get(ctx, &machine)
		if err != nil {
			machines.Items = cached
			return machines, err
		}

		machines.Items[i] = *machine
	}

	return machines, nil
}

func (service *machineV1alpha1Service) Watch(ctx context.Context, machine *machinev1alpha1.Machine) (chan *machinev1alpha1.Machine, chan error, error) {
	domId, ok := machine.Status.PlatformConfig.(xenlight.Domid)
	if !ok {
		return nil, nil, fmt.Errorf("machine has no platform config")
	}

	w, err := NewWatcher(domId)
	if err != nil {
		return nil, nil, err
	}

	// signals when xenstore tree was updated for domain
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
	domId, ok := machine.Status.PlatformConfig.(xenlight.Domid)
	if !ok {
		return nil, nil, fmt.Errorf("machine has no platform config")
	}

	xenCtx, err := xenlight.NewContext()
	if err != nil {
		return nil, nil, fmt.Errorf("could not create xen context: %v", err)
	}
	defer xenCtx.Close()

	pts, err := xenCtx.PrimaryConsoleGetTty(uint32(domId))
	if err != nil {
		return nil, nil, fmt.Errorf("could not get xen domain pts: %v", err)
	}

	// Start appending pts output to logfile: pts -> chan -> log file
	go func() {
		ptsChan := make(chan []byte, 10)
		errChan := make(chan error)

		ptsFD, err := os.OpenFile(pts, os.O_RDONLY, 0o644)
		if err != nil {
			log.G(ctx).Errorf("could not open xen domain pts: %v", err)
			return
		}

		logFD, err := os.OpenFile(machine.Status.LogFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.G(ctx).Errorf("log file not found after create: %v", err)
			return
		}

		go func() {
			for {
				buf := make([]byte, 1024*8)
				n, err := ptsFD.Read(buf)
				if err != nil {
					if err != os.ErrClosed {
						errChan <- err
					}

					if err == io.EOF {
						continue
					}

					return
				}
				ptsChan <- buf[:n]
			}
		}()

		for {
			select {
			case err := <-errChan:
				log.G(ctx).Errorf("could not read from pts: %v", err)
			case line := <-ptsChan:
				_, err := logFD.Write(line)
				if err != nil {
					log.G(ctx).Errorf("could not write to log file: %v", err)
				}
			case <-ctx.Done():
				logFD.Close()
				ptsFD.Close()
				return
			}
		}
	}()

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

func NewXenConfig() (*xenlight.DomainConfig, error) {
	xcfg, err := xenlight.NewDomainConfig()
	if err != nil {
		return nil, err
	}

	binfoPtr, err := xenlight.NewDomainBuildInfo(xenlight.DomainTypePv)
	if err != nil {
		return nil, err
	}

	xcfg.BInfo = *binfoPtr

	cinfoPtr, err := xenlight.NewDomainCreateInfo()
	if err != nil {
		return nil, err
	}

	xcfg.CInfo = *cinfoPtr

	xcfg.CInfo.Type = xenlight.DomainTypePv

	return xcfg, nil
}
