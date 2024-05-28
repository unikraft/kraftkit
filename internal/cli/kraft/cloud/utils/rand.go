// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// GenRandAuth generates a random authentication string.
func GenRandAuth() (string, error) {
	rndChars := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	maxIdx := big.NewInt(int64(len(rndChars)))

	const authLen = 32
	var auth strings.Builder
	auth.Grow(authLen)

	var i *big.Int
	var err error
	for range authLen {
		if i, err = rand.Int(rand.Reader, maxIdx); err != nil {
			return "", err
		}
		if err = auth.WriteByte(rndChars[i.Int64()]); err != nil {
			return "", err
		}
	}

	return auth.String(), nil
}
