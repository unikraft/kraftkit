// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Stefan Jumarea <stefanjumarea02@gmail.com>
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

// Package config provides the kraft configuration functions
package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/halorium/env"

	"kraftkit.sh/utils"
)

// EnvFeeder feeds using environment variables.
type EnvFeeder struct{}

// Feed the environment variables into the given interface.
func (f EnvFeeder) Feed(structure interface{}) error {
	err := env.Unmarshal(*structure.(**Config))

	var obj AuthConfig
	rv := reflect.ValueOf(obj)

	cfg := *structure.(**Config)
	cfg.Auth = make(map[string]AuthConfig)
	for i := 0; i < rv.NumField(); i++ {

		rsf := rv.Type().Field(i)
		tag := rsf.Tag.Get("env")

		if !strings.Contains(tag, "%s") {
			continue
		}

		prefix := strings.Split(tag, "%s")[0]
		suffix := strings.Split(tag, "%s")[1]

		envVars := utils.Filter(os.Environ(), func(s string) bool {
			return strings.HasPrefix(s, prefix) &&
				strings.HasSuffix(strings.Split(s, "=")[0], suffix)
		})

		for _, s := range envVars {
			index := utils.GetStringInBetween(s, prefix, suffix)
			index = strings.ToLower(index)

			entry, exists := cfg.Auth[index]
			if !exists {
				entry = *new(AuthConfig)
			}

			authRv := reflect.ValueOf(&entry).Elem()
			for j := 0; j < authRv.NumField(); j++ {
				authRsf := authRv.Type().Field(j)
				authRf := authRv.Field(j)
				authTag := authRsf.Tag.Get("env")

				stringValue := strings.Split(s, "=")[1]

				if strings.HasSuffix(strings.Split(s, "=")[0],
					strings.Split(authTag, "%s")[1]) {

					switch authRf.Type().Kind() {
					case reflect.String:
						authRf.SetString(stringValue)
					case reflect.Bool:
						val, err := strconv.ParseBool(stringValue)
						if err != nil {
							return err
						}
						authRf.SetBool(val)
					case reflect.Int:
						val, err := strconv.ParseInt(stringValue, 0, 32)
						if err != nil {
							return err
						}
						authRf.SetInt(val)
					}
				}
			}
			cfg.Auth[index] = entry

		}
	}

	return err
}

// Do nothing, we do not set the environment variables based on the
// given interface.
func (f EnvFeeder) Write(structure interface{}, merge bool) error {
	return nil
}
