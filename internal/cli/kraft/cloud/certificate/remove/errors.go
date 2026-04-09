// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import "errors"

var (
	ErrCertificateIdentifierRequired = errors.New("either specify a certificate name or UUID, or use the --all flag")
	ErrInvalidOutputFormat           = errors.New("invalid output format")
)
