// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package volume provides the representation of a volume within the context of
// a application project and seeded via a Kraftfile.
package volume

// Volume is a porclained interface which contains information about an
// individual volume that is to be mounted to a unikernel instance at runtime.
type Volume interface {
	// Driver is the name of the implementing strategy.  Volume drivers let you
	// store volumes on remote hosts or cloud providers, to encrypt the contents of
	// volumes, or to add other functionality.
	Driver() string

	// The source of the mount.  For named volumes, this is the name of the
	// volume.  For anonymous volumes, this field is omitted.
	Source() string

	// The destination takes as its value the path where the file or directory is
	// mounted in the machine.
	Destination() string

	// File permission mode (Linux only).
	Mode() string

	// Whether the volume is readonly.
	ReadOnly() bool
}

// VolumeConfig contains information about an individual volume that is to be
// mounted to a unikernel instance at runtime.
type VolumeConfig struct {
	driver      string
	source      string
	destination string
	mode        string
	readOnly    bool
}

// Driver implements Volume.
func (volume *VolumeConfig) Driver() string {
	return volume.driver
}

// Source implements Volume.
func (volume *VolumeConfig) Source() string {
	return volume.source
}

// Destination implements Volume.
func (volume *VolumeConfig) Destination() string {
	return volume.destination
}

// Mode implements Volume.
func (volume *VolumeConfig) Mode() string {
	return volume.mode
}

// ReadOnly implements Volume.
func (volume *VolumeConfig) ReadOnly() bool {
	return volume.readOnly
}

// MarshalYAML makes LibraryConfig implement yaml.Marshaller
func (volume *VolumeConfig) MarshalYAML() (interface{}, error) {
	ret := map[string]interface{}{}
	if len(volume.Source()) > 0 {
		ret["source"] = volume.Source()
		ret["readOnly"] = volume.ReadOnly()
	}
	if len(volume.Destination()) > 0 {
		ret["destination"] = volume.Destination()
	}
	if len(volume.Driver()) > 0 {
		ret["driver"] = volume.Driver()
	}
	if len(ret) == 0 {
		return nil, nil
	}
	return ret, nil
}
