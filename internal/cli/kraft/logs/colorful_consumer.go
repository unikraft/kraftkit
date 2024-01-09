// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"fmt"

	rainbow "kraftkit.sh/internal/rainbowprint"

	"kraftkit.sh/iostreams"
)

type colorfulConsumer struct {
	streams *iostreams.IOStreams
	color   rainbow.ColorFunc
	prefix  string
}

// NewColorfulConsumer creates a new log consumer which prefixes each line with a colorful prefix.
func NewColorfulConsumer(streams *iostreams.IOStreams, color bool, prefix string) (*colorfulConsumer, error) {
	if streams == nil {
		return nil, fmt.Errorf("cannot create a colorful consumer with nil IOStreams")
	}

	var colorFunc rainbow.ColorFunc
	if color {
		rainbow.SetANSIMode(streams, rainbow.Auto)
	} else {
		rainbow.SetANSIMode(streams, rainbow.Never)
	}

	colorFunc = rainbow.NextColor()
	return &colorfulConsumer{streams: streams, color: colorFunc, prefix: prefix}, nil
}

// Consume implements logConsumer
func (c colorfulConsumer) consume(s string) error {
	if c.prefix != "" {
		s = fmt.Sprintf("%s | %s", c.color(c.prefix), s)
	}

	fmt.Fprint(c.streams.Out, s)

	return nil
}
