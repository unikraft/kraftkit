// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

// RemoveDuplicates removes duplicates from a sorted slice in-place.
func RemoveDuplicates[T comparable](nums []T) []T {
	if len(nums) == 0 {
		return nums
	}

	j := 0 // index of the last unique element
	for i := 1; i < len(nums); i++ {
		if nums[i] != nums[j] {
			j++
			nums[j] = nums[i]
		}
	}
	return nums[:j+1]
}
