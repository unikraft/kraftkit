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

package config

import (
	"fmt"
)

const (
	KRAFTKIT_TOKEN = "KRAFTKIT_TOKEN"
)

type ReadOnlyEnvError struct {
	Variable string
}

func (e *ReadOnlyEnvError) Error() string {
	return fmt.Sprintf("read-only value in %s", e.Variable)
}

func InheritEnv(c Config) Config {
	return &envConfig{Config: c}
}

type envConfig struct {
	Config
}

func (c *envConfig) Get(key string) (string, error) {
	val, _, err := c.GetWithSource(key)
	return val, err
}

func (c *envConfig) GetWithSource(key string) (string, string, error) {
	return c.Config.GetWithSource(key)
}

func (c *envConfig) GetOrDefault(key string) (val string, err error) {
	val, _, err = c.GetOrDefaultWithSource(key)
	return
}

func (c *envConfig) GetOrDefaultWithSource(key string) (val string, src string, err error) {
	val, src, err = c.GetWithSource(key)
	if err == nil && val == "" {
		val = c.Default(key)
	}

	return
}

func (c *envConfig) Default(key string) string {
	return c.Config.Default(key)
}

func (c *envConfig) CheckWriteable(key string) error {
	return c.Config.CheckWriteable(key)
}
