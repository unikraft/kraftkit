// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package waitgroup

import "sync"

type WaitGroup[T comparable] struct {
	lsz sync.WaitGroup
	mu  sync.RWMutex
	li  []T
}

// Add to the wait group.
func (wg *WaitGroup[T]) Add(k T) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if wg.Contains(k) {
		return
	}

	wg.li = append(wg.li, k)
	wg.lsz.Add(1)
}

// Done signals that the provided entity can be removed from the wait group.
func (wg *WaitGroup[T]) Done(needle T) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if !wg.Contains(needle) {
		return
	}

	for i, k := range wg.li {
		if k == needle {
			wg.li = append(wg.li[:i], wg.li[i+1:]...)
			wg.lsz.Done()
			return
		}
	}
}

// Wait for all items in the wait group to be removed.
func (wg *WaitGroup[T]) Wait() {
	wg.lsz.Wait()
}

// Contains checks if the provided entity is still in the wait group.
func (wg *WaitGroup[T]) Contains(needle T) bool {
	for _, mid := range wg.li {
		if mid == needle {
			return true
		}
	}

	return false
}

// Items returns the list of items in the wait group.
func (wg *WaitGroup[T]) Items() []T {
	return wg.li
}
