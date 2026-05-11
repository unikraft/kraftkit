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
	pathpkg "path"
	"path/filepath"
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
	kraftexec "kraftkit.sh/exec"
	"kraftkit.sh/internal/logtail"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/name"
)

const (
	// DefaultMemory is the default memory allocation for Hyperlight VMs when
	// --memory is not supplied. This matches hyperlight-unikraft's host-side
	// default.
	DefaultMemory = "32Mi"

	// DefaultStack is the default guest stack size. Not yet surfaced through
	// the kraft CLI or Kraftfile; adjust via the WithStack functional option.
	// TODO: wire through Kraftfile platform config.
	DefaultStack = "8Mi"

	// HostBinary is the external command that runs a Unikraft unikernel on
	// Hyperlight. It must be installed in $PATH for this driver to work.
	HostBinary = "hyperlight-unikraft"
)

var reservedGuestMountPaths = map[string]struct{}{
	"/":     {},
	"/bin":  {},
	"/dev":  {},
	"/proc": {},
	"/sys":  {},
	"/usr":  {},
}

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

	if err := validateHyperlightPaths(machine.Status.KernelPath, machine.Status.InitrdPath); err != nil {
		return machine, err
	}

	if machine.Spec.Emulation {
		return machine, fmt.Errorf("hyperlight-unikraft does not support emulation mode (--disable-acceleration)")
	}

	if len(machine.Spec.Ports) > 0 {
		return machine, fmt.Errorf("hyperlight-unikraft does not support port publishing (--port)")
	}

	if len(machine.Spec.Networks) > 0 {
		return machine, fmt.Errorf("hyperlight-unikraft does not support network attachments (--network, --ip, --mac)")
	}

	if len(machine.Spec.Env) > 0 {
		return machine, fmt.Errorf("hyperlight-unikraft does not support environment injection (--env)")
	}

	if len(machine.Spec.KernelArgs) > 0 {
		return machine, fmt.Errorf("hyperlight-unikraft does not support kernel arguments (--kernel-arg); pass application arguments after --")
	}

	if _, err := exec.LookPath(HostBinary); err != nil {
		return machine, fmt.Errorf("%s not found in $PATH: %w", HostBinary, err)
	}

	mounts, err := hyperlightMountsFromMachine(machine)
	if err != nil {
		return machine, err
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

	if machine.Spec.Resources.Requests == nil {
		machine.Spec.Resources.Requests = corev1.ResourceList{}
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
		Mounts:     mounts,
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
// file, and records the child PID in machine.Status. The process is detached
// (own process group) so the VMM continues running after kraft exits, matching
// the firecracker driver's lifecycle.
func (service *machineV1alpha1Service) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	hlcfg, err := getHyperlightConfigFromPlatformConfig(machine.Status.PlatformConfig)
	if err != nil {
		return machine, err
	}

	if err := validateHyperlightPaths(hlcfg.KernelPath, hlcfg.InitRd); err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, err
	}

	args := []string{
		"--memory", hlcfg.Memory,
		"--stack", hlcfg.Stack,
	}
	if hlcfg.InitRd != "" {
		args = append(args, "--initrd", hlcfg.InitRd)
	}
	for _, mount := range hlcfg.Mounts {
		args = append(args, "--mount", mount)
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
	// We duplicate this fd into the child via stdout; safe to close our
	// own copy once the child is running.
	defer logFile.Close()

	// Use kraftkit's exec wrapper with WithDetach so the child gets its
	// own process group and survives kraft exiting — same pattern the
	// firecracker driver uses.
	proc, err := kraftexec.NewProcess(HostBinary, args,
		kraftexec.WithStdout(logFile),
		kraftexec.WithDetach(true),
	)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not prepare %s process: %w", HostBinary, err)
	}

	if err := proc.Start(ctx); err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not spawn %s: %w", HostBinary, err)
	}

	pid, err := proc.Pid()
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not read pid of spawned %s: %w", HostBinary, err)
	}

	// Release the Go-side handle so the child doesn't need a Wait()
	// and isn't kept alive through kraft's address space. Matches the
	// "fire and forget PID-tracked lifecycle" the firecracker driver
	// relies on.
	if err := proc.Release(); err != nil {
		log.G(ctx).WithError(err).Debug("releasing hyperlight process handle")
	}

	machine.Status.Pid = int32(pid)
	machine.Status.State = machinev1alpha1.MachineStateRunning
	machine.Status.StartedAt = time.Now()

	log.G(ctx).
		WithField("uid", machine.ObjectMeta.UID).
		WithField("pid", pid).
		WithField("cmd", proc.Cmdline()).
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

	if machine.Spec.Resources.Requests == nil {
		machine.Spec.Resources.Requests = corev1.ResourceList{}
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

func validateHyperlightPaths(kernelPath, initrdPath string) error {
	if kernelPath == "" {
		return fmt.Errorf("cannot create hyperlight instance without kernel")
	}

	if err := validateExistingFile(kernelPath, "hyperlight kernel"); err != nil {
		return err
	}

	if initrdPath != "" {
		if err := validateExistingFile(initrdPath, "hyperlight initrd"); err != nil {
			return err
		}
	}

	return nil
}

func validateExistingFile(path, description string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s path %q is not accessible: %w", description, path, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s path %q is a directory", description, path)
	}

	return nil
}

func hyperlightMountsFromMachine(machine *machinev1alpha1.Machine) ([]string, error) {
	if len(machine.Spec.Volumes) == 0 {
		return nil, nil
	}

	mounts := make([]string, 0, len(machine.Spec.Volumes))
	guestPaths := map[string]struct{}{}

	for _, volume := range machine.Spec.Volumes {
		switch volume.Spec.Driver {
		case "initrd":
			if volume.Spec.Destination == "/" && machine.Status.InitrdPath != "" {
				continue
			}
			if volume.Spec.Destination == "/" {
				return nil, fmt.Errorf("hyperlight initrd volume requires an initrd path")
			}
			return nil, fmt.Errorf("hyperlight-unikraft does not support KraftKit initrd volumes mounted at %q", volume.Spec.Destination)

		case "9pfs":
		default:
			return nil, fmt.Errorf("hyperlight-unikraft does not support KraftKit volume driver %q", volume.Spec.Driver)
		}

		if volume.Spec.ReadOnly {
			return nil, fmt.Errorf("hyperlight-unikraft does not support read-only KraftKit volumes")
		}

		if volume.Spec.Source == "" {
			return nil, fmt.Errorf("hyperlight volume source cannot be empty")
		}

		hostPath, err := filepath.Abs(volume.Spec.Source)
		if err != nil {
			return nil, fmt.Errorf("could not resolve hyperlight volume source %q: %w", volume.Spec.Source, err)
		}

		info, err := os.Stat(hostPath)
		if err != nil {
			return nil, fmt.Errorf("hyperlight volume source %q is not accessible: %w", hostPath, err)
		}

		if !info.IsDir() {
			return nil, fmt.Errorf("hyperlight volume source %q is not a directory", hostPath)
		}

		guestPath, err := normalizeHyperlightGuestMountPath(volume.Spec.Destination)
		if err != nil {
			return nil, err
		}

		if _, exists := guestPaths[guestPath]; exists {
			return nil, fmt.Errorf("hyperlight volume guest mount %q is duplicated", guestPath)
		}
		guestPaths[guestPath] = struct{}{}

		mounts = append(mounts, fmt.Sprintf("%s:%s", hostPath, guestPath))
	}

	return mounts, nil
}

func normalizeHyperlightGuestMountPath(guestPath string) (string, error) {
	if guestPath == "" {
		return "", fmt.Errorf("hyperlight volume destination cannot be empty")
	}

	if !pathpkg.IsAbs(guestPath) {
		return "", fmt.Errorf("hyperlight volume destination %q must be absolute", guestPath)
	}

	return guestPath, nil
}
