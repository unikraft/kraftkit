// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
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

package qmp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"time"

	"kraftkit.sh/utils"
)

var ErrAcceptedNonEvent = errors.New("did not receive an event")

type Timestamp struct {
	Seconds      uint64 `json:"seconds"`
	Microseconds uint64 `json:"microseconds"`
}

type QMPEvent[T utils.ComparableStringer] struct {
	Event     T         `json:"event"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type QMPEventMonitor[T utils.ComparableStringer] struct {
	client *bufio.Reader
	types  []T
}

func NewQMPEventMonitor[T utils.ComparableStringer](client io.ReadWriteCloser, types []T, typeMap map[T]reflect.Type) (*QMPEventMonitor[T], error) {
	monitor := QMPEventMonitor[T]{
		client: bufio.NewReader(client),
		types:  types,
	}

	return &monitor, nil
}

// Accept receives exactly one input event from the QMP service and then
// returns.  The method will wait until it receives the event.
func (em *QMPEventMonitor[T]) Accept() (*QMPEvent[T], error) {
	data, err := em.client.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	// Use a generic to serialize as a map (which is the very least we can expect)
	// from the Unmarshal result.  We can then peak at it before applying a type.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	typ, ok := raw["event"]
	if !ok {
		return nil, ErrAcceptedNonEvent
	}

	var t T
	found := false
	for _, needle := range em.types {
		if needle.String() == typ {
			t = needle
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("unknown QMP event type: %s", typ)
	}

	return &QMPEvent[T]{
		Event: t,
	}, nil
}
