// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package create

import (
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/MakeNowJust/heredoc"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rtspec "github.com/opencontainers/runtime-spec/specs-go"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/internal/set"
	libcontainer "kraftkit.sh/libmocktainer"
	"kraftkit.sh/libmocktainer/specconv"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/machine/qemu"
	"kraftkit.sh/oci"
)

// CreateOptions implements the OCI "create" command.
type CreateOptions struct {
	Bundle        string `long:"bundle" short:"b" usage:"path to the root of the bundle directory"`
	ConsoleSocket string `long:"console-socket" usage:"path to an AF_UNIX socket which will receive a file descriptor referencing the master end of the console's pseudoterminal"`
	PidFile       string `long:"pid-file" usage:"specify a file where the process ID will be written"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short: "Create a new unikernel",
		Args:  cobra.ExactArgs(1),
		Use:   "create [FLAGS] <unikernel-id>",
		Long: heredoc.Doc(`Create a new unikernel with the given ID.  IDs must be unique.

			The create command creates an instance of a unikernel for a bundle. The bundle
			is a directory with a specification file named "config.json" and a root
			filesystem.

			The specification file includes an args parameter. The args parameter is used
			to specify command(s) that get run when the unikernel is started. To change the
			command(s) that get executed on start, edit the args parameter of the spec`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, args []string) error {
	if opts.Bundle == "" {
		return fmt.Errorf("--bundle is required")
	}

	bundleConfig := filepath.Join(opts.Bundle, oci.ConfigFilename)
	fInfo, err := os.Stat(bundleConfig)
	if err != nil {
		return err
	}

	if !fInfo.Mode().IsRegular() {
		return fmt.Errorf("%q should be a regular file", bundleConfig)
	}

	return nil
}

const (
	flagRoot     = "root"
	flagSdCgroup = "systemd-cgroup"
)

// TODO(antoineco): the runtime spec (config.json) doesn't provide any hint
// about the desired target platform, so we will need to think about ways to
// figure this out.
const plat = mplatform.PlatformQEMU

const (
	specAnnotCRIContainerType = "io.kubernetes.cri.container-type"
	specAnnotUnikernel        = "org.unikraft.kernel"
)

func (opts *CreateOptions) Run(cmd *cobra.Command, args []string) (retErr error) {
	ctx := cmd.Context()

	defer func() {
		// Make sure the error is written to the configured log destination, so
		// that the message gets propagated through the caller (e.g. containerd-shim)
		if retErr != nil {
			log.G(ctx).Error(retErr)
		}
	}()

	rootDir := cmd.Flag(flagRoot).Value.String()
	if rootDir == "" {
		return fmt.Errorf("state directory (--%s flag) is not set", flagRoot)
	}

	sdcg, err := strconv.ParseBool(cmd.Flag(flagSdCgroup).Value.String())
	if err != nil {
		return fmt.Errorf("parsing --%s flag: %w", flagSdCgroup, err)
	}
	if sdcg {
		log.G(ctx).Warnf("ignoring --%s flag", flagSdCgroup)
	}

	var pidFile string
	if opts.PidFile != "" {
		if pidFile, err = filepath.Abs(opts.PidFile); err != nil {
			return fmt.Errorf("getting pid file abs path: %w", err)
		}
	}

	if err = os.Chdir(opts.Bundle); err != nil {
		return fmt.Errorf("changing working dir to OCI bundle: %w", err)
	}

	spec, err := loadSpec()
	if err != nil {
		return fmt.Errorf("loading runtime spec: %w", err)
	}
	spec.Linux.Namespaces = supportedNamespaces(spec.Linux.Namespaces)

	cID := args[0]

	if isCRISandbox(spec) {
		// NOTE(antoineco): alternatively we could start the sandbox container
		// using (upstream) libcontainer, providing that we are willing to import
		// it as a dependency additionally to libmocktainer.
		if len(spec.Process.Args) == 0 {
			return fmt.Errorf("sandbox container has no process arg")
		}
		cArgPath, err := securejoin.SecureJoin(spec.Root.Path, spec.Process.Args[0])
		if err != nil {
			return fmt.Errorf("joining path components: %w", err)
		}
		if cArgPath, err = filepath.Abs(cArgPath); err != nil {
			return err
		}
		spec.Process.Args[0] = cArgPath
	} else {
		cArgs, err := genMachineArgs(ctx, cID, rootDir, spec.Root.Path)
		if err != nil {
			return fmt.Errorf("generating machine args: %w", err)
		}
		spec.Process.Args = cArgs

		if spec.Annotations == nil {
			spec.Annotations = make(map[string]string, 1)
		}
		spec.Annotations[specAnnotUnikernel] = ""
	}

	c, err := createContainer(cID, rootDir, spec)
	if err != nil {
		return fmt.Errorf("creating container environment: %w", err)
	}

	if err = bootstrapContainer(ctx, c, spec.Process, pidFile); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	return nil
}

const specFile = "config.json"

// loadSpec loads the OCI runtime specification from the provided path.
func loadSpec() (*rtspec.Spec, error) {
	f, err := os.Open(specFile)
	if err != nil {
		return nil, fmt.Errorf("opening spec file: %w", err)
	}
	defer f.Close()

	spec := &rtspec.Spec{}
	if err = json.NewDecoder(f).Decode(spec); err != nil {
		return nil, fmt.Errorf("decoding JSON spec: %w", err)
	}

	return spec, nil
}

// isCRISandbox returns whether the current container is a CRI sandbox
// (Kubernetes "pause" container).
func isCRISandbox(spec *rtspec.Spec) bool {
	return spec.Annotations[specAnnotCRIContainerType] == "sandbox"
}

// genMachineArgs returns the command-line arguments for starting a unikernel machine.
func genMachineArgs(ctx context.Context, cID, rootDir, bundleRoot string) (args []string, retErr error) {
	m, err := newMachine(cID, rootDir, bundleRoot)
	if err != nil {
		return nil, fmt.Errorf("creating machine: %w", err)
	}

	mSvc, err := newMachineService(ctx)
	if err != nil {
		return nil, fmt.Errorf("instantiating machine service: %w", err)
	}

	// HACK: QMP socket paths are unusable with long container IDs (e.g. in Kubernetes).
	// Use a short ephemeral directory as a workaround.
	//
	//   FATA[0000] failed to create shim task: OCI runtime create failed:
	//   generating machine args: creating machine: could not start and wait for QEMU process:
	//   qemu-system-x86_64: -qmp unix:/run/containerd/runc/default/bf6f970d280ba13c36f34f8d8765603244362620d12b405a5e91bd822ead4c8a/qemu_control.sock,server,nowait:
	//   UNIX socket path '/run/containerd/runc/default/bf6f970d280ba13c36f34f8d8765603244362620d12b405a5e91bd822ead4c8a/qemu_control.sock' is too long
	//   Path must be less than 108 bytes
	//
	origStateDir := m.Status.StateDir
	m.Status.StateDir, err = os.MkdirTemp("", "runu-")
	if err != nil {
		return nil, fmt.Errorf("creating short temp state dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(m.Status.StateDir); err != nil {
			retErr = combineErrors(retErr, fmt.Errorf("deleting short temp state dir: %w", err))
		}
	}()

	// HACK: this starts a QEMU process without starting the guest (-S flag, "freeze CPU at startup").
	// We only do that for the sake of cloning the machine's startup command and
	// arguments, and set those same arguments inside the OCI container's config.
	m, err = mSvc.Create(ctx, m)
	if err != nil {
		return nil, fmt.Errorf("creating machine: %w", err)
	}

	defer func() {
		if _, err := mSvc.Stop(ctx, m); err != nil {
			retErr = combineErrors(retErr, fmt.Errorf("stopping machine: %w", err))
		}
		if _, err := mSvc.Delete(ctx, m); err != nil {
			retErr = combineErrors(retErr, fmt.Errorf("deleting machine: %w", err))
		}
	}()

	qCfg, ok := m.Status.PlatformConfig.(qemu.QemuConfig)
	if !ok {
		return nil, fmt.Errorf("machine does not embed a QEMU platform config")
	}

	var bin string
	switch mArch := m.Spec.Architecture; mArch {
	case "x86_64":
		bin = qemu.QemuSystemX86
	case "arm":
		bin = qemu.QemuSystemArm
	case "arm64":
		bin = qemu.QemuSystemAarch64
	default:
		return nil, fmt.Errorf("unsupported machine architecture: %s", mArch)
	}

	if config.G[config.KraftKit](ctx).Qemu != "" {
		bin = config.G[config.KraftKit](ctx).Qemu
	}

	exe, err := exec.NewExecutable(bin, qCfg)
	if err != nil {
		return nil, fmt.Errorf("preparing machine executable: %w", err)
	}

	args = sanitizeQemuArgs(exe.Args())

	// HACK: restore previously overridden stateDir
	for i := range args {
		args[i] = strings.Replace(args[i], m.Status.StateDir, origStateDir, 1)
	}

	return append([]string{bin}, args...), nil
}

// sanitizeQemuArgs filters out undesired QEMU commands line arguments, such as
// the "-daemonize" flag, from the given list.
func sanitizeQemuArgs(qemuArgs []string) []string {
	xFlags := set.NewStringSet("-daemonize", "-S")
	xFlagsVal := set.NewStringSet("-qmp", "-monitor", "-serial")

	filtered := qemuArgs[:0]
	for i := 0; i < len(qemuArgs); i++ {
		switch {
		case xFlags.Contains(qemuArgs[i]):
		case xFlagsVal.Contains(qemuArgs[i]):
			i++
		default:
			filtered = append(filtered, qemuArgs[i])
		}
	}
	return filtered
}

// newMachine returns a new Machine with the given ID.
func newMachine(mID string, rootDir, bundleRoot string) (*machineapi.Machine, error) {
	kPath, err := kernelAbsPath(bundleRoot)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path of kernel: %w", err)
	}

	irdPath, err := initrdAbsPath(bundleRoot)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path of initrd: %w", err)
	}

	kArch, err := kernelArchitecture(kPath)
	if err != nil {
		return nil, fmt.Errorf("getting kernel architecture: %w", err)
	}

	stateDir, err := securejoin.SecureJoin(rootDir, mID)
	if err != nil {
		return nil, fmt.Errorf("joining path components: %w", err)
	}

	// TODO(antoineco): convert CPU shares and memory limits from runtime spec
	// into Machine resource requirements.
	m := &machineapi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name: mID,
		},
		Spec: machineapi.MachineSpec{
			Platform:     plat.String(),
			Architecture: kArch,
		},
		Status: machineapi.MachineStatus{
			KernelPath: kPath,
			InitrdPath: irdPath,
			StateDir:   stateDir,
		},
	}

	return m, nil
}

const (
	kernelRelPath = "unikraft/bin/kernel"
	initrdRelPath = "unikraft/bin/initrd"
)

// kernelAbsPath returns the absolute path of the bundle's kernel.
func kernelAbsPath(bundleRoot string) (string, error) {
	return fileAbsPath(bundleRoot, kernelRelPath)
}

// initrdAbsPath returns the absolute path of the bundle's initrd.
func initrdAbsPath(bundleRoot string) (string, error) {
	p, err := fileAbsPath(bundleRoot, initrdRelPath)
	if err != nil {
		if pErr := (*os.PathError)(nil); errors.As(err, &pErr) && os.IsNotExist(pErr) {
			return "", nil
		}
	}
	return p, err
}

// fileAbsPath returns the absolute path of the given file relative to the
// bundle root, and performs some basic sanity checks on the value.
func fileAbsPath(bundleRoot, relPath string) (string, error) {
	path, err := securejoin.SecureJoin(bundleRoot, relPath)
	if err != nil {
		return "", fmt.Errorf("joining path components: %w", err)
	}

	if path, err = filepath.Abs(path); err != nil {
		return "", err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("reading information about file: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("file is not regular: %s", path)
	}

	return path, nil
}

// newMachineService returns a new MachineService.
func newMachineService(ctx context.Context) (machineapi.MachineService, error) {
	ms, ok := mplatform.Strategies()[plat]
	if !ok {
		return nil, fmt.Errorf("unsupported platform driver: %s", plat)
	}

	return ms.NewMachineV1alpha1(ctx)
}

// kernelArchitecture returns the architecture of the given kernel file.
// https://github.com/unikraft/kraftkit/blob/v0.6.4/cmd/kraft/run/runner_kernel.go#L47-L85
func kernelArchitecture(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening ELF file: %w", err)
	}
	defer f.Close()

	var arch string

	switch f.Machine {
	case elf.EM_X86_64, elf.EM_386:
		arch = "x86_64"
	case elf.EM_ARM:
		arch = "arm"
	case elf.EM_AARCH64:
		arch = "arm64"
	default:
		return "", fmt.Errorf("unsupported kernel architecture: %s", f.Machine)
	}

	return arch, nil
}

// supportedNamespaces filters out unsupported Linux namespaces from the given list.
func supportedNamespaces(nss []rtspec.LinuxNamespace) []rtspec.LinuxNamespace {
	knownNs := set.NewStringSet(specconv.KnownNamespaces()...)

	filtered := nss[:0]
	for _, ns := range nss {
		if knownNs.Contains(string(ns.Type)) {
			filtered = append(filtered, ns)
		}
	}
	return filtered
}

// createContainer creates a new container in a stopped state for the given
// container id inside the provided state directory (root).
func createContainer(cID, rootDir string, spec *rtspec.Spec) (*libcontainer.Container, error) {
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		Spec: spec,
	})
	if err != nil {
		return nil, fmt.Errorf("creating libcontainer configuration: %w", err)
	}

	return libcontainer.Create(rootDir, cID, config)
}

// bootstrapContainer starts the container bootstrap (init) process.
func bootstrapContainer(ctx context.Context, c *libcontainer.Container, sp *rtspec.Process, pidFile string) (retErr error) {
	defer func() {
		if retErr != nil {
			if err := c.Destroy(); err != nil {
				retErr = combineErrors(retErr, fmt.Errorf("destroying container: %w", err))
			}
		}
	}()

	p := newProcess(sp)
	p.LogLevel = strconv.Itoa(int(log.G(ctx).Level))

	if err := c.Start(p); err != nil {
		return fmt.Errorf("starting container init process: %w", err)
	}

	if pidFile != "" {
		if err := createPidFile(pidFile, p); err != nil {
			_ = p.Signal(syscall.SIGKILL)
			_, _ = p.Wait()
			return err
		}
	}

	return nil
}

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(sp *rtspec.Process) *libcontainer.Process {
	return &libcontainer.Process{
		Args:   sp.Args,
		Env:    sp.Env,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// createPidFile creates a file with the processes pid inside it atomically
// it creates a temp file with the paths filename + '.' infront of it
// then renames the file
func createPidFile(path string, process *libcontainer.Process) error {
	pid, err := process.Pid()
	if err != nil {
		return err
	}
	var (
		tmpDir  = filepath.Dir(path)
		tmpName = filepath.Join(tmpDir, "."+filepath.Base(path))
	)
	f, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0o666)
	if err != nil {
		return err
	}
	_, err = f.WriteString(strconv.Itoa(pid))
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// combineErrors is a helper for handling multiple potential errors, combining
// them as necessary. It is meant to be used in a deferred function.
func combineErrors(original, additional error) error {
	switch {
	case additional == nil:
		return original
	case original != nil:
		return fmt.Errorf("%w. Additionally: %w", original, additional)
	default:
		return additional
	}
}
