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

package qemu

import "strings"

type QemuConfig struct {
	// Command-line arguments for qemu-system-*
	Append     string            `flag:"-append"      json:"append,omitempty"`
	CharDevs   []QemuCharDev     `flag:"-chardev"     json:"chardev,omitempty"`
	CPU        QemuCPU           `flag:"-cpu"         json:"cpu,omitempty"`
	Daemonize  bool              `flag:"-daemonize"   json:"daemonize,omitempty"`
	Devices    []QemuDevice      `flag:"-device"      json:"device,omitempty"`
	Display    QemuDisplay       `flag:"-display"     json:"display,omitempty"`
	EnableKVM  bool              `flag:"-enable-kvm"  json:"enable_kvm,omitempty"`
	InitRd     string            `flag:"-initrd"      json:"initrd,omitempty"`
	Kernel     string            `flag:"-kernel"      json:"kernel,omitempty"`
	Machine    QemuMachine       `flag:"-machine"     json:"machine,omitempty"`
	Memory     QemuMemory        `flag:"-m"           json:"memory,omitempty"`
	Monitor    QemuHostCharDev   `flag:"-monitor"     json:"monitor,omitempty"`
	Name       string            `flag:"-name"        json:"name,omitempty"`
	NoACPI     bool              `flag:"-no-acpi"     json:"no_acpi,omitempty"`
	NoDefaults bool              `flag:"-nodefaults"  json:"no_defaults,omitempty"`
	NoGraphic  bool              `flag:"-nographic"   json:"no_graphic,omitempty"`
	NoReboot   bool              `flag:"-no-reboot"   json:"no_reboot,omitempty"`
	NoShutdown bool              `flag:"-no-shutdown" json:"no_shutdown,omitempty"`
	NoStart    bool              `flag:"-S"           json:"no_start,omitempty"`
	Parallel   QemuHostCharDev   `flag:"-parallel"    json:"parallel,omitempty"`
	PidFile    string            `flag:"-pidfile"     json:"pidfile,omitempty"`
	QMP        []QemuHostCharDev `flag:"-qmp"         json:"qmp,omitempty"`
	RTC        QemuRTC           `flag:"-rtc"         json:"rtc,omitempty"`
	Serial     QemuHostCharDev   `flag:"-serial"      json:"serial,omitempty"`
	SMP        QemuSMP           `flag:"-smp"         json:"smp,omitempty"`
	TBSize     int               `flag:"-tb-size"     json:"tb_size,omitempty"`
	VGA        QemuVGA           `flag:"-vga"         json:"vga,omitempty"`

	// Command-line arguments for qemu-system-i386 and qemu-system-x86_64 only
	NoHPET bool `flag:"-no-hpet" json:"no_hpet,omitempty"`
}

type QemuOption func(*QemuConfig) error

func NewQemuConfig(qopts ...QemuOption) (*QemuConfig, error) {
	qcfg := QemuConfig{}

	for _, o := range qopts {
		if err := o(&qcfg); err != nil {
			return nil, err
		}
	}

	return &qcfg, nil
}

func WithAppend(append ...string) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Append = qc.Append + " " + strings.Join(append, " ")
		return nil
	}
}

func WithCharDevice(chardev QemuCharDev) QemuOption {
	return func(qc *QemuConfig) error {
		if qc.CharDevs == nil {
			qc.CharDevs = make([]QemuCharDev, 0)
		}

		qc.CharDevs = append(qc.CharDevs, chardev)

		return nil
	}
}

func WithCPU(cpu QemuCPU) QemuOption {
	return func(qc *QemuConfig) error {
		qc.CPU = cpu
		return nil
	}
}

func WithDaemonize(daemonize bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Daemonize = daemonize
		return nil
	}
}

func WithDevice(device QemuDevice) QemuOption {
	return func(qc *QemuConfig) error {
		if qc.Devices == nil {
			qc.Devices = make([]QemuDevice, 0)
		}

		qc.Devices = append(qc.Devices, device)

		return nil
	}
}

func WithDisplay(display QemuDisplay) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Display = display
		return nil
	}
}

func WithEnableKVM(enableKVM bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.EnableKVM = enableKVM
		return nil
	}
}

func WithInitRd(initrd string) QemuOption {
	return func(qc *QemuConfig) error {
		qc.InitRd = initrd
		return nil
	}
}

func WithKernel(kernel string) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Kernel = kernel
		return nil
	}
}

func WithMachine(machine QemuMachine) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Machine = machine
		return nil
	}
}

func WithMemory(memory QemuMemory) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Memory = memory
		return nil
	}
}

func WithMonitor(chardev QemuHostCharDev) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Monitor = chardev
		return nil
	}
}

func WithName(name string) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Name = name
		return nil
	}
}

func WithNoACPI(noACPI bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.NoACPI = noACPI
		return nil
	}
}

func WithNoDefaults(noDefaults bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.NoDefaults = noDefaults
		return nil
	}
}

func WithNoGraphic(noGraphic bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.NoGraphic = noGraphic
		return nil
	}
}

func WithNoReboot(noReboot bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.NoReboot = noReboot
		return nil
	}
}

func WithNoShutdown(noShutdown bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.NoShutdown = noShutdown
		return nil
	}
}

func WithNoStart(noStart bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.NoStart = noStart
		return nil
	}
}

func WithParallel(chardev QemuHostCharDev) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Parallel = chardev
		return nil
	}
}

func WithPidFile(pidFile string) QemuOption {
	return func(qc *QemuConfig) error {
		qc.PidFile = pidFile
		return nil
	}
}

// WithQMP is similar to WithMonitor but opens in "control" mode.
func WithQMP(qmp QemuHostCharDev) QemuOption {
	return func(qc *QemuConfig) error {
		if qc.QMP == nil {
			qc.QMP = make([]QemuHostCharDev, 0)
		}

		qc.QMP = append(qc.QMP, qmp)

		return nil
	}
}

func WithRTC(rtc QemuRTC) QemuOption {
	return func(qc *QemuConfig) error {
		qc.RTC = rtc
		return nil
	}
}

func WithSerial(chardev QemuHostCharDev) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Serial = chardev
		return nil
	}
}

func WithSMP(smp QemuSMP) QemuOption {
	return func(qc *QemuConfig) error {
		qc.SMP = smp
		return nil
	}
}

func WithTBSize(tbSize int) QemuOption {
	return func(qc *QemuConfig) error {
		qc.TBSize = tbSize
		return nil
	}
}

func WithVGA(vga QemuVGA) QemuOption {
	return func(qc *QemuConfig) error {
		qc.VGA = vga
		return nil
	}
}

func WithNoHPET(noHPET bool) QemuOption {
	return func(qc *QemuConfig) error {
		qc.NoHPET = noHPET
		return nil
	}
}
