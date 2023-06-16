// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	zip "api.zip"
	"github.com/acorn-io/baaah/pkg/merr"
	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	goprocess "github.com/shirou/gopsutil/v3/process"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/internal/retrytimeout"
	"kraftkit.sh/machine/network/macaddr"
	"kraftkit.sh/machine/qemu/qmp"
	qmpv1alpha "kraftkit.sh/machine/qemu/qmp/v1alpha"
	"kraftkit.sh/unikraft/export/v0/ukargparse"
	"kraftkit.sh/unikraft/export/v0/uknetdev"
)

const (
	QemuSystemX86     = "qemu-system-x86_64"
	QemuSystemArm     = "qemu-system-arm"
	QemuSystemAarch64 = "qemu-system-aarch64"

	// Log tail buffering
	DefaultTailBufferSize = 4 * 1024
	DefaultTailPeekSize   = 1024
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
	if machine.ObjectMeta.UID == "" {
		machine.ObjectMeta.UID = uuid.NewUUID()
	}

	machine.Status.State = machinev1alpha1.MachineStateUnknown

	if len(machine.Status.StateDir) == 0 {
		machine.Status.StateDir = filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, string(machine.ObjectMeta.UID))
	}

	if err := os.MkdirAll(machine.Status.StateDir, 0o755); err != nil {
		return machine, err
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
		WithEnableKVM(true),
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

	// TODO(nderjung): This is standard "Unikraft" positional argument syntax
	// (kernel args and application arguments separated with "--").  The resulting
	// string should be standardized through a central function.
	args := kernelArgs.Strings()
	if len(args) > 0 {
		args = append(args, "--")
	}
	args = append(args, machine.Spec.ApplicationArgs...)
	qopts = append(qopts, WithAppend(args...))

	var bin string

	switch machine.Spec.Architecture {
	case "x86_64", "amd64":
		bin = QemuSystemX86

		if machine.Spec.Emulation {
			qopts = append(qopts,
				WithMachine(QemuMachine{
					Type: QemuMachineTypePC,
				}),
				WithCPU(QemuCPU{
					CPU: QemuCPUX86Qemu64,
					On:  QemuCPUFeatures{QemuCPUFeatureVmx},
					Off: QemuCPUFeatures{QemuCPUFeatureSvm},
				}),
			)
		} else {
			qopts = append(qopts,
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

		qopts = append(qopts,
			WithDevice(QemuDeviceSga{}),
		)

	case "arm":
		bin = QemuSystemArm

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

func qmpClientHandshake(conn *net.Conn) (*qmpv1alpha.QEMUMachineProtocolClient, error) {
	qmpClient := qmpv1alpha.NewQEMUMachineProtocolClient(*conn)

	greeting, err := qmpClient.Greeting()
	if err != nil {
		return nil, err
	}

	_, err = qmpClient.Capabilities(qmpv1alpha.CapabilitiesRequest{
		Arguments: qmpv1alpha.CapabilitiesRequestArguments{
			Enable: greeting.Qmp.Capabilities,
		},
	})
	if err != nil {
		return nil, err
	}

	return qmpClient, nil
}

func (service *machineV1alpha1Service) QMPClient(ctx context.Context, machine *machinev1alpha1.Machine) (*qmpv1alpha.QEMUMachineProtocolClient, error) {
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
		qmpv1alpha.EventTypes(),
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
			case qmpv1alpha.EVENT_STOP, qmpv1alpha.EVENT_SUSPEND, qmpv1alpha.EVENT_POWERDOWN:
				machine.Status.State = machinev1alpha1.MachineStatePaused
				events <- machine

			case qmpv1alpha.EVENT_RESUME:
				machine.Status.State = machinev1alpha1.MachineStateRunning
				events <- machine

			case qmpv1alpha.EVENT_RESET, qmpv1alpha.EVENT_WAKEUP:
				machine.Status.State = machinev1alpha1.MachineStateRestarting
				events <- machine

			case qmpv1alpha.EVENT_SHUTDOWN:
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
	_, err = qmpClient.Cont(qmpv1alpha.ContRequest{})
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

	_, err = qmpClient.Stop(qmpv1alpha.StopRequest{})
	if err != nil {
		return machine, err
	}

	machine.Status.State = machinev1alpha1.MachineStatePaused

	return machine, nil
}

// Logs implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (service *machineV1alpha1Service) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	logs := make(chan string)
	errs := make(chan error)

	// Start a goroutine which continuously outputs the logs to the provided
	// channel.
	go service.logs(ctx, machine, &logs, &errs)

	return logs, errs, nil
}

func (service *machineV1alpha1Service) logs(ctx context.Context, machine *machinev1alpha1.Machine, logs *chan string, errs *chan error) {
	f, err := os.Open(machine.Status.LogFile)
	if err != nil {
		*errs <- err
		return
	}

	// var offset int64
	reader := bufio.NewReaderSize(f, DefaultTailBufferSize)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		*errs <- err
		return
	}

	if err := watcher.Add(machine.Status.LogFile); err != nil {
		*errs <- err
		return
	}

	// First read everything that already exists inside of the log file.
	for {
		// discard leading NUL bytes
		var discarded int

		for {
			b, _ := reader.Peek(DefaultTailPeekSize)
			i := bytes.LastIndexByte(b, '\x00')

			if i > 0 {
				n, _ := reader.Discard(i + 1)
				discarded += n
			}

			if i+1 < DefaultTailPeekSize {
				break
			}
		}

		s, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			*errs <- err
			return
		}

		// If we encounter EOF before a line delimiter, ReadBytes() will return the
		// remaining bytes, so push them back onto the buffer, rewind our seek
		// position, and wait for further file changes.  We also have to save our
		// dangling byte count in the event that we want to re-open the file and
		// seek to the end.
		if err == io.EOF {
			l := len(s)

			_, err = f.Seek(-int64(l), io.SeekCurrent)
			if err != nil {
				*errs <- err
				return
			}

			reader.Reset(f)
			break
		}

		if len(s) > discarded {
			*logs <- string(s[discarded:])
		}
	}

	for {
		select {
		case <-ctx.Done():
			*errs <- ctx.Err()
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			switch event.Op {
			case fsnotify.Write:
				var discarded int

				for {
					b, _ := reader.Peek(DefaultTailPeekSize)
					i := bytes.LastIndexByte(b, '\x00')

					if i > 0 {
						n, _ := reader.Discard(i + 1)
						discarded += n
					}

					if i+1 < DefaultTailPeekSize {
						break
					}
				}

				s, err := reader.ReadBytes('\n')
				if err != nil && err != io.EOF {
					*errs <- err
					return
				}

				// If we encounter EOF before a line delimiter, ReadBytes() will return the
				// remaining bytes, so push them back onto the buffer, rewind our seek
				// position, and wait for further file changes.  We also have to save our
				// dangling byte count in the event that we want to re-open the file and
				// seek to the end.
				if err == io.EOF {
					l := len(s)

					_, err = f.Seek(-int64(l), io.SeekCurrent)
					if err != nil {
						*errs <- err
						return
					}

					reader.Reset(f)
					continue
				}

				*logs <- string(s[discarded:])
			}
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
	status, err := qmpClient.QueryStatus(qmpv1alpha.QueryStatusRequest{})
	if err != nil {
		// We cannot amend the status at this point, even if the process is
		// alive, since it is not an indicator of the state of the VM, only of the
		// VMM.  So we return what we already know via LookupMachineConfig.
		return machine, fmt.Errorf("could not query machine status via QMP: %v", err)
	}

	// Map the QMP status to supported machine states
	switch status.Return.Status {
	case qmpv1alpha.RUN_STATE_GUEST_PANICKED, qmpv1alpha.RUN_STATE_INTERNAL_ERROR, qmpv1alpha.RUN_STATE_IO_ERROR:
		state = machinev1alpha1.MachineStateFailed
		exitCode = 1

	case qmpv1alpha.RUN_STATE_PAUSED:
		state = machinev1alpha1.MachineStatePaused
		exitCode = -1

	case qmpv1alpha.RUN_STATE_RUNNING:
		state = machinev1alpha1.MachineStateRunning
		exitCode = -1

	case qmpv1alpha.RUN_STATE_SHUTDOWN:
		state = machinev1alpha1.MachineStateExited
		exitCode = 0

	case qmpv1alpha.RUN_STATE_SUSPENDED:
		state = machinev1alpha1.MachineStateSuspended
		exitCode = -1

	default:
		// qmpv1alpha.RUN_STATE_SAVE_VM,
		// qmpv1alpha.RUN_STATE_PRELAUNCH,
		// qmpv1alpha.RUN_STATE_RESTORE_VM,
		// qmpv1alpha.RUN_STATE_WATCHDOG,
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
	_, err = qmpClient.Quit(qmpv1alpha.QuitRequest{})
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

	errs = append(errs, os.Remove(machine.Status.LogFile))

	// Do not throw errors (likely these resources do not exist at this point)
	// when trying to remove ephemeral files that are controlled by the QEMU
	// process.
	_ = os.Remove(qcfg.QMP[0].Resource())
	_ = os.Remove(qcfg.QMP[1].Resource())

	return nil, errs.Err()
}
