// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package errortypes

import "errors"

var (
	InvalidOutputFormat           = errors.New("invalid output format")
	CertificateIdentifierRequired = errors.New("either specify a certificate name or UUID, or use the --all flag")
	CouldNotParsePEM              = errors.New("could not parse PEM")
	InvalidPrivateKeyFormat       = errors.New("could not parse private key in PKCS1 or PKCS8 format")
	CommonNameRequired            = errors.New("common name (CN) is required")
	PrivateKeyRequired            = errors.New("private key is required")
	ChainRequired                 = errors.New("chain is required")
)
