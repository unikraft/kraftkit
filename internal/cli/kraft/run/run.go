// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/start"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/arch"
)

type RunOptions struct {
	Architecture  string   `long:"arch" short:"m" usage:"Set the architecture"`
	Detach        bool     `long:"detach" short:"d" usage:"Run unikernel in background"`
	DisableAccel  bool     `long:"disable-acceleration" short:"W" usage:"Disable acceleration of CPU (usually enables TCG)"`
	InitRd        string   `long:"initrd" usage:"Use the specified initrd (readonly)" hidden:"true"`
	IP            string   `long:"ip" usage:"Assign the provided IP address"`
	KernelArgs    []string `long:"kernel-arg" short:"a" usage:"Set additional kernel arguments"`
	Kraftfile     string   `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	MacAddress    string   `long:"mac" usage:"Assign the provided MAC address"`
	Memory        string   `long:"memory" short:"M" usage:"Assign memory to the unikernel (K/Ki, M/Mi, G/Gi)" default:"64Mi"`
	Name          string   `long:"name" short:"n" usage:"Name of the instance"`
	Network       string   `long:"network" usage:"Attach instance to the provided network"`
	NoStart       bool     `long:"no-start" usage:"Do not start the machine"`
	Platform      string   `noattribute:"true"`
	Ports         []string `long:"port" short:"p" usage:"Publish a machine's port(s) to the host" split:"false"`
	Prefix        string   `long:"prefix" usage:"Prefix each log line with the given string"`
	PrefixName    bool     `long:"prefix-name" usage:"Prefix each log line with the machine name"`
	Remove        bool     `long:"rm" usage:"Automatically remove the unikernel when it shutsdown"`
	Rootfs        string   `long:"rootfs" usage:"Specify a path to use as root file system (can be volume or initramfs)"`
	RunAs         string   `long:"as" usage:"Force a specific runner"`
	Target        string   `long:"target" short:"t" usage:"Explicitly use the defined project target"`
	Volumes       []string `long:"volume" short:"v" usage:"Bind a volume to the instance"`
	WithKernelDbg bool     `long:"symbolic" usage:"Use the debuggable (symbolic) unikernel"`

	workdir           string
	platform          mplatform.Platform
	machineController machineapi.MachineService
}

// Run a Unikraft unikernel virtual machine locally.
func Run(ctx context.Context, opts *RunOptions, args ...string) error {
	if opts == nil {
		opts = &RunOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RunOptions{}, cobra.Command{
		Short:   "Run a unikernel",
		Use:     "run [FLAGS] PROJECT|PACKAGE|BINARY -- [APP ARGS]",
		Aliases: []string{"r"},
		Long: heredoc.Doc(`
			Run a unikernel virtual machine
		`),
		Example: heredoc.Doc(`
			Run a built target in the current working directory project:
			$ kraft run

			Run a specific target from a multi-target project at the provided project directory:
			$ kraft run -t TARGET path/to/project

			Run a specific kernel binary:
			$ kraft run --arch x86_64 --plat qemu path/to/kernel-x86_64-qemu

			Run a specific kernel binary with 1000 megabytes of memory:
			$ kraft run --arch x86_64 --plat qemu --memory 1G path/to/kernel-x86_64-qemu

			Run a specific kernel binary with 1024 megabytes of memory:
			$ kraft run --arch x86_64 --plat qemu --memory 1Gi path/to/kernel-x86_64-qemu

			Run an OCI-compatible unikernel, mapping port 8080 on the host to port 80 in the unikernel:
			$ kraft run -p 8080:80 unikraft.org/nginx:latest

			Attach the unikernel to an existing network kraft0:
			$ kraft run --network kraft0

			Run a Linux userspace binary in POSIX-/binary-compatibility mode:
			$ kraft run a.out

			Supply an initramfs CPIO archive file to the unikernel for its rootfs:
			$ kraft run --rootfs ./initramfs.cpio

			Supply a path which is dynamically serialized into an initramfs CPIO archive:
			$ kraft run --rootfs ./path/to/rootfs

			Mount a bi-directional path from on the host to the unikernel mapped to /dir:
			$ kraft run -v ./path/to/dir:/dir

			Supply a read-only root file system at / via initramfs CPIO archive and mount a bi-directional volume at /dir:
			$ kraft run --rootfs ./initramfs.cpio --volume ./path/to/dir:/dir

			Customize the default content directory of the official Unikraft NGINX OCI-compatible unikernel and map port 8080 to localhost:
			$ kraft run -v ./path/to/html:/nginx/html -p 8080:80 unikraft.org/nginx:latest
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[mplatform.Platform](
			mplatform.Platforms(),
			mplatform.Platform("auto"),
		),
		"plat",
		"Set the platform virtual machine monitor driver.",
	)

	return cmd
}

func (opts *RunOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	opts.Platform = cmd.Flag("plat").Value.String()

	if opts.RunAs == "" || !set.NewStringSet("kernel", "project").Contains(opts.RunAs) {
		// Set use of the global package manager.
		ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
		if err != nil {
			return err
		}

		cmd.SetContext(ctx)
	}

	if opts.RunAs != "" {
		runners, err := runnersByName()
		if err != nil {
			return err
		}
		if _, ok := runners[opts.RunAs]; !ok {
			choices := make([]string, len(runners))
			i := 0

			for choice := range runners {
				choices[i] = choice
				i++
			}

			return fmt.Errorf("unknown runner: %s (choice of %v)", opts.RunAs, choices)
		}
	}

	if opts.InitRd != "" {
		log.G(ctx).Warn("the --initrd flag is deprecated in favour of --rootfs")

		if opts.Rootfs != "" {
			log.G(ctx).Warn("both --initrd and --rootfs are set! ignorning value of --initrd")
		} else {
			log.G(ctx).Warn("for backwards-compatibility reasons the value of --initrd is set to --rootfs")
			opts.Rootfs = opts.InitRd
		}
	}

	return nil
}

func (opts *RunOptions) discoverMachineController(ctx context.Context) error {
	var err error

	opts.platform = mplatform.PlatformUnknown

	var mode mplatform.SystemMode
	defaultPlatform, mode, err := mplatform.Detect(ctx)
	if err != nil {
		return err
	} else if mode == mplatform.SystemGuest {
		log.G(ctx).Warn("using hardware emulation")
		opts.DisableAccel = true
	}

	if opts.Platform == "" || opts.Platform == "auto" || opts.Platform == defaultPlatform.String() {
		opts.platform = defaultPlatform
	} else {
		var ok bool
		opts.platform, ok = mplatform.PlatformsByName()[opts.Platform]
		if !ok {
			return fmt.Errorf("unknown platform driver '%s', however your system supports '%s'", opts.Platform, defaultPlatform.String())
		}
	}

	machineStrategy, ok := mplatform.Strategies()[opts.platform]
	if !ok {
		return fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", opts.Platform)
	}

	log.G(ctx).WithField("platform", opts.platform.String()).Debug("using")

	opts.machineController, err = machineStrategy.NewMachineV1alpha1(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (opts *RunOptions) Run(ctx context.Context, args []string) error {
	var err error

	if err = opts.discoverMachineController(ctx); err != nil {
		return err
	}

	if len(opts.Architecture) > 0 {
		if _, found := arch.ArchitecturesByName()[opts.Architecture]; !found {
			log.G(ctx).WithFields(logrus.Fields{
				"arch": opts.Architecture,
			}).Warn("unknown or incompatible")
		}
	}

	machine := &machineapi.Machine{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: machineapi.MachineSpec{
			Platform: opts.platform.String(),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
			},
			Emulation: opts.DisableAccel,
		},
	}

	var run runner
	var errs []error
	runners, err := runners()
	if err != nil {
		return err
	}

	// Iterate through the list of built-in runners which sequentially tests and
	// first test whether the --as flag has been set to force a specific runner or
	// whether the current context matches the requirements for being run given
	// its context.  The first to test positive is used to prepare the machine
	// specification which is later passed to the controller.
	for _, candidate := range runners {
		if opts.RunAs != "" && candidate.String() != opts.RunAs {
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
			log.G(ctx).
				WithField("runner", candidate.String()).
				Debugf("cannot run because: %v", err)
		}
	}
	if run == nil {
		return fmt.Errorf("could not determine how to run provided input: %w", errors.Join(errs...))
	}

	log.G(ctx).WithField("runner", run.String()).Debug("using")

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

	if err := opts.parsePorts(ctx, machine); err != nil {
		return err
	}

	if err := opts.parseNetworks(ctx, machine); err != nil {
		return err
	}

	if err := opts.parseVolumes(ctx, machine); err != nil {
		return err
	}

	if err := opts.assignName(ctx, machine); err != nil {
		return err
	}

	if err := opts.prepareRootfs(ctx, machine); err != nil {
		return err
	}

	// Create the machine
	machine, err = opts.machineController.Create(ctx, machine)
	if err != nil {
		return err
	}

	if opts.NoStart {
		// Output the name of the instance such that it can be piped
		fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", machine.Name)
		return nil
	}

	return start.Start(ctx, &start.StartOptions{
		Detach:     opts.Detach,
		Platform:   opts.platform.String(),
		Prefix:     opts.Prefix,
		PrefixName: opts.PrefixName,
		Remove:     opts.Remove,
	}, machine.Name)
}
