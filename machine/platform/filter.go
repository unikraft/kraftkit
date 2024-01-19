// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"context"
	"fmt"

	zip "api.zip"
	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
)

// storePlatformFilter is a mechanism to narrow the results returned from the
// store which is shared between all platforms.  In this filter, we prefix all
// requests to the Zip API client with a check for the machine's specification
// of the platform based on the provided argument.
func storePlatformFilter(platform Platform) zip.OnBefore {
	return func(_ context.Context, req zip.ReferenceObject) (any, error) {
		// If this object is listable, attempt to retrieve from a list from
		// the store instead.
		if list, ok := req.(*zip.ObjectList[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus]); ok {
			cached := list.Items
			list.Items = []zip.Object[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus]{}

			for _, machine := range cached {
				if machine.Spec.Platform != platform.String() {
					continue
				}

				list.Items = append(list.Items, machine)
			}
			return list, nil
		}

		// Cast the referenceable object, which we know is a spec-and-status object.
		obj := req.(*zip.Object[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus])

		if obj.Spec.Platform != platform.String() {
			return nil, fmt.Errorf("wanted machine platform \"%s\" but got \"%s\" instead for instance \"%s\"", obj.Spec.Platform, platform.String(), obj.Name)
		}

		return obj, nil
	}
}
