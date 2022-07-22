// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft GmbH.
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

package version

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
)

func NewCmdVersion(f *cmdfactory.Factory, version, buildDate string) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "version",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprint(f.IOStreams.Out, Format(version, buildDate))
		},
	}

	return cmd
}

func Format(version, buildTime string) string {
	version = strings.TrimPrefix(version, "v")

	var dateStr string
	if buildTime != "" {
		dateStr = fmt.Sprintf(" (%s)", buildTime)
	}

	return fmt.Sprintf("version %s%s\n%s\n", version, dateStr, version)
}
