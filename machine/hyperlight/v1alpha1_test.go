// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/config"
)

func touchFile(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withStubBinary(t *testing.T, name string) {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, name)
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func testCtx(t *testing.T) context.Context {
	t.Helper()

	runtimeDir := t.TempDir()
	return config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{RuntimeDir: runtimeDir},
	})
}

func validMachine(t *testing.T) (*machinev1alpha1.Machine, string, string) {
	t.Helper()

	workdir := t.TempDir()
	kernelPath := filepath.Join(workdir, "kernel")
	initrdPath := filepath.Join(workdir, "initrd.cpio")
	volumeDir := filepath.Join(workdir, "volume")
	if err := os.MkdirAll(volumeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	touchFile(t, kernelPath)
	touchFile(t, initrdPath)

	memory, err := resource.ParseQuantity("512Mi")
	if err != nil {
		t.Fatal(err)
	}

	machine := &machinev1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: machinev1alpha1.MachineSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: memory,
				},
			},
			Ports: machinev1alpha1.MachinePorts{
				{HostPort: 8080, MachinePort: 8080},
			},
			Volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{
					Driver:      "9pfs",
					Source:      volumeDir,
					Destination: "/mnt/data",
				}},
			},
			ApplicationArgs: []string{"--verbose"},
		},
		Status: machinev1alpha1.MachineStatus{
			KernelPath: kernelPath,
			InitrdPath: initrdPath,
		},
	}

	return machine, kernelPath, initrdPath
}

func TestCreate_RejectsUnsupportedSpec(t *testing.T) {
	service := machineV1alpha1Service{}

	tests := []struct {
		name       string
		mutate     func(*machinev1alpha1.Machine)
		useStub    bool
		errContain string
	}{
		{
			name: "emulation mode",
			mutate: func(m *machinev1alpha1.Machine) {
				m.Spec.Emulation = true
			},
			useStub:    true,
			errContain: "emulation",
		},
		{
			name: "network attachments",
			mutate: func(m *machinev1alpha1.Machine) {
				m.Spec.Networks = []networkv1alpha1.NetworkSpec{{Driver: "bridge"}}
			},
			useStub:    true,
			errContain: "network attachments",
		},
		{
			name: "environment injection",
			mutate: func(m *machinev1alpha1.Machine) {
				m.Spec.Env = map[string]string{"FOO": "bar"}
			},
			useStub:    true,
			errContain: "environment injection",
		},
		{
			name: "kernel arguments",
			mutate: func(m *machinev1alpha1.Machine) {
				m.Spec.KernelArgs = []string{"console=ttyS0"}
			},
			useStub:    true,
			errContain: "kernel arguments",
		},
		{
			name:       "missing hyperlight-unikraft binary",
			useStub:    false,
			errContain: "not found in $PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.useStub {
				withStubBinary(t, HostBinary)
			} else {
				t.Setenv("PATH", "")
			}

			machine, _, _ := validMachine(t)
			if tt.mutate != nil {
				tt.mutate(machine)
			}

			_, err := service.Create(testCtx(t), machine)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.errContain) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.errContain)
			}
		})
	}
}

func TestCreate_PlatformConfigMarshalArgsRoundTrip(t *testing.T) {
	withStubBinary(t, HostBinary)

	machine, kernelPath, initrdPath := validMachine(t)
	service := machineV1alpha1Service{}

	created, err := service.Create(testCtx(t), machine)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	hlcfg, ok := created.Status.PlatformConfig.(HyperlightConfig)
	if !ok {
		t.Fatalf("PlatformConfig type = %T, want HyperlightConfig", created.Status.PlatformConfig)
	}

	absVolume, err := filepath.Abs(filepath.Join(filepath.Dir(kernelPath), "volume"))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"--memory", "512Mi",
		"--stack", DefaultStack,
		"--port", "8080",
		"--initrd", initrdPath,
		"--mount", absVolume + ":/mnt/data",
		kernelPath,
		"--",
		"--verbose",
	}

	got := hlcfg.MarshalArgs(machine.Spec.ApplicationArgs)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MarshalArgs() = %v, want %v", got, want)
	}
}

func TestStart_ExecAppArgsConflict(t *testing.T) {
	withStubBinary(t, HostBinary)

	machine, _, _ := validMachine(t)
	service := machineV1alpha1Service{}

	created, err := service.Create(testCtx(t), machine)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	hlcfg, ok := created.Status.PlatformConfig.(HyperlightConfig)
	if !ok {
		t.Fatal("expected HyperlightConfig platform config")
	}
	hlcfg.Exec = "print('hi')"
	created.Status.PlatformConfig = hlcfg
	created.Spec.ApplicationArgs = []string{"ignored"}

	updated, err := service.Start(testCtx(t), created)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "conflicts with application arguments") {
		t.Fatalf("error = %q, want exec/app-args conflict", err.Error())
	}
	if updated.Status.State != machinev1alpha1.MachineStateFailed {
		t.Errorf("state = %q, want %q", updated.Status.State, machinev1alpha1.MachineStateFailed)
	}
}
