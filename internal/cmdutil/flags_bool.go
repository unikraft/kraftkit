// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright (c) 2012 Alex Ogier. All rights reserved.
// Copyright (c) 2012 The Go Authors. All rights reserved.
// Copyright (2) 2022 Unikraft GmbH. All rights reserved.
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

package cmdutil

import (
	"strconv"

	"github.com/spf13/pflag"
)

type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Set(val string) error {
	v, err := strconv.ParseBool(val)
	*b = boolValue(v)
	return err
}

func (b *boolValue) Type() string {
	return "bool"
}

func (b *boolValue) String() string { return strconv.FormatBool(bool(*b)) }

// BoolVarP is like BoolVar, but accepts a shorthand letter that can be used
// after a single dash.
func BoolVarP(p *bool, name, shorthand string, value bool, usage string) *pflag.Flag {
	flag := VarPF(newBoolValue(value, p), name, shorthand, usage)
	flag.NoOptDefVal = "true"
	return flag
}
