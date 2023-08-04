// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	zip "api.zip"
	"github.com/acorn-io/baaah/pkg/merr"
	"github.com/mitchellh/mapstructure"
	goprocess "github.com/shirou/gopsutil/v3/process"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/internal/logtail"
	"kraftkit.sh/internal/retrytimeout"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network/macaddr"
	"kraftkit.sh/machine/qemu/qmp"
	qmpapi "kraftkit.sh/machine/qemu/qmp/v7alpha2"
	"kraftkit.sh/unikraft/export/v0/ukargparse"
	"kraftkit.sh/unikraft/export/v0/uknetdev"
	"kraftkit.sh/unikraft/export/v0/vfscore"
)

const (
	QemuSystemX86     = "qemu-system-x86_64"
	QemuSystemArm     = "qemu-system-arm"
	QemuSystemAarch64 = "qemu-system-aarch64"
)

// machineV1alpha1Service ...
type machineV1alpha1Service struct {
	eopts []exec.ExecOption
}

// NewMachineV1alpha1Service implements kraftkit.sh/machine/platform.NewStrategyConstructor
func NewMachineV1alpha1Service(ctx context.Context, opts ...any) (machinev1alpha1.MachineService, error) {
	service := machineV1alpha1Service{}

	for _, opt := range opts {
		qopt, ok := opt.(MachineServiceV1alpha1Option)
		if !ok {
			panic("cannot apply non-MachineServiceV1alpha1Option type methods")
		}

		if err := qopt(&service); err != nil {
			return nil, err
		}
	}

	return &service, nil
}

// Create implements kraftkit.sh/api/machine/v1alpha1.MachineService.Create
func (service *machineV1alpha1Service) Create(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	if machine.Status.KernelPath == "" {
		return machine, fmt.Errorf("empty kernel path")
	}

	if _, err := os.Stat(machine.Status.KernelPath); err != nil && os.IsNotExist(err) {
		return machine, fmt.Errorf("supplied kernel path does not exist: %s", machine.Status.KernelPath)
	}

	var bin string

	switch machine.Spec.Architecture {
	case "x86_64", "amd64":
		bin = QemuSystemX86
	case "arm":
		bin = QemuSystemArm
	case "arm64":
		bin = QemuSystemAarch64
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", machine.Spec.Architecture)
	}

	// Determine the version of QEMU so as to both determine whether it is a
	// suitable version and to adjust the supplied command-line arguments.
	qemuVersion, err := GetQemuVersionFromBin(ctx, bin)
	if err != nil {
		return machine, err
	}

	if qemuVersion.LessThan(QemuVersion4_2_0) {
		return machine, fmt.Errorf("unsupported QEMU version: %s: please upgrade to a newer version", qemuVersion.String())
	}

	// Determine the QEMU machine type to use
	qemuAccels, err := GetQemuMachineAccelFromBin(ctx, bin)
	if err != nil {
		return machine, err
	}

	if machine.Spec.Emulation {
		emulation := false
		for _, accel := range qemuAccels {
			if accel == QemuMachineAccelTCG {
				emulation = true
				break
			}
		}

		if !emulation {
			return machine, fmt.Errorf("emulation requested but TCG is not available")
		}
	} else {
		platform := false
		for _, accel := range qemuAccels {
			if accel == QemuMachineAccelKVM {
				platform = true
				break
			}
		}

		if !platform {
			return machine, fmt.Errorf("platform %s requested but it's not available", QemuMachineAccelKVM)
		}
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

	group, err := user.LookupGroup(config.G[config.KraftKit](ctx).UserGroup)
	if err == nil {
		gid, err := strconv.ParseInt(group.Gid, 10, 32)
		if err != nil {
			return machine, fmt.Errorf("could not parse group ID for kraftkit: %w", err)
		}

		if err := os.Chown(machine.Status.StateDir, os.Getuid(), int(gid)); err != nil {
			return machine, fmt.Errorf("could not change group ownership of machine state dir: %w", err)
		}
	} else {
		log.G(ctx).
			WithField("error", err).
			Debug("kraftkit group not found, falling back to current user")
	}

	// Set and create the log file for this machine
	if len(machine.Status.LogFile) == 0 {
		machine.Status.LogFile = filepath.Join(machine.Status.StateDir, "machine.log")
	}

	if machine.Spec.Resources.Requests.Memory().Value() == 0 {
		quantity, err := resource.ParseQuantity("64M")
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}

		machine.Spec.Resources.Requests[corev1.ResourceMemory] = quantity
	}

	if machine.Spec.Resources.Requests.Cpu().Value() == 0 {
		quantity, err := resource.ParseQuantity("1")
		if err != nil {
			machine.Status.State = machinev1alpha1.MachineStateFailed
			return machine, err
		}

		machine.Spec.Resources.Requests[corev1.ResourceCPU] = quantity
	}

	qopts := []QemuOption{
		WithDaemonize(true),
		WithNoGraphic(true),
		WithPidFile(filepath.Join(machine.Status.StateDir, "machine.pid")),
		WithNoReboot(true),
		WithNoStart(true),
		WithName(string(machine.ObjectMeta.UID)),
		WithKernel(machine.Status.KernelPath),
		WithVGA(QemuVGANone),
		WithMemory(QemuMemory{
			// The value returned from Memory() is in bytes
			Size: uint64(machine.Spec.Resources.Requests.Memory().Value() / 1000000),
			Unit: QemuMemoryUnitMB,
		}),
		// Create a QMP connection solely for manipulating the machine
		WithQMP(QemuHostCharDevUnix{
			SocketDir: machine.Status.StateDir,
			Name:      "qemu_control",
			NoWait:    true,
			Server:    true,
		}),
		// Create a QMP connection solely for listening to events
		WithQMP(QemuHostCharDevUnix{
			SocketDir: machine.Status.StateDir,
			Name:      "qemu_events",
			NoWait:    true,
			Server:    true,
		}),
		WithSerial(QemuHostCharDevFile{
			Monitor:  false,
			Filename: machine.Status.LogFile,
		}),
		WithMonitor(QemuHostCharDevUnix{
			SocketDir: machine.Status.StateDir,
			Name:      "qemu_mon",
			NoWait:    true,
			Server:    true,
		}),
		WithSMP(QemuSMP{
			CPUs:    uint64(machine.Spec.Resources.Requests.Cpu().Value()),
			Threads: 1,
			Sockets: 1,
		}),
		WithVGA(QemuVGANone),
		WithRTC(QemuRTC{
			Base: QemuRTCBaseUtc,
		}),
		WithDisplay(QemuDisplayNone{}),
		WithParallel(QemuHostCharDevNone{}),
	}

	// TODO: Parse Rootfs types
	if len(machine.Status.InitrdPath) > 0 {
		qopts = append(qopts,
			WithInitRd(machine.Status.InitrdPath),
		)
	}

	if len(machine.Spec.Ports) > 0 {
		// Start MAC addresses iteratively.
		startMac, err := macaddr.GenerateMacAddress(true)
		if err != nil {
			return machine, err
		}

		for i, port := range machine.Spec.Ports {
			mac := port.MacAddress
			if mac == "" {
				startMac = macaddr.IncrementMacAddress(startMac)
				mac = startMac.String()
			}

			hostnetid := fmt.Sprintf("hostnet%d", i)
			qopts = append(qopts,
				WithDevice(QemuDeviceVirtioNetPci{
					Mac:    mac,
					Netdev: hostnetid,
				}),
				WithNetDevice(QemuNetDevUser{
					Id:      hostnetid,
					Hostfwd: fmt.Sprintf("%s::%d-:%d", port.Protocol, port.HostPort, port.MachinePort),
				}),
			)
		}
	}

	kernelArgs, err := ukargparse.Parse(machine.Spec.KernelArgs...)
	if err != nil {
		return machine, err
	}

	if len(machine.Spec.Networks) > 0 {
		// Start MAC addresses iteratively.  Each interface will have the last
		// hexdecimal byte increase by 1 starting at 1, allowing for easy-to-spot
		// interface IDs from the MAC address.  The return value below returns `:00`
		// as the last byte.
		startMac, err := macaddr.GenerateMacAddress(true)
		if err != nil {
			return machine, err
		}

		i := 0 // host network ID.

		// Iterate over each interface of each network interface associated with
		// this machine and attach it as a device.
		for _, network := range machine.Spec.Networks {
			for _, iface := range network.Interfaces {
				mac := iface.Spec.MacAddress
				if mac == "" {
					// Increase the MAC address value by 1 such that we are able to
					// identify interface IDs.
					startMac = macaddr.IncrementMacAddress(startMac)
					mac = startMac.String()
				}

				hostnetid := fmt.Sprintf("hostnet%d", i)
				qopts = append(qopts,
					// TODO(nderjung): The network device should be customizable based on
					// the network spec or machine spec.  Additional insight can be provided
					// by inspecting the KConfig options.  Potentially the MachineSpec is
					// updated to reflect different systems or provide access to the
					// KConfig values.
					WithDevice(QemuDeviceVirtioNetPci{
						Netdev: hostnetid,
						Mac:    mac,
					}),
					WithNetDevice(QemuNetDevTap{
						Id:         hostnetid,
						Ifname:     iface.Spec.IfName,
						Br:         network.IfName,
						Script:     "no", // Disable execution
						Downscript: "no", // Disable execution
					}),
				)

				// Assign the first interface statically via command-line arguments, also
				// checking if the built-in arguments for
				if !kernelArgs.Contains(uknetdev.ParamIpv4Addr) && i == 0 {
					kernelArgs = append(kernelArgs,
						uknetdev.ParamIpv4Addr.WithValue(iface.Spec.IP),
						uknetdev.ParamIpv4GwAddr.WithValue(network.Gateway),
						uknetdev.ParamIpv4SubnetMask.WithValue(network.Netmask),
					)
				}

				// Increment the host network ID for additional interfaces.
				i++
			}
		}
	}

	var fstab []string

	for i, vol := range machine.Spec.Volumes {
		switch vol.Spec.Driver {
		case "9pfs":
			hvirtioid := fmt.Sprintf("hvirtio%d", i+1)
			mounttag := fmt.Sprintf("fs%d", i+1)
			qopts = append(qopts,
				WithFsDevice(QemuFsDevLocal{
					SecurityModel: QemuFsDevLocalSecurityModelPassthrough,
					Id:            hvirtioid,
					Path:          vol.Spec.Source,
				}),
				WithDevice(QemuDeviceVirtio9pPci{
					Fsdev:    hvirtioid,
					MountTag: mounttag,
				}),
			)

			fstab = append(fstab, vfscore.NewFstabEntry(
				mounttag,
				vol.Spec.Destination,
				vol.Spec.Driver,
				// TODO(nderjung): Options (such as ro/rw) are yet supported by
				// Unikraft:
				"",
				"",
			).String())

		default:
			return machine, fmt.Errorf("unsupported QEMU volume driver: %v", vol.Spec.Driver)
		}
	}

	if len(fstab) > 0 {
		kernelArgs = append(kernelArgs,
			vfscore.ParamVfsFstab.WithValue(fstab),
		)
	}

	// TODO(nderjung): This is standard "Unikraft" positional argument syntax
	// (kernel args and application arguments separated with "--").  The resulting
	// string should be standardized through a central function.
	args := kernelArgs.Strings()
	if len(args) > 0 {
		args = append(args, "--")
	}
	args = append(args, machine.Spec.ApplicationArgs...)
	qopts = append(qopts, WithAppend(args...))

	switch machine.Spec.Architecture {
	case "x86_64", "amd64":
		if machine.Spec.Emulation {
			qopts = append(qopts,
				WithMachine(QemuMachine{
					Type: QemuMachineTypePC,
				}),
				WithCPU(QemuCPU{
					CPU: QemuCPUX86Qemu64,
					On:  QemuCPUFeatures{QemuCPUFeaturePdpe1gb},
					Off: QemuCPUFeatures{QemuCPUFeatureVmx, QemuCPUFeatureSvm},
				}),
			)
		} else {
			qopts = append(qopts,
				WithEnableKVM(true),
				WithMachine(QemuMachine{
					Type:         QemuMachineTypePC,
					Accelerators: []QemuMachineAccelerator{QemuMachineAccelKVM},
				}),
				WithCPU(QemuCPU{
					CPU: QemuCPUX86Host,
					On:  QemuCPUFeatures{QemuCPUFeatureX2apic},
					Off: QemuCPUFeatures{QemuCPUFeaturePmu},
				}),
			)
		}
		if qemuVersion.LessThan(QemuVersion8_0_0) {
			qopts = append(qopts,
				WithDevice(QemuDeviceSga{}),
			)
		}
	case "arm", "arm64":
		qopts = append(qopts,
			WithMachine(QemuMachine{
				Type: QemuMachineTypeVirt,
			}),
			WithCPU(QemuCPU{
				CPU: QemuCPUArmCortexA53,
			}),
		)

	default:
		return nil, fmt.Errorf("unsupported architecture: %s", machine.Spec.Architecture)
	}

	// Create a log file just for the QEMU process which can be used to debug
	// issues when starting the VMM.
	qemuLogFile := filepath.Join(machine.Status.StateDir, "qemu.log")
	fi, err := os.Create(qemuLogFile)
	if err != nil {
		return machine, err
	}

	defer fi.Close()

	service.eopts = append(service.eopts,
		exec.WithStdout(fi),
	)

	qcfg, err := NewQemuConfig(qopts...)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not generate QEMU config: %v", err)
	}

	machine.Status.PlatformConfig = *qcfg

	e, err := exec.NewExecutable(bin, *qcfg)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not prepare QEMU executable: %v", err)
	}

	process, err := exec.NewProcessFromExecutable(e, service.eopts...)
	if err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed
		return machine, fmt.Errorf("could not prepare QEMU process: %v", err)
	}

	machine.CreationTimestamp = metav1.Now()

	// Start and also wait for the process to be released, this ensures the
	// program is actively being executed.
	if err := process.StartAndWait(ctx); err != nil {
		machine.Status.State = machinev1alpha1.MachineStateFailed

		// Propagate the contents of the QEMU log file as an error
		if errLog, err2 := os.ReadFile(qemuLogFile); err2 == nil {
			err = errors.Join(fmt.Errorf(strings.TrimSpace(string(errLog))), err)
		}

		return machine, fmt.Errorf("could not start and wait for QEMU process: %v", err)
	}

	machine.Status.State = machinev1alpha1.MachineStateCreated

	return machine, nil
}

// Update implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (service *machineV1alpha1Service) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	panic("not implemented: kraftkit.sh/machine/qemu.machineV1alpha1Service.Update")
}

// getQEMUConfigFromPlatformConfig converts the provided platformConfig
// interface into meaningful QemuConfig.
func getQEMUConfigFromPlatformConfig(platformConfig interface{}) (*QemuConfig, error) {
	qcfgptr, ok := platformConfig.(*QemuConfig)
	if ok {
		return qcfgptr, nil
	}

	// If we cannot directly cast it to the structure, attempt to decode a
	// mapstructure version of the same configuration.
	var qcfg QemuConfig
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result: &qcfg,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			// Directly embbed a less-erroring version of StringToIPHookFunc[0] which
			// does not return an error when parsing an IP that returns nil.
			// [0]: https://github.com/mitchellh/mapstructure/blob/bf980b35cac4dfd34e05254ee5aba086504c3f96/decode_hooks.go#L141-L163
			func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
				if f.Kind() != reflect.String {
					return data, nil
				}
				if t != reflect.TypeOf(net.IP{}) {
					return data, nil
				}

				// Instead of parsing it, just return an empty net.IP structure, since
				// we do not yet set IP addresses with Firecracker's configuration.
				return net.IP{}, nil
			},
		),
	})
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(platformConfig); err != nil {
		return nil, err
	}

	return &qcfg, nil
}

func qmpClientHandshake(conn *net.Conn) (*qmpapi.QEMUMachineProtocolClient, error) {
	qmpClient := qmpapi.NewQEMUMachineProtocolClient(*conn)

	greeting, err := qmpClient.Greeting()
	if err != nil {
		return nil, err
	}

	_, err = qmpClient.Capabilities(qmpapi.CapabilitiesRequest{
		Arguments: qmpapi.CapabilitiesRequestArguments{
			Enable: greeting.Qmp.Capabilities,
		},
	})
	if err != nil {
		return nil, err
	}

	return qmpClient, nil
}

func (service *machineV1alpha1Service) QMPClient(ctx context.Context, machine *machinev1alpha1.Machine) (*qmpapi.QEMUMachineProtocolClient, error) {
	qcfg, err := getQEMUConfigFromPlatformConfig(machine.Status.PlatformConfig)
	if err != nil {
		return nil, err
	}

	// Always use index 0 for manipulating the machine
	conn, err := qcfg.QMP[0].Connection()
	if err != nil {
		return nil, err
	}

	return qmpClientHandshake(&conn)
}

func processFromPidFile(pidFile string) (*goprocess.Process, error) {
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return nil, fmt.Errorf("could not read pid file: %v", err)
	}

	pid, err := strconv.ParseUint(strings.TrimSpace(string(pidData)), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not convert pid string \"%s\" to uint64: %v", pidData, err)
	}

	process, err := goprocess.NewProcess(int32(pid))
	if err != nil {
		return nil, fmt.Errorf("could not look up process %d: %v", pid, err)
	}

	return process, nil
}

// Watch implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (service *machineV1alpha1Service) Watch(ctx context.Context, machine *machinev1alpha1.Machine) (chan *machinev1alpha1.Machine, chan error, error) {
	events := make(chan *machinev1alpha1.Machine)
	errs := make(chan error)

	qcfg, ok := machine.Status.PlatformConfig.(QemuConfig)
	if !ok {
		return nil, nil, fmt.Errorf("cannot cast QEMU platform configuration from machine status")
	}

	// Always use index 1 for monitoring events
	conn, err := qcfg.QMP[1].Connection()
	if err != nil {
		return nil, nil, err
	}

	// Perform the handshake
	_, err = qmpClientHandshake(&conn)
	if err != nil {
		return nil, nil, err
	}

	monitor, err := qmp.NewQMPEventMonitor(conn,
		qmpapi.EventTypes(),
		nil,
	)
	if err != nil {
		return nil, nil, err
	}

	// firstCall is used to initialize the channel with the current state of the
	// machine, so that it can be immediately acted upon.
	firstCall := true

	go func() {
	accept:
		for {
			// First check if the context has been cancelled
			select {
			case <-ctx.Done():
				break accept
			default:
			}

			// Check the current state
			machine, err := service.Get(ctx, machine)
			if err != nil {
				errs <- err
				continue
			}

			// Initialize with the current state
			if firstCall {
				events <- machine
				firstCall = false
			}

			// Listen for changes in state
			event, err := monitor.Accept()
			if err != nil {
				errs <- err
				continue
			}

			// Send the event through the channel
			switch event.Event {
			case qmpapi.EVENT_STOP, qmpapi.EVENT_SUSPEND, qmpapi.EVENT_POWERDOWN:
				machine.Status.State = machinev1alpha1.MachineStatePaused
				events <- machine

			case qmpapi.EVENT_RESUME:
				machine.Status.State = machinev1alpha1.MachineStateRunning
				events <- machine

			case qmpapi.EVENT_RESET, qmpapi.EVENT_WAKEUP:
				machine.Status.State = machinev1alpha1.MachineStateRestarting
				events <- machine

			case qmpapi.EVENT_SHUTDOWN:
				machine.Status.State = machinev1alpha1.MachineStateExited
				events <- machine

				if !qcfg.NoShutdown {
					break accept
				}
			default:
				errs <- fmt.Errorf("unsupported event: %s", event.Event)
			}
		}
	}()

	return events, errs, nil
}

// Start implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (service *machineV1alpha1Service) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	qmpClient, err := service.QMPClient(ctx, machine)
	if err != nil {
		return machine, fmt.Errorf("could not start qemu instance: %v", err)
	}

	defer qmpClient.Close()
	_, err = qmpClient.Cont(qmpapi.ContRequest{})
	if err != nil {
		return machine, err
	}

	qcfg, ok := machine.Status.PlatformConfig.(QemuConfig)
	if !ok {
		return machine, fmt.Errorf("cannot cast QEMU platform configuration from machine status")
	}

	process, err := processFromPidFile(qcfg.PidFile)
	if err != nil {
		return machine, err
	}

	machine.Status.Pid = process.Pid
	machine.Status.State = machinev1alpha1.MachineStateRunning
	machine.Status.StartedAt = time.Now()

	return machine, nil
}

// Pause implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (service *machineV1alpha1Service) Pause(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	qmpClient, err := service.QMPClient(ctx, machine)
	if err != nil {
		return machine, fmt.Errorf("could not start qemu instance: %v", err)
	}

	defer qmpClient.Close()

	_, err = qmpClient.Stop(qmpapi.StopRequest{})
	if err != nil {
		return machine, err
	}

	machine.Status.State = machinev1alpha1.MachineStatePaused

	return machine, nil
}

// Logs implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (service *machineV1alpha1Service) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	out, errOut, err := logtail.NewLogTail(ctx, machine.Status.LogFile)
	if err != nil {
		return nil, nil, err
	}

	// Wait and trim the preamble from the logs before returning
	for {
		select {
		case line := <-out:
			if !qemuShowSgaBiosPreamble {
				if strings.Contains(line, "Booting from ") {
					qemuShowSgaBiosPreamble = true
				}
				continue
			}
			return out, errOut, nil

		case err := <-errOut:
			return nil, nil, err

		case <-ctx.Done():
			return out, errOut, nil
		}
	}
}

// Get implements kraftkit.sh/api/machine/v1alpha1/MachineService.Get
func (service *machineV1alpha1Service) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	state := machinev1alpha1.MachineStateUnknown
	savedState := machine.Status.State

	qcfg, ok := machine.Status.PlatformConfig.(QemuConfig)
	if !ok {
		return machine, fmt.Errorf("cannot read QEMU platform configuration from machine status")
	}

	// Check if the process is alive, which ultimately indicates to us whether we
	// able to speak to the exposed QMP socket
	activeProcess := false
	if process, err := processFromPidFile(qcfg.PidFile); err == nil {
		activeProcess, err = process.IsRunning()
		if err != nil {
			state = machinev1alpha1.MachineStateExited
		}
	}

	exitedAt := machine.Status.ExitedAt
	exitCode := machine.Status.ExitCode

	defer func() {
		if exitCode >= 0 && machine.Status.ExitedAt.IsZero() {
			exitedAt = time.Now()
		}

		// Update the machine config with the latest values if they are different from
		// what we have on record
		if machine.Status.ExitedAt != exitedAt || machine.Status.ExitCode != exitCode {
			machine.Status.ExitedAt = exitedAt
			machine.Status.ExitCode = exitCode
		}

		// Set the start time to now if it was not previously set
		if machine.Status.StartedAt.IsZero() && state == machinev1alpha1.MachineStateRunning {
			machine.Status.StartedAt = time.Now()
		}

		// Finally, save the state if it is different from the what we have on
		// record
		if state != savedState {
			machine.Status.State = state
		}
	}()

	if !activeProcess {
		state = machinev1alpha1.MachineStateExited
		if savedState == machinev1alpha1.MachineStateRunning {
			exitCode = 1
		}
		return machine, nil
	}

	qmpClient, err := service.QMPClient(ctx, machine)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		state = machinev1alpha1.MachineStateExited
		exitCode = 1
		return machine, nil
	} else if err != nil {
		return machine, fmt.Errorf("could not attach to QMP client: %v", err)
	}

	defer qmpClient.Close()

	// Grab the actual state of the machine by querying QMP
	status, err := qmpClient.QueryStatus(qmpapi.QueryStatusRequest{})
	if err != nil {
		// We cannot amend the status at this point, even if the process is
		// alive, since it is not an indicator of the state of the VM, only of the
		// VMM.  So we return what we already know via LookupMachineConfig.
		return machine, fmt.Errorf("could not query machine status via QMP: %v", err)
	}

	// Map the QMP status to supported machine states
	switch status.Return.Status {
	case qmpapi.RUN_STATE_GUEST_PANICKED, qmpapi.RUN_STATE_INTERNAL_ERROR, qmpapi.RUN_STATE_IO_ERROR:
		state = machinev1alpha1.MachineStateFailed
		exitCode = 1

	case qmpapi.RUN_STATE_PAUSED:
		state = machinev1alpha1.MachineStatePaused
		exitCode = -1

	case qmpapi.RUN_STATE_RUNNING:
		state = machinev1alpha1.MachineStateRunning
		exitCode = -1

	case qmpapi.RUN_STATE_SHUTDOWN:
		state = machinev1alpha1.MachineStateExited
		exitCode = 0

	case qmpapi.RUN_STATE_SUSPENDED:
		state = machinev1alpha1.MachineStateSuspended
		exitCode = -1

	default:
		// qmpapi.RUN_STATE_SAVE_VM,
		// qmpapi.RUN_STATE_PRELAUNCH,
		// qmpapi.RUN_STATE_RESTORE_VM,
		// qmpapi.RUN_STATE_WATCHDOG,
		state = machinev1alpha1.MachineStateUnknown
		exitCode = -1
	}

	return machine, nil
}

// List implements kraftkit.sh/api/machine/v1alpha1.MachineService.List
func (service *machineV1alpha1Service) List(ctx context.Context, machines *machinev1alpha1.MachineList) (*machinev1alpha1.MachineList, error) {
	cached := machines.Items
	machines.Items = make([]zip.Object[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus], len(cached))

	// Iterate through each machine and grab the latest status
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

// Stop implements kraftkit.sh/api/machine/v1alpha1.MachineService.Stop
func (service *machineV1alpha1Service) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	qmpClient, err := service.QMPClient(ctx, machine)
	if err != nil {
		return machine, fmt.Errorf("could not stop qemu instance: %v", err)
	}

	defer qmpClient.Close()
	_, err = qmpClient.Quit(qmpapi.QuitRequest{})
	if err != nil {
		return machine, err
	}

	qcfg, ok := machine.Status.PlatformConfig.(QemuConfig)
	if !ok {
		return machine, fmt.Errorf("cannot read QEMU platform configuration from machine status")
	}

	machine.Status.State = machinev1alpha1.MachineStateExited

	if err := retrytimeout.RetryTimeout(5*time.Second, func() error {
		if _, err := os.ReadFile(qcfg.PidFile); !os.IsNotExist(err) {
			return fmt.Errorf("process still active")
		}

		return nil
	}); err != nil {
		return machine, err
	}

	return machine, nil
}

// Delete implements kraftkit.sh/api/machine/v1alpha1.MachineService.Delete
func (service *machineV1alpha1Service) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	qcfg, ok := machine.Status.PlatformConfig.(QemuConfig)
	if !ok {
		return machine, fmt.Errorf("cannot read QEMU platform configuration from machine status")
	}

	var errs merr.Errors

	err := os.RemoveAll(machine.Status.StateDir)
	if err != nil {
		errs = append(errs, fmt.Errorf("error deleting QEMU's state directory %s: %w", machine.Status.StateDir, err))
	}

	// Do not throw errors (likely these resources do not exist at this point)
	// when trying to remove ephemeral files that are controlled by the QEMU
	// process.
	_ = os.Remove(qcfg.QMP[0].Resource())
	_ = os.Remove(qcfg.QMP[1].Resource())

	return nil, errs.Err()
}
