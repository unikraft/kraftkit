// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	zip "api.zip"
	"github.com/go-viper/mapstructure/v2"
	goprocess "github.com/shirou/gopsutil/v3/process"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/logtail"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/name"
)

const (
	// DefaultMemory is the default memory allocation for Hyperlight VMs when
	// --memory is not supplied. Small guests fit comfortably; heavier
	// interpreters need an explicit override upward.
	DefaultMemory = "16Mi"

	// DefaultStack is the default guest stack size. Not yet surfaced through
	// the kraft CLI or Kraftfile; adjust via the WithStack functional option.
	// TODO: wire through Kraftfile platform config.
	DefaultStack = "8Mi"

	// HostBinary is the external command that runs a Unikraft unikernel on
	// Hyperlight. It must be installed in $PATH for this driver to work.
	HostBinary = "hyperlight-unikraft"
)

// machineV1alpha1Service drives Hyperlight unikernels via the hyperlight-unikraft
// binary. Each machine corresponds to a child process that owns the in-process
// Hyperlight sandbox; PID tracking mirrors the firecracker driver so kraft ps,
// stop, rm, logs, etc. work across kraft invocations.
type machineV1alpha1Service struct{}

// NewMachineV1alpha1Service creates a new Hyperlight machine service.
func NewMachineV1alpha1Service(ctx context.Context, opts ...any) (machinev1alpha1.MachineService, error) {
	service := machineV1alpha1Service{}

	for _, opt := range opts {
		hopt, ok := opt.(MachineServiceV1alpha1Option)
		if !ok {
			continue
		}
		if err := hopt(&service); err != nil {
			return nil, err
		}
	}

	return &service, nil
}

// Create implements kraftkit.sh/api/machine/v1alpha1.MachineService.Create.
// It does not spawn the hyperlight-unikraft subprocess yet — that happens in
// Start, matching the firecracker/qemu lifecycle.
func (service *machineV1alpha1Service) Create(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.KernelPath == "" {
		return machine, fmt.Errorf("cannot create hyperlight instance without kernel")
	}

	if machine.Spec.Emulation {
		return machine, fmt.Errorf("hyperlight does not support emulation mode")
	}

	if _, err := exec.LookPath(HostBinary); err != nil {
		return machine, fmt.Errorf("%s not found in $PATH: %w", HostBinary, err)
	}

	if machine.ObjectMeta.UID == "" {
		machineID, err := name.NewRandomMachineID()
		if err != nil {
			return nil, fmt.Errorf("could not generate machine UID: %w", err)
		}
		machine.ObjectMeta.UID = types.UID(machineID.ShortString())
	}

	if machine.Status.StateDir == "" {
		machine.Status.StateDir = filepath.Join(
			config.G[config.KraftKit](ctx).RuntimeDir,
			string(machine.ObjectMeta.UID),
		)
	}

	if err := os.MkdirAll(machine.Status.StateDir, 0o755); err != nil {
		return machine, fmt.Errorf("could not create state directory: %w", err)
	}

	if machine.Spec.Resources.Requests.Memory().Value() == 0 {
		q, err := resource.ParseQuantity(DefaultMemory)
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}
		machine.Spec.Resources.Requests[corev1.ResourceMemory] = q
	}

	if machine.Spec.Resources.Requests.Cpu().Value() == 0 {
		q, err := resource.ParseQuantity("1")
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}
		machine.Spec.Resources.Requests[corev1.ResourceCPU] = q
	}

	logFile := filepath.Join(machine.Status.StateDir, "vmm.log")
	machine.Status.LogFile = logFile
	// Create the log file up front so `kraft logs --follow` can tail it the
	// moment Start returns, even before the child process has written output.
	if f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		f.Close()
	}

	machine.Status.PlatformConfig = HyperlightConfig{
		KernelPath: machine.Status.KernelPath,
		Memory:     machine.Spec.Resources.Requests.Memory().String(),
		Stack:      DefaultStack,
		InitRd:     machine.Status.InitrdPath,
		LogPath:    logFile,
	}

	machine.CreationTimestamp = metav1.Now()
	machine.Status.State = machinev1alpha1.MachineStateCreated

	log.G(ctx).
		WithField("uid", machine.ObjectMeta.UID).
		WithField("kernel", machine.Status.KernelPath).
		Debug("hyperlight instance created")

	return machine, nil
}

// Update implements kraftkit.sh/api/machine/v1alpha1.MachineService.
func (service *machineV1alpha1Service) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	return machine, nil
}

// Watch implements kraftkit.sh/api/machine/v1alpha1.MachineService.
func (service *machineV1alpha1Service) Watch(ctx context.Context, machine *machinev1alpha1.Machine) (chan *machinev1alpha1.Machine, chan error, error) {
	events := make(chan *machinev1alpha1.Machine)
	errs := make(chan error)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				updated, err := service.Get(ctx, machine)
				if err != nil {
					errs <- err
					return
				}

				if updated.Status.State != machine.Status.State {
					events <- updated
					machine = updated
				}

				if updated.Status.State == machinev1alpha1.MachineStateExited ||
					updated.Status.State == machinev1alpha1.MachineStateFailed {
					return
				}

				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	return events, errs, nil
}

// Start implements kraftkit.sh/api/machine/v1alpha1.MachineService. It spawns
// the hyperlight-unikraft subprocess, redirects its stdout/stderr to the log
// file, and records the child PID in machine.Status.
func (service *machineV1alpha1Service) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	hlcfg, err := getHyperlightConfigFromPlatformConfig(machine.Status.PlatformConfig)
	if err != nil {
		return machine, err
	}

	args := []string{
		"--memory", hlcfg.Memory,
		"--stack", hlcfg.Stack,
	}
	if hlcfg.InitRd != "" {
		args = append(args, "--initrd", hlcfg.InitRd)
	}
	args = append(args, hlcfg.KernelPath)
	if len(machine.Spec.ApplicationArgs) > 0 {
		args = append(args, "--")
		args = append(args, machine.Spec.ApplicationArgs...)
	}

	logFile, err := os.OpenFile(hlcfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not open log file: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(HostBinary, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not spawn %s: %w", HostBinary, err)
	}

	machine.Status.Pid = int32(cmd.Process.Pid)
	machine.Status.State = machinev1alpha1.MachineStateRunning
	machine.Status.StartedAt = time.Now()

	// Reap the child in the background so it doesn't sit as a zombie. If kraft
	// itself exits, the VMM process continues and eventually finishes on its
	// own — just like the firecracker driver.
	go func() { _ = cmd.Wait() }()

	log.G(ctx).
		WithField("uid", machine.ObjectMeta.UID).
		WithField("pid", cmd.Process.Pid).
		WithField("cmd", strings.Join(append([]string{HostBinary}, args...), " ")).
		Debug("hyperlight instance started")

	return machine, nil
}

// Pause implements kraftkit.sh/api/machine/v1alpha1.MachineService.
func (service *machineV1alpha1Service) Pause(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	return machine, fmt.Errorf("hyperlight does not support pause")
}

// Stop implements kraftkit.sh/api/machine/v1alpha1.MachineService.
func (service *machineV1alpha1Service) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.State == machinev1alpha1.MachineStateExited {
		return machine, nil
	}

	if machine.Status.Pid <= 0 {
		machine.Status.State = machinev1alpha1.MachineStateExited
		return machine, nil
	}

	proc, err := goprocess.NewProcess(machine.Status.Pid)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateExited
		return machine, nil
	}

	if err := proc.Terminate(); err != nil {
		return machine, fmt.Errorf("could not terminate hyperlight process %d: %w", machine.Status.Pid, err)
	}

	machine.Status.State = machinev1alpha1.MachineStateExited
	machine.Status.ExitedAt = time.Now()
	return machine, nil
}

// Delete implements kraftkit.sh/api/machine/v1alpha1.MachineService.
func (service *machineV1alpha1Service) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	machine, _ = service.Stop(ctx, machine)

	if machine.Status.StateDir != "" {
		os.RemoveAll(machine.Status.StateDir)
	}

	return machine, nil
}

// Get implements kraftkit.sh/api/machine/v1alpha1.MachineService.Get.
func (service *machineV1alpha1Service) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	hlcfg, err := getHyperlightConfigFromPlatformConfig(machine.Status.PlatformConfig)
	if err != nil {
		return machine, err
	}

	if hlcfg.Memory != "" {
		q, err := resource.ParseQuantity(hlcfg.Memory)
		if err != nil {
			return machine, fmt.Errorf("invalid memory quantity %q in platform config: %w", hlcfg.Memory, err)
		}
		machine.Spec.Resources.Requests[corev1.ResourceMemory] = q
	}
	cpu, err := resource.ParseQuantity("1")
	if err != nil {
		return machine, err
	}
	machine.Spec.Resources.Requests[corev1.ResourceCPU] = cpu

	machine.Status.PlatformConfig = *hlcfg

	// If the state is already terminal, don't second-guess it.
	switch machine.Status.State {
	case machinev1alpha1.MachineStateExited,
		machinev1alpha1.MachineStateFailed,
		machinev1alpha1.MachineStateErrored:
		return machine, nil
	}

	if machine.Status.Pid <= 0 {
		// Not yet started.
		return machine, nil
	}

	proc, err := goprocess.NewProcess(machine.Status.Pid)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateExited
		if machine.Status.ExitedAt.IsZero() {
			machine.Status.ExitedAt = time.Now()
		}
		return machine, nil
	}

	running, err := proc.IsRunning()
	if err != nil || !running {
		machine.Status.State = machinev1alpha1.MachineStateExited
		if machine.Status.ExitedAt.IsZero() {
			machine.Status.ExitedAt = time.Now()
		}
		return machine, nil
	}

	machine.Status.State = machinev1alpha1.MachineStateRunning
	return machine, nil
}

// List implements kraftkit.sh/api/machine/v1alpha1.MachineService.List.
func (service *machineV1alpha1Service) List(ctx context.Context, machines *machinev1alpha1.MachineList) (*machinev1alpha1.MachineList, error) {
	cached := machines.Items
	machines.Items = make([]zip.Object[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus], 0, len(cached))

	for _, machine := range cached {
		updated, err := service.Get(ctx, &machine)
		if err != nil {
			machines.Items = cached
			return machines, err
		}
		machines.Items = append(machines.Items, *updated)
	}

	return machines, nil
}

// Logs implements kraftkit.sh/api/machine/v1alpha1.MachineService.
func (service *machineV1alpha1Service) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	hlcfg, err := getHyperlightConfigFromPlatformConfig(machine.Status.PlatformConfig)
	if err != nil {
		return nil, nil, err
	}
	return logtail.NewLogTail(ctx, hlcfg.LogPath)
}

func getHyperlightConfigFromPlatformConfig(platformConfig interface{}) (*HyperlightConfig, error) {
	if p, ok := platformConfig.(*HyperlightConfig); ok {
		return p, nil
	}
	if p, ok := platformConfig.(HyperlightConfig); ok {
		return &p, nil
	}

	var hlcfg HyperlightConfig
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: &hlcfg})
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(platformConfig); err != nil {
		return nil, err
	}
	return &hlcfg, nil
}
