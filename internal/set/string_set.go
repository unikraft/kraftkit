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

package set

import "strings"

var exists = struct{}{}

type stringSet struct {
	v []string
	m map[string]struct{}
}

// NewStringSet returns a new stringSet instance initialized with the given
// values, if any are provided.
func NewStringSet(values ...string) *stringSet {
	s := &stringSet{
		m: make(map[string]struct{}, len(values)),
		v: make([]string, 0, len(values)),
	}

	s.Add(values...)

	return s
}

func (s *stringSet) Add(values ...string) *stringSet {
	for _, value := range values {
		if s.Contains(value) {
			continue
		}
		s.m[value] = exists
		s.v = append(s.v, value)
	}

	return s
}

func (s *stringSet) Remove(values ...string) *stringSet {
	for _, value := range values {
		if !s.Contains(value) {
			continue
		}
		delete(s.m, value)
		s.v = sliceWithout(s.v, value)
	}

	return s
}

func sliceWithout(s []string, v string) []string {
	idx := -1
	for i, item := range s {
		if item == v {
			idx = i
			break
		}
	}
	if idx < 0 {
		return s
	}
	return append(s[:idx], s[idx+1:]...)
}

func (s *stringSet) ContainsExactly(value string) bool {
	return s.ContainsExactlyAnyOf(value)
}

func (s *stringSet) ContainsExactlyAnyOf(values ...string) bool {
	for _, value := range values {
		if _, c := s.m[value]; c {
			return true
		}
	}
	return false
}

func (s *stringSet) Contains(value string) bool {
	return s.ContainsAnyOf(value)
}

func (s *stringSet) ContainsAnyOf(values ...string) bool {
	for _, c := range s.v {
		for _, value := range values {
			if strings.Contains(c, value) {
				return true
			}
		}
	}
	return false
}

func (s *stringSet) Len() int {
	return len(s.m)
}

func (s *stringSet) ToSlice() []string {
	return s.v
}

func (s1 *stringSet) Equal(s2 *stringSet) bool {
	if s1.Len() != s2.Len() {
		return false
	}
	isEqual := true
	for _, v := range s1.v {
		if !s2.Contains(v) {
			isEqual = false
			break
		}
	}
	return isEqual
}
