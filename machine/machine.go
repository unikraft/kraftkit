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

package machine

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

// MachineName is the name of the guest.
type MachineName string
