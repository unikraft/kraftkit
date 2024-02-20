// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package network

import (
	"context"
	"fmt"

	zip "api.zip"
	"github.com/acorn-io/baaah/pkg/merr"
	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
)

type networkV1alpha1ServiceIterator struct {
	strategies map[string]networkv1alpha1.NetworkService
}

// NewNetworkV1alpha1ServiceIterator returns a
// networkv1alpha1.NetworkService-compatible implementation which iterates over
// each supported network driver and calls the representing method.  This is
// useful in circumstances where the driver is not supplied.  The first network
// driver to succeed is returned in all circumstances.
func NewNetworkV1alpha1ServiceIterator(ctx context.Context) (networkv1alpha1.NetworkService, error) {
	var err error
	iterator := networkV1alpha1ServiceIterator{
		strategies: map[string]networkv1alpha1.NetworkService{},
	}

	for driver, strategy := range hostSupportedStrategies() {
		iterator.strategies[driver], err = strategy.NewNetworkV1alpha1(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &iterator, nil
}

// Create implements kraftkit.sh/api/network/v1alpha1.Create
func (iterator *networkV1alpha1ServiceIterator) Create(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Create(ctx, network)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return network, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Start implements kraftkit.sh/api/network/v1alpha1.Start
func (iterator *networkV1alpha1ServiceIterator) Start(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Start(ctx, network)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return network, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Stop implements kraftkit.sh/api/network/v1alpha1.Stop
func (iterator *networkV1alpha1ServiceIterator) Stop(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Stop(ctx, network)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return network, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Update implements kraftkit.sh/api/network/v1alpha1.Update.
func (iterator *networkV1alpha1ServiceIterator) Update(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Update(ctx, network)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return network, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Delete implements kraftkit.sh/api/network/v1alpha1.Delete
func (iterator *networkV1alpha1ServiceIterator) Delete(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Delete(ctx, network)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return network, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Get implements kraftkit.sh/api/network/v1alpha1.Get
func (iterator *networkV1alpha1ServiceIterator) Get(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Get(ctx, network)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return network, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// List implements kraftkit.sh/api/network/v1alpha1.List
func (iterator *networkV1alpha1ServiceIterator) List(ctx context.Context, cached *networkv1alpha1.NetworkList) (*networkv1alpha1.NetworkList, error) {
	found := []zip.Object[networkv1alpha1.NetworkSpec, networkv1alpha1.NetworkStatus]{}

	for _, strategy := range iterator.strategies {
		ret, err := strategy.List(ctx, &networkv1alpha1.NetworkList{})
		if err != nil {
			continue
		}

		found = append(found, ret.Items...)
	}

	cached.Items = found

	return cached, nil
}
