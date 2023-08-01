// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/wrangler/pkg/signals"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
)

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

	machine.Spec.Kernel = "project://" + opts.project.Name() + ":" + targetName
	machine.Spec.Architecture = opts.target.Architecture().Name()
	machine.Spec.Platform = opts.target.Platform().Name()
	machine.Spec.ApplicationArgs = opts.Args

	machine.Status.KernelPath = opts.target.Kernel()

	if len(opts.InitRd) > 0 {
		machine.Status.InitrdPath = opts.InitRd
	}

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

	// If the user has supplied an initram path, set this now, this overrides any
	// preparation and is considered higher priority compared to what has been set
	// prior to this point.
	if opts.InitRd != "" {
		if machine.ObjectMeta.UID == "" {
			machine.ObjectMeta.UID = uuid.NewUUID()
		}

		if len(machine.Status.StateDir) == 0 {
			machine.Status.StateDir = filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, string(machine.ObjectMeta.UID))
		}

		if err := os.MkdirAll(machine.Status.StateDir, 0o755); err != nil {
			return fmt.Errorf("could not make machine state dir: %w", err)
		}

		var ramfs *initrd.InitrdConfig
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("could not get current working directory: %w", err)
		}

		if strings.Contains(opts.InitRd, initrd.InputDelimeter) {
			output := filepath.Join(machine.Status.StateDir, "initramfs.cpio")

			log.G(ctx).
				WithField("output", output).
				Debug("serializing initramfs cpio archive")

			ramfs, err = initrd.NewFromMapping(cwd, output, opts.InitRd)
			if err != nil {
				return fmt.Errorf("could not prepare initramfs: %w", err)
			}
		} else if f, err := os.Stat(opts.InitRd); err == nil && f.IsDir() {
			output := filepath.Join(machine.Status.StateDir, "initramfs.cpio")

			log.G(ctx).
				WithField("output", output).
				Debug("serializing initramfs cpio archive")

			ramfs, err = initrd.NewFromMapping(cwd, output, fmt.Sprintf("%s:/", opts.InitRd))
			if err != nil {
				return fmt.Errorf("could not prepare initramfs: %w", err)
			}
		} else {
			ramfs, err = initrd.NewFromFile(cwd, opts.InitRd)
			if err != nil {
				return fmt.Errorf("could not prepare initramfs: %w", err)
			}
		}

		machine.Status.InitrdPath = ramfs.Output
	}

	// Create the machine
	machine, err = controller.Create(ctx, machine)
	if err != nil {
		return err
	}

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
			// Wait on either channel
			select {
			case update := <-events:
				switch update.Status.State {
				case machineapi.MachineStateExited, machineapi.MachineStateFailed:
					signals.RequestShutdown()
					break loop
				}

			case err := <-errs:
				log.G(ctx).Errorf("received event error: %v", err)
				signals.RequestShutdown()
				break loop

			case <-ctx.Done():
				break loop
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

loop:
	for {
		// Wait on either channel
		select {
		case line := <-logs:
			fmt.Fprint(iostreams.G(ctx).Out, line)

		case err := <-errs:
			log.G(ctx).Errorf("received event error: %v", err)
			signals.RequestShutdown()
			break loop

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

	return nil
}
