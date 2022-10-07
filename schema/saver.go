// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"io"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
	"kraftkit.sh/unikraft/app"
)

type SaverOptions struct {
	// Whether or not to save the old kraftfile
	saveOldKraftfile bool
	// // Slice of component options to apply to each loaded component
	// componentOptions []component.ComponentOption
	// // Access to a general purpose logger
	// log log.Logger
}

type SaverOption func(so *SaverOptions) error

func WithSaveOldConfig(saveOldKraftfile bool) SaverOption {
	return func(so *SaverOptions) error {
		so.saveOldKraftfile = saveOldKraftfile
		return nil
	}
}

func SaveApplicationConfig(project *app.ApplicationConfig, sopts ...SaverOption) error {
	options := &SaverOptions{}
	for _, o := range sopts {
		err := o(options)
		if err != nil {
			return err
		}
	}

	// Marshal the application config
	b, err := yaml.Marshal(project)
	if err != nil {
		return err
	}

	// Write the application config to the first kraft config file
	kraftFile := project.KraftFiles[0]

	// Copy the old file to a backup with .old appended
	if options.saveOldKraftfile {
		source, err := os.Open(kraftFile)
		if err != nil {
			return err
		}

		destination, err := os.Create(kraftFile + ".old")
		if err != nil {
			return err
		}

		_, err = io.Copy(destination, source)
		if err != nil {
			return err
		}
		source.Close()
		destination.Close()
	}

	if err := ioutil.WriteFile(kraftFile, b, 0644); err != nil {
		return err
	}

	return nil
}
