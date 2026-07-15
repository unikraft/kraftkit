// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/cpuid/v2"
	"github.com/mattn/go-shellwords"
	"github.com/rancher/wrangler/pkg/signals"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/machine/volume"
	"kraftkit.sh/unikraft/export/v0/ukrandom"
)

// parseKraftfileEnv sets the environment variables declared in the Kraftfile,
// as well as those supplied via the `env` input, on the machine.  Variables
// declared without a value are looked up in the host environment.
func (opts *GithubAction) parseKraftfileEnv(machine *machineapi.Machine) {
	if machine.Spec.Env == nil {
		machine.Spec.Env = make(map[string]string)
	}

	for k, v := range opts.project.Env() {
		if v != "" {
			machine.Spec.Env[k] = v
			continue
		}

		if v, ok := os.LookupEnv(k); ok {
			machine.Spec.Env[k] = v
		}
	}

	for _, env := range opts.envs {
		k, v, ok := strings.Cut(env, "=")
		if !ok {
			if v, found := os.LookupEnv(k); found {
				machine.Spec.Env[k] = v
			}

			continue
		}

		machine.Spec.Env[k] = v
	}
}

// parseKraftfileVolumes attaches the volumes declared in the Kraftfile to the
// machine.
func (opts *GithubAction) parseKraftfileVolumes(ctx context.Context, machine *machineapi.Machine) error {
	if opts.project.Volumes() == nil {
		return nil
	}

	var err error
	controllers := map[string]volumeapi.VolumeService{}
	if machine.Spec.Volumes == nil {
		machine.Spec.Volumes = make([]volumeapi.Volume, 0)
	}

	for _, volcfg := range opts.project.Volumes() {
		driver := volcfg.Driver()

		if len(driver) == 0 {
			for sname, strategy := range volume.Strategies() {
				if ok, _ := strategy.IsCompatible(volcfg.Source(), nil); !ok {
					continue
				}

				if _, ok := controllers[sname]; !ok {
					log.G(ctx).WithField("volume strategy", sname).Debug("found volume strategy")
					controllers[sname], err = strategy.NewVolumeV1alpha1(ctx)
					if err != nil {
						return fmt.Errorf("could not prepare %s volume service: %w", sname, err)
					}
				}

				driver = sname
			}
		} else {
			strategy, exists := volume.Strategies()[driver]
			if !exists {
				return fmt.Errorf("unknown volume driver %s specified", driver)
			}

			if ok, _ := strategy.IsCompatible(volcfg.Source(), nil); !ok {
				return fmt.Errorf("volume driver %s is incompatible with source %s", driver, volcfg.Source())
			}

			if _, ok := controllers[driver]; !ok {
				log.G(ctx).WithField("volume strategy", driver).Debug("found volume strategy")
				controllers[driver], err = strategy.NewVolumeV1alpha1(ctx)
				if err != nil {
					return fmt.Errorf("could not prepare %s volume service: %w", driver, err)
				}
			}
		}

		if len(driver) == 0 {
			return fmt.Errorf("could not find compatible volume driver for %s", volcfg.Source())
		}

		// Check if this could be a named volume.
		vol, err := controllers[driver].Get(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: volcfg.Source(),
			},
		})
		if err == nil && vol != nil && vol.Spec.Source != "" {
			vol.Spec.Destination = volcfg.Destination()
			machine.Spec.Volumes = append(machine.Spec.Volumes, *vol)
			continue
		}

		vol, err = controllers[driver].Create(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%d", machine.ObjectMeta.Name, len(machine.Spec.Volumes)),
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      driver,
				Source:      volcfg.Source(),
				Destination: volcfg.Destination(),
				ReadOnly:    volcfg.ReadOnly(),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create volume: %w", err)
		}

		machine.Spec.Volumes = append(machine.Spec.Volumes, *vol)
	}

	return nil
}

func (opts *GithubAction) execute(ctx context.Context) error {
	var err error

	if opts.Timeout == 0 {
		opts.Timeout = 10
	}

	plat, ok := mplatform.PlatformsByName()[opts.Plat]
	if !ok {
		return fmt.Errorf("unknown platform: %s", opts.Plat)
	}

	machineStrategy, ok := mplatform.Strategies()[plat]
	if !ok {
		return fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", opts.Plat)
	}

	controller, err := machineStrategy.NewMachineV1alpha1(ctx)
	if err != nil {
		return err
	}

	machine := &machineapi.Machine{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: machineapi.MachineSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
			},
			Emulation: true,
		},
	}

	// Provide a meaningful name
	targetName := opts.target.Name()
	if targetName == opts.project.Name() || targetName == "" {
		targetName = opts.target.Platform().Name() + "/" + opts.target.Architecture().Name()
	}

	args, err := shellwords.Parse(opts.Args)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		args = opts.project.Command()
	}

	machine.Spec.Kernel = "project://" + opts.project.Name() + ":" + targetName
	machine.Spec.Architecture = opts.target.Architecture().Name()
	machine.Spec.Platform = opts.target.Platform().Name()

	// Arguments preceding a `--` separator are passed to the kernel itself, the
	// remainder are passed to the application.
	var kernelArgs []string
	var appArgs []string

	for _, arg := range args {
		if arg == "--" {
			kernelArgs = appArgs
			appArgs = []string{}
			continue
		}

		appArgs = append(appArgs, arg)
	}

	// Unikraft v0.17.0 and greater seed their randomness from the CPU, which is
	// not always available on the host running the action.  Supply a seed via the
	// command line when the unikernel supports it.
	hasUkRandom := !opts.target.KConfig().AllNoOrUnset(
		"CONFIG_LIBUKRANDOM",
	)
	hasCmdlineSupport := !opts.target.KConfig().AllNoOrUnset(
		"CONFIG_LIBUKRANDOM_CMDLINE_SEED",
	)
	hasNoCpuRandomnessSupport := opts.target.KConfig().AllNoOrUnset("CONFIG_LIBUKRANDOM_LCPU") ||
		!(cpuid.CPU.Has(cpuid.RDRAND) || cpuid.CPU.Has(cpuid.RNDR))

	if hasUkRandom && hasNoCpuRandomnessSupport {
		if hasCmdlineSupport {
			kernelArgs = append(kernelArgs, ukrandom.ParamRandomSeed.WithValue(ukrandom.NewRandomSeed()).String())
		} else {
			log.G(ctx).Warn("RDRAND is not supported by the host CPU to be able to run Unikraft v0.17.0 and greater with CPU-generated randomness")
		}
	}

	machine.Spec.KernelArgs = kernelArgs
	machine.Spec.ApplicationArgs = appArgs

	kernelPath := opts.target.Kernel()
	if opts.Dbg {
		kernelPath = opts.target.KernelDbg()
	}
	if !filepath.IsAbs(kernelPath) {
		kernelPath = filepath.Join(opts.Workdir, kernelPath)
	}
	machine.Status.KernelPath = kernelPath

	if _, err := os.Stat(machine.Status.KernelPath); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("cannot run the selected project target '%s' without building the kernel: try running `kraft build` first: %w", targetName, err)
	}

	if len(opts.Memory) > 0 {
		quantity, err := resource.ParseQuantity(opts.Memory)
		if err != nil {
			return err
		}

		machine.Spec.Resources.Requests[corev1.ResourceMemory] = quantity
	}

	machine.ObjectMeta.Name = "github-action"

	// Only attach the initrd when the rootfs is not already embedded in the
	// kernel.
	noEmbedded := opts.target.KConfig().AllNoOrUnset(
		"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD",
		"CONFIG_LIBVFSCORE_AUTOMOUNT_CI_EINITRD",
		"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD_PATH",
		"CONFIG_LIBPOSIX_VFS_FSTAB_EINITRD",
		"CONFIG_LIBPOSIX_VFS_FSTAB_EINITRD_PATH",
		"CONFIG_LIBPOSIX_VFS_FSTAB_BUILTIN_EINITRD",
		"CONFIG_LIBPOSIX_VFS_FSTAB_FALLBACK_EINITRD",
	)

	if opts.Rootfs != "" && noEmbedded {
		machine.Status.InitrdPath = opts.initrdPath
	}

	// If automounting is enabled and an initramfs is provided, set it as a
	// volume.
	if opts.target.KConfig().AnyYes(
		"CONFIG_LIBVFSCORE_FSTAB", // Deprecated
		"CONFIG_LIBVFSCORE_AUTOMOUNT_UP",
	) && noEmbedded && (len(machine.Status.InitrdPath) > 0 || len(opts.Rootfs) > 0) {
		machine.Spec.Volumes = append(machine.Spec.Volumes, volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "rootfs",
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      "initrd",
				Destination: "/",
			},
		})
	}

	if err := opts.parseKraftfileVolumes(ctx, machine); err != nil {
		return err
	}

	opts.parseKraftfileEnv(machine)

	// Create the machine
	machine, err = controller.Create(ctx, machine)
	if err != nil {
		return err
	}

	var exitErr error
	requestShutdown := false
	logsFinished := make(chan bool, 1)

	go func() {
		events, errs, err := controller.Watch(ctx, machine)
		if err != nil {
			log.G(ctx).Errorf("could not listen for machine updates: %v", err)
			signals.RequestShutdown()
			return
		}

		log.G(ctx).Trace("waiting for machine events")

	loop:
		for {
			if requestShutdown {
				<-logsFinished
				signals.RequestShutdown()
				break loop
			}

			// Wait on either channel
			select {
			case update := <-events:
				switch update.Status.State {
				case machineapi.MachineStateErrored:
					signals.RequestShutdown()
					exitErr = fmt.Errorf("machine fatally exited")
					requestShutdown = true

				case machineapi.MachineStateExited, machineapi.MachineStateFailed:
					requestShutdown = true
				}

			case err := <-errs:
				log.G(ctx).Errorf("received event error: %v", err)
				signals.RequestShutdown()
				break loop

			case <-ctx.Done():
				requestShutdown = true
			}
		}
	}()

	// Start the machine
	machine, err = controller.Start(ctx, machine)
	if err != nil {
		signals.RequestShutdown()
		return err
	}

	logs, errs, err := controller.Logs(ctx, machine)
	if err != nil {
		signals.RequestShutdown()
		return fmt.Errorf("could not listen for machine logs: %v", err)
	}

	// Set a timer for 10 seconds for the machine to start and then stop it
	// if it hasn't started.
	// Useful for when the machine does not quit on its own.
	timer := time.AfterFunc(time.Duration(opts.Timeout)*time.Second, func() {
		if machine.Status.State == machineapi.MachineStateExited ||
			machine.Status.State == machineapi.MachineStateFailed {
			return
		}

		if _, err := controller.Stop(ctx, machine); err != nil {
			log.G(ctx).Errorf("could not stop: %v", err)
		}

		if _, err := controller.Delete(ctx, machine); err != nil {
			log.G(ctx).Errorf("could not remove: %v", err)
		}
	})
	defer timer.Stop()

	var line string
loop:
	for {
		// Wait on either channel
		select {
		case <-time.After(10 * time.Millisecond):
			if requestShutdown && line == "" {
				break loop
			} else if line != "" {
				line = ""
			}

		case line = <-logs:
			fmt.Fprint(iostreams.G(ctx).Out, line)

		case err := <-errs:
			if errors.Is(err, io.EOF) && requestShutdown {
				break loop
			} else if !errors.Is(err, io.EOF) {
				log.G(ctx).Errorf("received log error: %v", err)
				signals.RequestShutdown()
				break loop
			}

		case <-ctx.Done():
			break loop
		}
	}

	if machine.Status.State == machineapi.MachineStateExited {
		return nil
	}

	if machine.Status.State == machineapi.MachineStateFailed {
		return fmt.Errorf("machine failed when running")
	}

	if _, err := controller.Stop(ctx, machine); err != nil {
		log.G(ctx).Errorf("could not stop: %v", err)
	}

	if _, err := controller.Delete(ctx, machine); err != nil {
		log.G(ctx).Errorf("could not remove: %v", err)
	}

	return exitErr
}
