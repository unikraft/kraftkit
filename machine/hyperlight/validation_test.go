// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package hyperlight

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
)

func TestValidateHyperlightRuntimeConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *HyperlightConfig
		wantErr    bool
		errContain string
		wantAllow  []string
		wantPorts  []int32
	}{
		{
			name: "nil config",
			cfg:  nil,
		},
		{
			name: "valid stack",
			cfg:  &HyperlightConfig{Stack: "8Mi"},
		},
		{
			name:       "invalid stack quantity",
			cfg:        &HyperlightConfig{Stack: "not-memory"},
			wantErr:    true,
			errContain: "stack quantity",
		},
		{
			name:       "non-positive stack",
			cfg:        &HyperlightConfig{Stack: "0Mi"},
			wantErr:    true,
			errContain: "greater than 0",
		},
		{
			name:       "negative repeat",
			cfg:        &HyperlightConfig{Repeat: -1},
			wantErr:    true,
			errContain: "non-negative",
		},
		{
			name:       "allow and block both set",
			cfg:        &HyperlightConfig{NetAllow: []string{"a"}, NetBlock: []string{"b"}},
			wantErr:    true,
			errContain: "cannot both be set",
		},
		{
			name:      "allow list trims and dedupes",
			cfg:       &HyperlightConfig{NetAllow: []string{" a ", "a", "b"}},
			wantAllow: []string{"a", "b"},
		},
		{
			name:       "empty allow entry",
			cfg:        &HyperlightConfig{NetAllow: []string{"  "}},
			wantErr:    true,
			errContain: "cannot be empty",
		},
		{
			name:       "port out of range",
			cfg:        &HyperlightConfig{Ports: []int32{0}},
			wantErr:    true,
			errContain: "outside the valid range",
		},
		{
			name:       "duplicate port",
			cfg:        &HyperlightConfig{Ports: []int32{8080, 8080}},
			wantErr:    true,
			errContain: "duplicated",
		},
		{
			name:      "valid ports",
			cfg:       &HyperlightConfig{Ports: []int32{80, 443}},
			wantPorts: []int32{80, 443},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHyperlightRuntimeConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateHyperlightRuntimeConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error = %v, want substring %q", err, tt.errContain)
				}
			}
			if tt.cfg != nil && tt.wantAllow != nil {
				if !reflect.DeepEqual(tt.cfg.NetAllow, tt.wantAllow) {
					t.Errorf("NetAllow = %v, want %v", tt.cfg.NetAllow, tt.wantAllow)
				}
			}
			if tt.cfg != nil && tt.wantPorts != nil {
				if !reflect.DeepEqual(tt.cfg.Ports, tt.wantPorts) {
					t.Errorf("Ports = %v, want %v", tt.cfg.Ports, tt.wantPorts)
				}
			}
		})
	}
}

func TestNormalizeHyperlightGuestMountPath(t *testing.T) {
	tests := []struct {
		name       string
		guestPath  string
		want       string
		wantErr    bool
		errContain string
	}{
		{
			name:       "empty path",
			guestPath:  "",
			wantErr:    true,
			errContain: "cannot be empty",
		},
		{
			name:       "relative path",
			guestPath:  "data",
			wantErr:    true,
			errContain: "must be absolute",
		},
		{
			name:       "reserved root",
			guestPath:  "/",
			wantErr:    true,
			errContain: "reserved guest directory",
		},
		{
			name:       "reserved exact bin",
			guestPath:  "/bin",
			wantErr:    true,
			errContain: "reserved guest directory",
		},
		{
			name:       "shadows reserved usr",
			guestPath:  "/usr/lib",
			wantErr:    true,
			errContain: "reserved guest directory",
		},
		{
			name:      "safe mount path",
			guestPath: "/mnt/data",
			want:      "/mnt/data",
		},
		{
			name:      "cleans dot segments",
			guestPath: "/mnt/data/.",
			want:      "/mnt/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeHyperlightGuestMountPath(tt.guestPath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeHyperlightGuestMountPath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error = %v, want substring %q", err, tt.errContain)
				}
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeHyperlightMountSpec(t *testing.T) {
	hostDir := t.TempDir()
	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	absHost, err := filepath.Abs(hostDir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		spec       string
		wantMount  string
		wantGuest  string
		wantErr    bool
		errContain string
	}{
		{
			name:       "empty spec",
			spec:       "",
			wantErr:    true,
			errContain: "cannot be empty",
		},
		{
			name:       "empty host",
			spec:       ":/guest",
			wantErr:    true,
			errContain: "source cannot be empty",
		},
		{
			name:       "empty guest",
			spec:       hostDir + ":",
			wantErr:    true,
			errContain: "destination cannot be empty",
		},
		{
			name:      "host only defaults guest to /host",
			spec:      hostDir,
			wantMount: absHost + ":/host",
			wantGuest: "/host",
		},
		{
			name:       "host path is file",
			spec:       filePath + ":/data",
			wantErr:    true,
			errContain: "not a directory",
		},
		{
			name:      "host and guest",
			spec:      hostDir + ":/mnt/data",
			wantMount: absHost + ":/mnt/data",
			wantGuest: "/mnt/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMount, gotGuest, err := normalizeHyperlightMountSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeHyperlightMountSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error = %v, want substring %q", err, tt.errContain)
				}
			}
			if !tt.wantErr {
				if gotMount != tt.wantMount {
					t.Errorf("mount = %q, want %q", gotMount, tt.wantMount)
				}
				if gotGuest != tt.wantGuest {
					t.Errorf("guest = %q, want %q", gotGuest, tt.wantGuest)
				}
			}
		})
	}
}

func TestValidateExistingFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "kernel")
	if err := os.WriteFile(file, []byte("kernel"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		path       string
		wantErr    bool
		errContain string
	}{
		{
			name:       "missing file",
			path:       filepath.Join(dir, "missing"),
			wantErr:    true,
			errContain: "not accessible",
		},
		{
			name:       "directory path",
			path:       dir,
			wantErr:    true,
			errContain: "is a directory",
		},
		{
			name: "valid file",
			path: file,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExistingFile(tt.path, "hyperlight kernel")
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateExistingFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error = %v, want substring %q", err, tt.errContain)
				}
			}
		})
	}
}

func TestHyperlightPortsFromMachine(t *testing.T) {
	tests := []struct {
		name       string
		ports      machinev1alpha1.MachinePorts
		want       []int32
		wantErr    bool
		errContain string
	}{
		{
			name: "empty ports",
		},
		{
			name: "valid same-port mapping",
			ports: machinev1alpha1.MachinePorts{
				{HostPort: 8080, MachinePort: 8080},
			},
			want: []int32{8080},
		},
		{
			name: "host ip 0.0.0.0 is allowed",
			ports: machinev1alpha1.MachinePorts{
				{HostIP: "0.0.0.0", HostPort: 443, MachinePort: 443},
			},
			want: []int32{443},
		},
		{
			name: "host ip binding rejected",
			ports: machinev1alpha1.MachinePorts{
				{HostIP: "127.0.0.1", HostPort: 8080, MachinePort: 8080},
			},
			wantErr:    true,
			errContain: "host IP binding",
		},
		{
			name: "port forwarding rejected",
			ports: machinev1alpha1.MachinePorts{
				{HostPort: 8080, MachinePort: 80},
			},
			wantErr:    true,
			errContain: "host port forwarding",
		},
		{
			name: "udp protocol rejected",
			ports: machinev1alpha1.MachinePorts{
				{HostPort: 53, MachinePort: 53, Protocol: corev1.ProtocolUDP},
			},
			wantErr:    true,
			errContain: "only supports TCP",
		},
		{
			name: "tcp protocol allowed",
			ports: machinev1alpha1.MachinePorts{
				{HostPort: 80, MachinePort: 80, Protocol: corev1.ProtocolTCP},
			},
			want: []int32{80},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := &machinev1alpha1.Machine{
				Spec: machinev1alpha1.MachineSpec{Ports: tt.ports},
			}
			got, err := hyperlightPortsFromMachine(machine)
			if (err != nil) != tt.wantErr {
				t.Fatalf("hyperlightPortsFromMachine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error = %v, want substring %q", err, tt.errContain)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ports = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHyperlightMountsFromMachine(t *testing.T) {
	hostDir := t.TempDir()
	filePath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	absHost, err := filepath.Abs(hostDir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		volumes    []volumev1alpha1.Volume
		initrdPath string
		want       []string
		wantErr    bool
		errContain string
	}{
		{
			name: "9pfs writable directory",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{
					Driver:      "9pfs",
					Source:      hostDir,
					Destination: "/mnt/data",
				}},
			},
			want: []string{absHost + ":/mnt/data"},
		},
		{
			name: "unsupported driver",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{Driver: "block", Source: hostDir, Destination: "/data"}},
			},
			wantErr:    true,
			errContain: "does not support KraftKit volume driver",
		},
		{
			name: "read-only volume rejected",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{
					Driver: "9pfs", Source: hostDir, Destination: "/mnt/data", ReadOnly: true,
				}},
			},
			wantErr:    true,
			errContain: "read-only",
		},
		{
			name: "empty source rejected",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{Driver: "9pfs", Destination: "/mnt/data"}},
			},
			wantErr:    true,
			errContain: "source cannot be empty",
		},
		{
			name: "source file rejected",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{
					Driver: "9pfs", Source: filePath, Destination: "/mnt/data",
				}},
			},
			wantErr:    true,
			errContain: "not a directory",
		},
		{
			name: "initrd at root skipped when initrd already set",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{Driver: "initrd", Destination: "/"}},
			},
			initrdPath: "/existing/initrd.cpio",
		},
		{
			name: "initrd at root without path rejected",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{Driver: "initrd", Destination: "/"}},
			},
			wantErr:    true,
			errContain: "requires an initrd path",
		},
		{
			name: "initrd at non-root rejected",
			volumes: []volumev1alpha1.Volume{
				{Spec: volumev1alpha1.VolumeSpec{Driver: "initrd", Destination: "/data"}},
			},
			wantErr:    true,
			errContain: "does not support KraftKit initrd volumes mounted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := &machinev1alpha1.Machine{
				Status: machinev1alpha1.MachineStatus{InitrdPath: tt.initrdPath},
				Spec:   machinev1alpha1.MachineSpec{Volumes: tt.volumes},
			}
			got, err := hyperlightMountsFromMachine(machine, map[string]struct{}{})
			if (err != nil) != tt.wantErr {
				t.Fatalf("hyperlightMountsFromMachine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error = %v, want substring %q", err, tt.errContain)
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("mounts = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mounts[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestHyperlightMountsFromSpecs(t *testing.T) {
	hostDir := t.TempDir()
	absHost, err := filepath.Abs(hostDir)
	if err != nil {
		t.Fatal(err)
	}

	guestPaths := map[string]struct{}{"/mnt/data": {}}

	tests := []struct {
		name       string
		specs      []string
		guestPaths map[string]struct{}
		want       []string
		wantErr    bool
		errContain string
	}{
		{
			name:  "valid mount spec",
			specs: []string{hostDir + ":/mnt/other"},
			want:  []string{absHost + ":/mnt/other"},
		},
		{
			name:       "duplicate guest path",
			specs:      []string{hostDir + ":/mnt/data"},
			guestPaths: guestPaths,
			wantErr:    true,
			errContain: "duplicated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := tt.guestPaths
			if paths == nil {
				paths = map[string]struct{}{}
			}
			got, err := hyperlightMountsFromSpecs(tt.specs, paths)
			if (err != nil) != tt.wantErr {
				t.Fatalf("hyperlightMountsFromSpecs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContain != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContain) {
					t.Fatalf("error = %v, want substring %q", err, tt.errContain)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mounts = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetHyperlightConfigFromPlatformConfig(t *testing.T) {
	original := HyperlightConfig{
		Memory:     "64Mi",
		Stack:      "16Mi",
		KernelPath: "/kernel",
		Ports:      []int32{8080},
		Quiet:      true,
		Net:        true,
	}

	t.Run("nil config", func(t *testing.T) {
		cfg, err := getHyperlightConfigFromPlatformConfig(nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*cfg, HyperlightConfig{}) {
			t.Errorf("got %#v, want empty config", *cfg)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		in := original
		cfg, err := getHyperlightConfigFromPlatformConfig(&in)
		if err != nil {
			t.Fatal(err)
		}
		if cfg != &in {
			t.Fatal("expected same pointer")
		}
	})

	t.Run("value", func(t *testing.T) {
		cfg, err := getHyperlightConfigFromPlatformConfig(original)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*cfg, original) {
			t.Errorf("got %#v, want %#v", *cfg, original)
		}
	})

	t.Run("gob round trip", func(t *testing.T) {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(original); err != nil {
			t.Fatal(err)
		}

		var decoded HyperlightConfig
		if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
			t.Fatal(err)
		}

		cfg, err := getHyperlightConfigFromPlatformConfig(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*cfg, original) {
			t.Errorf("got %#v, want %#v", *cfg, original)
		}
	})
}
