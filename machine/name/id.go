// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package name

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// MachineID is a 16 byte universally unique identifier.
type (
	MachineID      string
	MachineShortID string
)

func (mid MachineID) String() string {
	return string(mid)
}

func (mid MachineID) Short() MachineShortID {
	return TruncateMachineID(mid)
}

func (mid MachineID) ShortString() string {
	return TruncateMachineID(mid).String()
}

func (mid MachineShortID) String() string {
	return string(mid)
}

const (
	MachineIDLen      = 64
	MachineIDShortLen = 12
)

var (
	NullMachineID       = MachineID("")
	validShortMachineID = regexp.MustCompile(fmt.Sprintf(`^[a-f0-9]{%d}$`, MachineIDLen))
	validHex            = regexp.MustCompile(fmt.Sprintf(`^[a-f0-9]{%d}$`, MachineIDShortLen))
)

// IsShortID determines if an arbitrary string *looks like* a short ID.
func IsShortMachineID(id string) bool {
	return validShortMachineID.MatchString(id)
}

// TruncateMachineID returns a shorthand version of a string identifier for
// convenience.
//
// A collision with other shorthands is very unlikely, but possible.  In case of
// a collision a lookup with TruncIndex.Get() will fail, and the caller will
// need to use a longer prefix, or the full-length Id.
func TruncateMachineID(mid MachineID) MachineShortID {
	id := mid.String()
	if i := strings.IndexRune(id, ':'); i >= 0 {
		id = id[i+1:]
	}

	if len(id) > MachineIDShortLen {
		id = id[:MachineIDShortLen]
	}

	return MachineShortID(id)
}

// NewRandomMachineID returns a random machine ID.
//
// It uses the crypto/rand reader as a source of randomness.
func NewRandomMachineID() (MachineID, error) {
	b := make([]byte, 32)
	for {
		if _, err := rand.Read(b); err != nil {
			return NullMachineID, err
		}

		mid := MachineID(hex.EncodeToString(b))
		if _, err := strconv.ParseInt(TruncateMachineID(mid).String(), 10, 64); err == nil {
			continue
		}

		return mid, nil
	}
}

// ValidateMachineID checks whether a MachineID string is a valid image
// MachineID.
func ValidateMachineID(id string) error {
	if ok := validHex.MatchString(id); !ok {
		return fmt.Errorf("image ID %q is invalid", id)
	}

	return nil
}
