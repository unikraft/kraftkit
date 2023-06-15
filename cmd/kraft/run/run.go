// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/packmanager"
)

type Run struct {
	Architecture  string   `long:"arch" short:"m" usage:"Set the architecture"`
	Detach        bool     `long:"detach" short:"d" usage:"Run unikernel in background"`
	DisableAccel  bool     `long:"disable-acceleration" short:"W" usage:"Disable acceleration of CPU (usually enables TCG)"`
	InitRd        string   `long:"initrd" usage:"Use the specified initrd"`
	IP            string   `long:"ip" usage:"Assign the provided IP address"`
	KernelArgs    []string `long:"kernel-arg" short:"a" usage:"Set additional kernel arguments"`
	MacAddress    string   `long:"mac" usage:"Assign the provided MAC address"`
	Memory        string   `long:"memory" short:"M" usage:"Assign MB memory to the unikernel" default:"64M"`
	Name          string   `long:"name" short:"n" usage:"Name of the instance"`
	Network       string   `long:"network" usage:"Attach instance to the provided network in the format <driver>:<network>, e.g. bridge:kraft0"`
	Ports         []string `long:"port" short:"p" usage:"Publish a machine's port(s) to the host" split:"false"`
	Remove        bool     `long:"rm" usage:"Automatically remove the unikernel when it shutsdown"`
	RunAs         string   `long:"as" usage:"Force a specific runner"`
	Target        string   `long:"target" short:"t" usage:"Explicitly use the defined project target"`
	WithKernelDbg bool     `long:"symbolic" usage:"Use the debuggable (symbolic) unikernel"`

	platform          mplatform.Platform
	networkDriver     string
	networkName       string
	networkController networkapi.NetworkService
	machineController machineapi.MachineService
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Run{}, cobra.Command{
		Short:   "Run a unikernel",
		Use:     "run [FLAGS] PROJECT|PACKAGE|BINARY -- [APP ARGS]",
		Aliases: []string{"r"},
		Long: heredoc.Doc(`
			Launch a unikernel`),
		Example: heredoc.Doc(`
			# (project) single target in cwd.
			kraft run

			# (project) single target at path.
			kraft run path/to/project

			# (project) select a single target from multiple in project cwd.
			kraft run -t TARGET

			# (project) multiple targets at path.
			kraft run -t TARGET path/to/project

			# (kernel) Run a unikernel kernel image.
			kraft run path/to/kernel-x86_64-kvm

			# (package) Run an OCI-compatible unikernel.
			kraft run unikraft.org/helloworld:latest`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag(set.NewStringSet(mplatform.DriverNames()...).Add("auto").ToSlice(), "auto"),
		"plat",
		"Set the platform virtual machine monitor driver.",
	)

	return cmd
}

// runner is an interface for defining different mechanisms to execute the
// provided unikernel.  Standardizing first the check, Runnable, to determine
// whether the provided input is capable of executing, and Prepare, actually
// performing the preparation of the Machine specification for the controller.
type runner interface {
	// String implements fmt.Stringer and returns the name of the implementing
	// runner.
	fmt.Stringer

	// Runnable checks whether the provided configuration is runnable.
	Runnable(context.Context, *Run, ...string) (bool, error)

	// Prepare the provided configuration into a machine specification ready for
	// execution by the controller.
	Prepare(context.Context, *Run, *machineapi.Machine, ...string) error
}

// runners is the list of built-in runners which are checked sequentially for
// capability.  The first to test positive via Runnable is used with the
// controller.
func runners() []runner {
	return []runner{
		&runnerKernel{},
		&runnerProject{},
		&runnerPackage{},
	}
}

func (opts *Run) Pre(cmd *cobra.Command, _ []string) error {
	var err error
	ctx := cmd.Context()

	// Discover the network controller strategy.
	if opts.Network == "" && opts.IP != "" {
		return fmt.Errorf("cannot assign IP address without providing --network")
	} else if opts.Network != "" && !strings.Contains(opts.Network, ":") {
		return fmt.Errorf("specifying a network must be in the format <driver>:<network> e.g. --network=bridge:kraft0")
	}

	if opts.Network != "" {
		// TODO(nderjung): With a little bit more work, the driver can be
		// automatically detected.
		parts := strings.SplitN(opts.Network, ":", 2)
		opts.networkDriver, opts.networkName = parts[0], parts[1]

		networkStrategy, ok := network.Strategies()[opts.networkDriver]
		if !ok {
			return fmt.Errorf("unsupported network driver strategy: %v (contributions welcome!)", opts.networkDriver)
		}

		opts.networkController, err = networkStrategy.NewNetworkV1alpha1(ctx)
		if err != nil {
			return err
		}
	}

	// Discover the platform machine controller strataegy.
	plat := cmd.Flag("plat").Value.String()
	opts.platform = mplatform.PlatformUnknown

	if plat == "" || plat == "auto" {
		var mode mplatform.SystemMode
		opts.platform, mode, err = mplatform.Detect(ctx)
		if mode == mplatform.SystemGuest {
			return fmt.Errorf("nested virtualization not supported")
		} else if err != nil {
			return err
		}
	} else {
		var ok bool
		opts.platform, ok = mplatform.Platforms()[plat]
		if !ok {
			return fmt.Errorf("unknown platform driver: %s", opts.platform)
		}
	}

	machineStrategy, ok := mplatform.Strategies()[opts.platform]
	if !ok {
		return fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", opts.platform.String())
	}

	opts.machineController, err = machineStrategy.NewMachineV1alpha1(ctx)
	if err != nil {
		return err
	}

	// Set use of the global package manager.
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	return nil
}

func (opts *Run) Run(cmd *cobra.Command, args []string) error {
	var err error
	ctx := cmd.Context()

	machine := &machineapi.Machine{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: machineapi.MachineSpec{
			Rootfs: opts.InitRd,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
			},
		},
	}

	if err := opts.parsePorts(ctx, machine); err != nil {
		return err
	}

	if err := opts.parseNetworks(ctx, machine); err != nil {
		return err
	}

	if err := opts.assignName(ctx, machine); err != nil {
		return err
	}

	// Iterate through the list of built-in runners which sequentially tests
	// whether the provided input matches the requirements for being run given its
	// context.  The first to test positive is used to prepare the machine
	// specification which is later passed to the controller.
	var run runner
	var errs []error
	for _, candidate := range runners() {
		if opts.RunAs != "" {
			if candidate.String() == opts.RunAs {
				if _, err := candidate.Runnable(ctx, opts, args...); err != nil {
					return fmt.Errorf("forced runner %s but provided argument was not capable: %v", opts.RunAs, err)
				}
				run = candidate
				break
			}

			continue
		}

		log.G(ctx).
			WithField("runner", candidate.String()).
			Trace("checking runnability")

		capable, err := candidate.Runnable(ctx, opts, args...)
		if capable && err == nil {
			run = candidate
			break
		} else if err != nil {
			errs = append(errs, err)
		}
	}
	if run == nil {
		return fmt.Errorf("could not determine what to run: %v", errs)
	}

	// Prepare the machine specification based on the compatible runner.
	if err := run.Prepare(ctx, opts, machine, args...); err != nil {
		return err
	}

	// Override with command-line flags
	if len(opts.KernelArgs) > 0 {
		machine.Spec.KernelArgs = opts.KernelArgs
	}

	if len(opts.Memory) > 0 {
		quantity, err := resource.ParseQuantity(opts.Memory)
		if err != nil {
			return err
		}

		machine.Spec.Resources.Requests[corev1.ResourceMemory] = quantity
	}

	// Create the machine
	machine, err = opts.machineController.Create(ctx, machine)
	if err != nil {
		return err
	}

	// Tail the logs if -d|--detach is not provided
	if !opts.Detach {
		go func() {
			events, errs, err := opts.machineController.Watch(ctx, machine)
			if err != nil {
				log.G(ctx).Errorf("could not listen for machine updates: %v", err)
				signals.RequestShutdown()
				return
			}

			log.G(ctx).Debug("waiting for machine events")

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
	}

	// Start the machine
	machine, err = opts.machineController.Start(ctx, machine)
	if err != nil {
		signals.RequestShutdown()
		return err
	}

	if !opts.Detach {
		logs, errs, err := opts.machineController.Logs(ctx, machine)
		if err != nil {
			signals.RequestShutdown()
			return fmt.Errorf("could not listen for machine logs: %v", err)
		}

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

		// Remove the instance on Ctrl+C if the --rm flag is passed
		if opts.Remove {
			if _, err := opts.machineController.Stop(ctx, machine); err != nil {
				return fmt.Errorf("could not stop: %v", err)
			}
			if _, err := opts.machineController.Delete(ctx, machine); err != nil {
				return fmt.Errorf("could not remove: %v", err)
			}
		}
	} else {
		// Output the name of the instance such that it can be piped
		fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", machine.Name)
	}

	return nil
}
