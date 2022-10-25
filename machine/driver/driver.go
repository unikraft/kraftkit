// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package driver

import (
	"context"
	"fmt"
	"io"
	"time"

	"kraftkit.sh/machine"
	"kraftkit.sh/machine/driveropts"
	"kraftkit.sh/machine/qemu"
	"kraftkit.sh/utils"
)

type DriverType string

const (
	// UnknownDriver driver is used to indicate an unknown or misspecified driver
	UnknownDriver = DriverType("unknown")

	// QemuDriver is the QEMU hypervisor
	QemuDriver = DriverType("qemu")
)

func (dt DriverType) String() string {
	return string(dt)
}

// DriverNameToType
func DriverNames() []string {
	return []string{
		string(QemuDriver),
	}
}

func DriverTypeFromName(name string) DriverType {
	if utils.Contains(DriverNames(), name) {
		return DriverType(name)
	}

	return UnknownDriver
}

// Driver represents the interface necessary to be implemented to manage the
// lifcycle of a machine.
type Driver interface {
	// Create a machine using this driver with the defined `MachineOption`s.
	Create(context.Context, ...machine.MachineOption) (machine.MachineID, error)

	// Start requests the machine to begin its execution if paused.
	Start(context.Context, machine.MachineID) error

	// Stop requests the machine to stop its execution if running.
	Stop(context.Context, machine.MachineID) error

	// Wait for the machine to complete its execution if running.
	Wait(context.Context, machine.MachineID) (int, time.Time, error)

	// StartAndWait starts the machine and then waits for the machine to exit
	// before returning.
	StartAndWait(context.Context, machine.MachineID) (int, time.Time, error)

	// Pid returns the process ID of the machine VMM
	Pid(ctx context.Context, mid machine.MachineID) (uint32, error)

	// Pause a machine given its MachineID.
	Pause(context.Context, machine.MachineID) error

	// Destroy a machine given its MachineID.
	Destroy(context.Context, machine.MachineID) error

	// Tail the serial console of the machine by providing.
	TailWriter(context.Context, machine.MachineID, io.Writer) error

	// List all machines supervised by the current driver.
	List(context.Context) ([]machine.MachineID, error)

	// State returns the machine state given a MachineID.
	State(context.Context, machine.MachineID) (machine.MachineState, error)

	// Shutdown sends a shutdown signal to the machine given a MachineID.
	Shutdown(context.Context, machine.MachineID) error

	// ListenStatusUpdate returns two channels, one for receiving the state of a
	// machine and any live errors.  This can be used to monitor a given machine
	// by its MachineID.  The method returns the last error if the channels cannot
	// be initialized.
	ListenStatusUpdate(context.Context, machine.MachineID) (chan machine.MachineState, chan error, error)
}

// New creates an instantiated driver which can create and manage the lifecycle
// of a machine.  The returning interface is implemented by the driver.
func New(driverType DriverType, opts ...driveropts.DriverOption) (driver Driver, err error) {
	switch driverType {
	case QemuDriver:
		driver, err = qemu.NewQemuDriver(opts...)
	default:
		return nil, fmt.Errorf("unknown machine driver: %s", driverType.String())
	}

	return
}
