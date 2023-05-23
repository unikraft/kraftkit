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
	"strconv"
	"strings"
	"time"

	"kraftkit.sh/exec"
	"kraftkit.sh/internal/retrytimeout"
	"kraftkit.sh/machine"
	"kraftkit.sh/machine/driveropts"
	"kraftkit.sh/machine/qemu/qmp"
	qmpv1alpha "kraftkit.sh/machine/qemu/qmp/v1alpha"

	"github.com/fsnotify/fsnotify"
	goprocess "github.com/shirou/gopsutil/v3/process"
)

const (
	QemuSystemX86     = "qemu-system-x86_64"
	QemuSystemArm     = "qemu-system-arm"
	QemuSystemAarch64 = "qemu-system-aarch64"

	// Log tail buffering
	DefaultTailBufferSize = 4 * 1024
	DefaultTailPeekSize   = 1024
)

type QemuDriver struct {
	dopts *driveropts.DriverOptions
}

func NewQemuDriver(opts ...driveropts.DriverOption) (*QemuDriver, error) {
	dopts, err := driveropts.NewDriverOptions(opts...)
	if err != nil {
		return nil, err
	}

	if dopts.Store == nil {
		return nil, fmt.Errorf("cannot instantiate QEMU driver without machine store")
	}

	driver := QemuDriver{
		dopts: dopts,
	}

	return &driver, nil
}

func (qd *QemuDriver) Create(ctx context.Context, opts ...machine.MachineOption) (mid machine.MachineID, err error) {
	mcfg, err := machine.NewMachineConfig(opts...)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could build machine config: %v", err)
	}

	if mid, err = machine.NewRandomMachineID(); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not generate new machine ID: %v", err)
	}

	mcfg.ID = mid

	pidFile := filepath.Join(qd.dopts.RuntimeDir, mid.String()+".pid")

	// Set and create the log file for this machine
	if mcfg.LogFile == "" {
		mcfg.LogFile = filepath.Join(qd.dopts.RuntimeDir, mid.String()+".log")
	}

	qopts := []QemuOption{
		WithDaemonize(true),
		WithEnableKVM(true),
		WithNoGraphic(true),
		WithNoReboot(true),
		WithNoStart(true),
		WithPidFile(pidFile),
		WithName(mid.String()),
		WithKernel(mcfg.KernelPath),
		WithAppend(mcfg.Arguments...),
		WithVGA(QemuVGANone),
		WithMemory(QemuMemory{
			Size: mcfg.MemorySize,
			Unit: QemuMemoryUnitMB,
		}),
		// Create a QMP connection solely for manipulating the machine
		WithQMP(QemuHostCharDevUnix{
			SocketDir: qd.dopts.RuntimeDir,
			Name:      mid.String() + "_control",
			NoWait:    true,
			Server:    true,
		}),
		// Create a QMP connection solely for listening to events
		WithQMP(QemuHostCharDevUnix{
			SocketDir: qd.dopts.RuntimeDir,
			Name:      mid.String() + "_events",
			NoWait:    true,
			Server:    true,
		}),
		WithSerial(QemuHostCharDevFile{
			Monitor:  false,
			Filename: mcfg.LogFile,
		}),
		WithMonitor(QemuHostCharDevUnix{
			SocketDir: qd.dopts.RuntimeDir,
			Name:      mid.String() + "_mon",
			NoWait:    true,
			Server:    true,
		}),
		WithSMP(QemuSMP{
			CPUs:    mcfg.NumVCPUs,
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

	if len(mcfg.InitrdPath) > 0 {
		qopts = append(qopts,
			WithInitRd(mcfg.InitrdPath),
		)
	}

	var bin string

	switch mcfg.Architecture {
	case "x86_64", "amd64":
		bin = QemuSystemX86

		if mcfg.HardwareAcceleration {
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
		} else {
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
		return machine.NullMachineID, fmt.Errorf("unsupported architecture: %s", mcfg.Architecture)
	}

	qcfg, err := NewQemuConfig(qopts...)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could not generate QEMU config: %v", err)
	}

	e, err := exec.NewExecutable(bin, *qcfg)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could not prepare QEMU executable: %v", err)
	}

	process, err := exec.NewProcessFromExecutable(e, qd.dopts.ExecOptions...)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could not prepare QEMU process: %v", err)
	}

	mcfg.CreatedAt = time.Now()

	// Start and also wait for the process to quit as we have invoked
	// daemonization of the process.  When it exits, we'll have a PID we can use
	// to manipulate the VMM.
	if err := process.StartAndWait(ctx); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not start and wait for QEMU process: %v", err)
	}

	defer func() {
		if err != nil {
			if dErr := qd.Destroy(ctx, mid); dErr != nil {
				err = fmt.Errorf("%w. Additionally, while destroying machine: %w", err, dErr)
			}
		}
	}()

	if err = qd.dopts.Store.SaveMachineConfig(mid, *mcfg); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not save machine config: %v", err)
	}

	if err = qd.dopts.Store.SaveDriverConfig(mid, *qcfg); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not save driver config: %v", err)
	}

	if err = qd.dopts.Store.SaveMachineState(mid, machine.MachineStateCreated); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not save machine state: %v", err)
	}

	return mid, nil
}

func (qd *QemuDriver) Config(ctx context.Context, mid machine.MachineID) (*QemuConfig, error) {
	dcfg := &QemuConfig{}

	if err := qd.dopts.Store.LookupDriverConfig(mid, dcfg); err != nil {
		return nil, err
	}

	return dcfg, nil
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

func (qd *QemuDriver) QMPClient(ctx context.Context, mid machine.MachineID) (*qmpv1alpha.QEMUMachineProtocolClient, error) {
	qcfg, err := qd.Config(ctx, mid)
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

func (qd *QemuDriver) Pid(ctx context.Context, mid machine.MachineID) (uint32, error) {
	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return 0, err
	}

	pidData, err := os.ReadFile(qcfg.PidFile)
	if err != nil {
		return 0, fmt.Errorf("could not read pid file: %v", err)
	}

	pid, err := strconv.ParseUint(strings.TrimSpace(string(pidData)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not convert pid string \"%s\" to uint64: %v", pidData, err)
	}

	return uint32(pid), nil
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

func (qd *QemuDriver) ListenStatusUpdate(ctx context.Context, mid machine.MachineID) (chan machine.MachineState, chan error, error) {
	events := make(chan machine.MachineState)
	errs := make(chan error)

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return nil, nil, err
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
			state, err := qd.State(ctx, mid)
			if err != nil {
				errs <- err
				continue
			}

			// Initialize with the current state
			if firstCall {
				events <- state
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
				events <- machine.MachineStatePaused

			case qmpv1alpha.EVENT_RESUME:
				events <- machine.MachineStateRunning

			case qmpv1alpha.EVENT_RESET, qmpv1alpha.EVENT_WAKEUP:
				events <- machine.MachineStateRestarting

			case qmpv1alpha.EVENT_SHUTDOWN:
				events <- machine.MachineStateExited

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

func (qd *QemuDriver) AddBridge() {}

func (qd *QemuDriver) Start(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return fmt.Errorf("could not start qemu instance: %v", err)
	}

	defer qmpClient.Close()
	_, err = qmpClient.Cont(qmpv1alpha.ContRequest{})
	if err != nil {
		return err
	}

	// TODO: Timeout? Unikernels boot quickly, but a user environment may be
	// saturated...

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return err
	}

	// Check if the process is alive
	process, err := processFromPidFile(qcfg.PidFile)
	if err != nil {
		return err
	}

	isRunning, err := process.IsRunning()
	if err != nil {
		return err
	}

	if isRunning {
		if err := qd.dopts.Store.SaveMachineState(mid, machine.MachineStateRunning); err != nil {
			return err
		}
	}

	return err
}

func (qd *QemuDriver) exitStatusAndAtFromConfig(mid machine.MachineID) (exitStatus int, exitedAt time.Time, err error) {
	exitStatus = -1 // return -1 if the process hasn't started
	exitedAt = time.Time{}

	var mcfg machine.MachineConfig
	if err := qd.dopts.Store.LookupMachineConfig(mid, &mcfg); err != nil {
		return exitStatus, exitedAt, fmt.Errorf("could not look up machine config: %v", err)
	}

	exitStatus = mcfg.ExitStatus
	exitedAt = mcfg.ExitedAt

	return
}

func (qd *QemuDriver) Wait(ctx context.Context, mid machine.MachineID) (exitStatus int, exitedAt time.Time, err error) {
	exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(mid)
	if err != nil {
		return
	}

	events, errs, err := qd.ListenStatusUpdate(ctx, mid)
	if err != nil {
		return
	}

	for {
		select {
		case state := <-events:
			exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(mid)

			switch state {
			case machine.MachineStateExited, machine.MachineStateDead:
				return
			}

		case err2 := <-errs:
			exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(mid)

			if errors.Is(err2, qmp.ErrAcceptedNonEvent) {
				continue
			}

			return

		case <-ctx.Done():
			exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(mid)

			// TODO: Should we return an error if the context is cancelled?
			return
		}
	}
}

func (qd *QemuDriver) StartAndWait(ctx context.Context, mid machine.MachineID) (int, time.Time, error) {
	if err := qd.Start(ctx, mid); err != nil {
		// return -1 if the process hasn't started.
		return -1, time.Time{}, err
	}

	return qd.Wait(ctx, mid)
}

func (qd *QemuDriver) Pause(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return fmt.Errorf("could not start qemu instance: %v", err)
	}

	defer qmpClient.Close()

	_, err = qmpClient.Stop(qmpv1alpha.StopRequest{})
	if err != nil {
		return err
	}

	return qd.dopts.Store.SaveMachineState(mid, machine.MachineStatePaused)
}

func (qd *QemuDriver) TailWriter(ctx context.Context, mid machine.MachineID, writer io.Writer) error {
	var mcfg machine.MachineConfig
	if err := qd.dopts.Store.LookupMachineConfig(mid, &mcfg); err != nil {
		return fmt.Errorf("could not look up machine config: %v", err)
	}

	f, err := os.Open(mcfg.LogFile)
	if err != nil {
		return err
	}

	// var offset int64
	reader := bufio.NewReaderSize(f, DefaultTailBufferSize)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(mcfg.LogFile); err != nil {
		return err
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
			return err
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
				return err
			}

			reader.Reset(f)
			break
		}

		fmt.Fprintf(writer, "%s", s[discarded:])
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
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
					return err
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
						return err
					}

					reader.Reset(f)
					continue
				}

				fmt.Fprintf(writer, "%s", s[discarded:])
			}
		}
	}
}

func (qd *QemuDriver) State(ctx context.Context, mid machine.MachineID) (state machine.MachineState, err error) {
	state = machine.MachineStateUnknown

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return
	}

	state, err = qd.dopts.Store.LookupMachineState(mid)
	if err != nil {
		return
	}

	savedState := state

	var mcfg machine.MachineConfig
	if err := qd.dopts.Store.LookupMachineConfig(mid, &mcfg); err != nil {
		return state, fmt.Errorf("could not look up machine config: %v", err)
	}

	// Check if the process is alive, which ultimately indicates to us whether we
	// able to speak to the exposed QMP socket
	activeProcess := false
	if process, err := processFromPidFile(qcfg.PidFile); err == nil {
		activeProcess, err = process.IsRunning()
		if err != nil {
			state = machine.MachineStateDead
			activeProcess = false
		}
	}

	exitedAt := mcfg.ExitedAt
	exitStatus := mcfg.ExitStatus

	defer func() {
		if exitStatus >= 0 && mcfg.ExitedAt.IsZero() {
			exitedAt = time.Now()
		}

		// Update the machine config with the latest values if they are different from
		// what we have on record
		if mcfg.ExitedAt != exitedAt || mcfg.ExitStatus != exitStatus {
			mcfg.ExitedAt = exitedAt
			mcfg.ExitStatus = exitStatus
			if err = qd.dopts.Store.SaveMachineConfig(mid, mcfg); err != nil {
				return
			}
		}

		// Finally, save the state if it is different from the what we have on record
		if state != savedState {
			if err = qd.dopts.Store.SaveMachineState(mid, state); err != nil {
				return
			}
		}
	}()

	if !activeProcess {
		if savedState == machine.MachineStateRunning {
			state = machine.MachineStateDead
			exitStatus = 1
		}
		return
	}

	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		state = machine.MachineStateDead
		exitStatus = 1
		return
	} else if err != nil {
		return state, fmt.Errorf("could not attach to QMP client: %v", err)
	}

	defer qmpClient.Close()

	// Grab the actual state of the machine by querying QMP
	status, err := qmpClient.QueryStatus(qmpv1alpha.QueryStatusRequest{})
	if err != nil {
		// We cannot amend the status at this point, even if the process is
		// alive, since it is not an indicator of the state of the VM, only of the
		// VMM.  So we return what we already know via LookupMachineConfig.
		return state, fmt.Errorf("could not query machine status via QMP: %v", err)
	}

	// Map the QMP status to supported machine states
	switch status.Return.Status {
	case qmpv1alpha.RUN_STATE_GUEST_PANICKED, qmpv1alpha.RUN_STATE_INTERNAL_ERROR, qmpv1alpha.RUN_STATE_IO_ERROR:
		state = machine.MachineStateDead
		exitStatus = 1

	case qmpv1alpha.RUN_STATE_PAUSED:
		state = machine.MachineStatePaused
		exitStatus = -1

	case qmpv1alpha.RUN_STATE_RUNNING:
		state = machine.MachineStateRunning
		exitStatus = -1

	case qmpv1alpha.RUN_STATE_SHUTDOWN:
		state = machine.MachineStateExited
		exitStatus = 0

	case qmpv1alpha.RUN_STATE_SUSPENDED:
		state = machine.MachineStateSuspended
		exitStatus = -1

	default:
		// qmpv1alpha.RUN_STATE_SAVE_VM,
		// qmpv1alpha.RUN_STATE_PRELAUNCH,
		// qmpv1alpha.RUN_STATE_RESTORE_VM,
		// qmpv1alpha.RUN_STATE_WATCHDOG,
		state = machine.MachineStateUnknown
		exitStatus = -1
	}

	return
}

func (qd *QemuDriver) List(ctx context.Context) ([]machine.MachineID, error) {
	var mids []machine.MachineID

	midmap, err := qd.dopts.Store.ListAllMachineConfigs()
	if err != nil {
		return nil, err
	}

	for mid, mcfg := range midmap {
		if mcfg.DriverName == "qemu" {
			mids = append(mids, mid)
		}
	}

	return mids, nil
}

func (qd *QemuDriver) Stop(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return err
	}

	defer qmpClient.Close()
	_, err = qmpClient.Quit(qmpv1alpha.QuitRequest{})
	if err != nil {
		return err
	}

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return err
	}

	if err := retrytimeout.RetryTimeout(5*time.Second, func() error {
		if _, err := os.ReadFile(qcfg.PidFile); !os.IsNotExist(err) {
			return fmt.Errorf("process still active")
		}

		return nil
	}); err != nil {
		return err
	}

	return qd.dopts.Store.SaveMachineState(mid, machine.MachineStateExited)
}

func (qd *QemuDriver) Destroy(ctx context.Context, mid machine.MachineID) error {
	state, err := qd.dopts.Store.LookupMachineState(mid)
	if err != nil {
		return err
	}

	switch state {
	case machine.MachineStateUnknown,
		machine.MachineStateExited,
		machine.MachineStateDead:
	default:
		if err := qd.Stop(ctx, mid); err != nil {
			return err
		}
	}

	return qd.dopts.Store.Purge(mid)
}

func (qd *QemuDriver) Shutdown(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return err
	}

	defer qmpClient.Close()
	_, err = qmpClient.SystemPowerdown(qmpv1alpha.SystemPowerdownRequest{})
	if err != nil {
		return err
	}

	return nil
}
