// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import "errors"

var (
	ConfigNil                  = errors.New("cannot instantiate ConfigManager without Config")
	UnsupportedTypeConversion  = errors.New("unsupported type conversion")
	ConfigMustBeStructPointer  = errors.New("cfg must be a pointer to a struct")
	InvalidKey                 = errors.New("invalid key")
	CannotSetField             = errors.New("cannot set field")
	CannotUnsetField           = errors.New("cannot unset field")
	CannotTraverseFurtherInMap = errors.New("cannot traverse further into map")
	SamePath                   = errors.New("same path")
	NotExist                   = errors.New("not exist")
)
