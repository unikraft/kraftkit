// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

import (
	"context"
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/logs"
	"kraftkit.sh/internal/waitgroup"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network"
	mplatform "kraftkit.sh/machine/platform"
)

type StartOptions struct {
	All        bool   `long:"all" usage:"Start all machines"`
	Detach     bool   `long:"detach" short:"d" usage:"Run in background"`
	Platform   string `noattribute:"true"`
	Prefix     string `long:"prefix" usage:"Prefix each log line with the given string"`
	PrefixName bool   `long:"prefix-name" usage:"Prefix each log line with the machine name"`
	Remove     bool   `long:"rm" usage:"Automatically remove the unikernel when it shutsdown"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short:   "Start one or more machines",
		Use:     "start [FLAGS] MACHINE [MACHINE [...]]",
		Aliases: []string{},
		Long:    "Start one or more machines",
		Example: heredoc.Doc(`
			# Start a machine
			$ kraft start my-machine
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().VarP(
		cmdfactory.NewEnumFlag[mplatform.Platform](
			mplatform.Platforms(),
			mplatform.Platform("auto"),
		),
		"plat",
		"p",
		"Set the platform virtual machine monitor driver.  Set to 'auto' to detect the guest's platform and 'host' to use the host platform.",
	)

	return cmd
}

func (opts *StartOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && !opts.All {
		return fmt.Errorf("please supply a machine ID or name or use the --all flag")
	}

	opts.Platform = cmd.Flag("plat").Value.String()
	return nil
}

func (opts *StartOptions) Run(ctx context.Context, args []string) error {
	return Start(ctx, opts, args...)
}

// Start a set of machines by name.
func Start(ctx context.Context, opts *StartOptions, machineNames ...string) error {
	var err error

	platform := mplatform.PlatformUnknown
	var machineController machineapi.MachineService

	if opts.Platform == "auto" {
		machineController, err = mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	} else {
		if opts.Platform == "host" {
			platform, _, err = mplatform.Detect(ctx)
			if err != nil {
				return err
			}
		} else {
			var ok bool
			platform, ok = mplatform.PlatformsByName()[opts.Platform]
			if !ok {
				return fmt.Errorf("unknown platform driver: %s", opts.Platform)
			}
		}

		strategy, ok := mplatform.Strategies()[platform]
		if !ok {
			return fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", platform.String())
		}

		machineController, err = strategy.NewMachineV1alpha1(ctx)
	}
	if err != nil {
		return err
	}

	var machines []machineapi.Machine

	if opts.All {
		knownMachines, err := machineController.List(ctx, &machineapi.MachineList{})
		if err != nil {
			return err
		}

		machines = knownMachines.Items
	} else {
		for _, name := range machineNames {
			machine, err := machineController.Get(ctx, &machineapi.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			})
			if err != nil {
				return err
			}

			machines = append(machines, *machine)
		}
	}

	var errGroup []error
	observations := waitgroup.WaitGroup[*machineapi.Machine]{}

	for _, machine := range machines {
		machine := machine // Go closures

		log.G(ctx).
			WithField("machine", machine.Name).
			Trace("starting")

		if _, err := machineController.Start(ctx, &machine); err != nil {
			return err
		}

		if opts.Detach {
			// Output the name of the instance such that it can be piped
			fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", machine.Name)
			continue
		}

		observations.Add(&machine)

		// Start a thread which tails the logs of this machine
		go func(machine *machineapi.Machine) {
			defer func() {
				observations.Done(machine)
			}()

			if opts.PrefixName && opts.Prefix == "" || len(machines) > 1 {
				opts.Prefix = machine.Name
			}

			consumer, err := logs.NewColorfulConsumer(iostreams.G(ctx), !config.G[config.KraftKit](ctx).NoColor, opts.Prefix)
			if err != nil {
				errGroup = append(errGroup, err)
				return
			}

			// FollowLogs will block until either the machine exits or the context is
			// cancelled.
			exitErr := logs.FollowLogs(ctx, machine, machineController, consumer)

			log.G(ctx).
				WithField("machine", machine.Name).
				Trace("stopping")

			if _, err := machineController.Stop(ctx, machine); err != nil {
				log.G(ctx).Errorf("could not stop: %v", err)
			}

			// Remove the instance on Ctrl+C if the --rm flag is passed
			if opts.Remove {
				log.G(ctx).
					WithField("machine", machine.Name).
					Trace("removing")

				if _, err := machineController.Delete(ctx, machine); err != nil {
					log.G(ctx).Errorf("could not remove: %v", err)
				}
			}

			errGroup = append(errGroup, exitErr)
		}(&machine)
	}

	if opts.Detach {
		return nil
	}

	observations.Wait()

	var networkController networkapi.NetworkService

	for _, machine := range machines {
		// Set up a clean up method to remove the interface if the machine exits and
		// we are requesting to remove the machine.
		if opts.Remove && !opts.Detach && len(machine.Spec.Networks) > 0 {
			if networkController == nil {
				networkController, err = network.NewNetworkV1alpha1ServiceIterator(ctx)
				if err != nil {
					return fmt.Errorf("instantiating network service controller iterator: %w", err)
				}
			}

			for _, network := range machine.Spec.Networks {
				// Get the latest version of the network.
				found, err := networkController.Get(ctx, &networkapi.Network{
					ObjectMeta: metav1.ObjectMeta{
						Name: network.IfName,
					},
				})
				if err != nil {
					return fmt.Errorf("could not get network information for %s: %v", network.IfName, err)
				}

				// Remove the new network interface
				for i, iface := range found.Spec.Interfaces {
					if iface.UID == machine.Spec.Networks[0].Interfaces[0].UID {
						ret := make([]networkapi.NetworkInterfaceTemplateSpec, 0)
						ret = append(ret, found.Spec.Interfaces[:i]...)
						found.Spec.Interfaces = append(ret, found.Spec.Interfaces[i+1:]...)
						break
					}
				}

				if _, err = networkController.Update(ctx, found); err != nil {
					return fmt.Errorf("could not update network %s: %v", network.IfName, err)
				}
			}
		}
	}

	return errors.Join(errGroup...)
}
