// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package api

import (
	"encoding/gob"

	zip "api.zip"
	"k8s.io/apimachinery/pkg/api/resource"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
)

func init() {
	gob.Register(resource.Quantity{})
}

func RegisterSchemes() error {
	return zip.Register(
		machinev1alpha1.AddToScheme,
		networkv1alpha1.AddToScheme,
		volumev1alpha1.AddToScheme,
	)
}
