// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package iostreams

import (
	"errors"
	"testing"
)

func TestIOStreams_ForceTerminal(t *testing.T) {
	tests := []struct {
		name      string
		iostreams *IOStreams
		arg       string
		wantTTY   bool
		wantWidth int
	}{
		{
			name:      "explicit width",
			iostreams: &IOStreams{},
			arg:       "72",
			wantTTY:   true,
			wantWidth: 72,
		},
		{
			name: "measure width",
			iostreams: &IOStreams{
				ttySize: func() (int, int, error) {
					return 72, 0, nil
				},
			},
			arg:       "true",
			wantTTY:   true,
			wantWidth: 72,
		},
		{
			name: "measure width fails",
			iostreams: &IOStreams{
				ttySize: func() (int, int, error) {
					return -1, -1, errors.New("ttySize sabotage!")
				},
			},
			arg:       "true",
			wantTTY:   true,
			wantWidth: 80,
		},
		{
			name: "apply percentage",
			iostreams: &IOStreams{
				ttySize: func() (int, int, error) {
					return 72, 0, nil
				},
			},
			arg:       "50%",
			wantTTY:   true,
			wantWidth: 36,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.iostreams.ForceTerminal(tt.arg)
			if isTTY := tt.iostreams.IsStdoutTTY(); isTTY != tt.wantTTY {
				t.Errorf("IOStreams.IsStdoutTTY() = %v, want %v", isTTY, tt.wantTTY)
			}
			if tw := tt.iostreams.TerminalWidth(); tw != tt.wantWidth {
				t.Errorf("IOStreams.TerminalWidth() = %v, want %v", tw, tt.wantWidth)
			}
		})
	}
}
