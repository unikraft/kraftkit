// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukrandom

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"strings"

	"kraftkit.sh/unikraft/export/v0/ukargparse"
)

var ParamRandomSeed = ukargparse.NewParamStrSlice("random", "seed", nil)

// ExportedParams returns the parameters available by this exported library.
func ExportedParams() []ukargparse.Param {
	return []ukargparse.Param{
		ParamRandomSeed,
	}
}

// RandomSeed are the 8 ints that are required by the ukrandom library.
type RandomSeed [8]uint32

// NewRandomSeed generates a new set of true random integers or nothing if error.
func NewRandomSeed() (random RandomSeed) {
	maxUint32 := big.NewInt(math.MaxUint32)
	for i := 0; i < 8; i++ {
		val, err := rand.Int(rand.Reader, maxUint32)
		if err != nil {
			return RandomSeed{}
		}
		random[i] = uint32(val.Uint64())
	}

	return random
}

// String implements fmt.Stringer and returns a valid set of random bytes.
func (rng RandomSeed) String() string {
	var sb strings.Builder

	sb.WriteString("[ ")
	for i := 0; i < 8; i++ {
		sb.WriteString(fmt.Sprintf("%04x ", rng[i]))
	}
	sb.WriteString("]")

	return sb.String()
}
