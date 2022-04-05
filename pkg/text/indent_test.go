// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft UG.
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

package text

import "testing"

func Test_Indent(t *testing.T) {
	type args struct {
		s      string
		indent string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				s:      "",
				indent: "--",
			},
			want: "",
		},
		{
			name: "blank",
			args: args{
				s:      "\n",
				indent: "--",
			},
			want: "\n",
		},
		{
			name: "indent",
			args: args{
				s:      "one\ntwo\nthree",
				indent: "--",
			},
			want: "--one\n--two\n--three",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Indent(tt.args.s, tt.args.indent); got != tt.want {
				t.Errorf("indent() = %q, want %q", got, tt.want)
			}
		})
	}
}
