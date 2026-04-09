// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import "errors"

var (
	ErrCouldNotParsePEM        = errors.New("could not parse PEM")
	ErrInvalidPrivateKeyFormat = errors.New("could not parse private key in PKCS1 or PKCS8 format")
	ErrCommonNameRequired      = errors.New("common name (CN) is required")
	ErrPrivateKeyRequired      = errors.New("private key is required")
	ErrChainRequired           = errors.New("chain is required")
)
