// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import "strings"

type QemuConfig struct {
	// Command-line arguments for qemu-system-*
	Accel      QemuMachineAccelerator `flag:"-accel"       json:"accel,omitempty"`
	Append     string                 `flag:"-append"      json:"append,omitempty"`
	CharDevs   []QemuCharDev          `flag:"-chardev"     json:"chardev,omitempty"`
	CPU        QemuCPU                `flag:"-cpu"         json:"cpu,omitempty"`
	Daemonize  bool                   `flag:"-daemonize"   json:"daemonize,omitempty"`
	Devices    []QemuDevice           `flag:"-device"      json:"device,omitempty"`
	Display    QemuDisplay            `flag:"-display"     json:"display,omitempty"`
	EnableKVM  bool                   `flag:"-enable-kvm"  json:"enable_kvm,omitempty"`
	FsDevs     []QemuFsDev            `flag:"-fsdev"       json:"fsdev,omitempty"`
	InitRd     string                 `flag:"-initrd"      json:"initrd,omitempty"`
	Kernel     string                 `flag:"-kernel"      json:"kernel,omitempty"`
	Machine    QemuMachine            `flag:"-machine"     json:"machine,omitempty"`
	Memory     QemuMemory             `flag:"-m"           json:"memory,omitempty"`
	Monitor    QemuHostCharDev        `flag:"-monitor"     json:"monitor,omitempty"`
	Name       string                 `flag:"-name"        json:"name,omitempty"`
	NetDevs    []QemuNetDev           `flag:"-netdev"      json:"netdev,omitempty"`
	NoACPI     bool                   `flag:"-no-acpi"     json:"no_acpi,omitempty"`
	NoDefaults bool                   `flag:"-nodefaults"  json:"no_defaults,omitempty"`
	NoGraphic  bool                   `flag:"-nographic"   json:"no_graphic,omitempty"`
	NoReboot   bool                   `flag:"-no-reboot"   json:"no_reboot,omitempty"`
	NoShutdown bool                   `flag:"-no-shutdown" json:"no_shutdown,omitempty"`
	NoStart    bool                   `flag:"-S"           json:"no_start,omitempty"`
	Parallel   QemuHostCharDev        `flag:"-parallel"    json:"parallel,omitempty"`
	PidFile    string                 `flag:"-pidfile"     json:"pidfile,omitempty"`
	QMP        []QemuHostCharDev      `flag:"-qmp"         json:"qmp,omitempty"`
	RTC        QemuRTC                `flag:"-rtc"         json:"rtc,omitempty"`
	Serial     []QemuHostCharDev      `flag:"-serial"      json:"serial,omitempty"`
	SMP        QemuSMP                `flag:"-smp"         json:"smp,omitempty"`
	TBSize     int                    `flag:"-tb-size"     json:"tb_size,omitempty"`
	VGA        QemuVGA                `flag:"-vga"         json:"vga,omitempty"`
	Version    bool                   `flag:"-version"     json:"-"`

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

func WithAccel(accel QemuMachineAccelerator) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Accel = accel
		return nil
	}
}

func WithAppend(append ...string) QemuOption {
	return func(qc *QemuConfig) error {
		qc.Append = strings.Join(append, " ")
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

func WithFsDevice(fsdev QemuFsDev) QemuOption {
	return func(qc *QemuConfig) error {
		if qc.FsDevs == nil {
			qc.FsDevs = make([]QemuFsDev, 0)
		}

		qc.FsDevs = append(qc.FsDevs, fsdev)

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

func WithNetDevice(netdev QemuNetDev) QemuOption {
	return func(qc *QemuConfig) error {
		if qc.NetDevs == nil {
			qc.NetDevs = make([]QemuNetDev, 0)
		}

		qc.NetDevs = append(qc.NetDevs, netdev)

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
		if qc.Serial == nil {
			qc.Serial = make([]QemuHostCharDev, 0)
		}

		qc.Serial = append(qc.Serial, chardev)
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
