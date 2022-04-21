// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package pkg

import (
	"fmt"
	"reflect"

	"go.unikraft.io/kit/pkg/log"
)

type PackageManager struct {
	log          *log.Logger
	opts         []PackageOption
	handlerTypes map[string]reflect.Type
}

// Global package manager
var manager *PackageManager

func NewPackageManager(l *log.Logger, opts ...PackageOption) *PackageManager {
	// Also use the logger within individual handlers
	opts = append(opts, WithLogger(l))

	pm := &PackageManager{
		handlerTypes: make(map[string]reflect.Type),
		log:          l,
		opts:         opts,
	}

	// Assign a global manager if not present
	if manager == nil {
		manager = pm
	}

	return pm
}

func RegisterHandlerType(handler string, iface reflect.Type) error {
	if manager == nil {
		return fmt.Errorf("package manager not registered")
	}

	return manager.Register(handler, iface)
}

func (pm *PackageManager) AddOptions(opt ...PackageOption) {
	pm.opts = append(pm.opts, opt...)
}

func (pm *PackageManager) IsRegistered(mediaType string) bool {
	if _, ok := pm.handlerTypes[mediaType]; ok {
		return true
	}

	return false
}

func (pm *PackageManager) Register(handler string, iface reflect.Type) error {
	if pm.IsRegistered(handler) {
		return fmt.Errorf("package handler type already registered")
	}

	pm.log.Trace("Registering package handler type: %s", handler)
	pm.handlerTypes[handler] = iface

	return nil
}

func (pm *PackageManager) HandlerTypes() []string {
	types := make([]string, len(pm.handlerTypes))

	i := 0
	for k := range pm.handlerTypes {
		types[i] = k
		i++
	}

	return types
}

func (pm *PackageManager) NewHandlerFromPath(path string) (Package, error) {
	for k := range pm.handlerTypes {
		p, _ := pm.NewFromHandlerType(k)
		if p.Compatible(path) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("cannot determine media type from path")
}

func (pm *PackageManager) NewFromHandlerType(handler string) (Package, error) {
	if !pm.IsRegistered(handler) {
		return nil, fmt.Errorf("package handler type not registered: %s", handler)
	}

	pm.log.Trace("Initialising package handler type: %s", handler)
	pi := reflect.New(pm.handlerTypes[handler]).Interface()
	pi.(Package).Init(pm.opts...)
	return pi.(Package), nil
}
