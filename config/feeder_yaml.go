// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH. All rights reserved.
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

package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"kraftkit.sh/internal/yamlmerger"
)

// YamlFeeder feeds using a YAML file.
type YamlFeeder struct {
	File string
}

func (f YamlFeeder) Feed(structure interface{}) error {
	file, err := os.Open(filepath.Clean(f.File))
	if err != nil {
		return fmt.Errorf("cannot open yaml file: %v", err)
	}

	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// File is empty, ignore
	if stat.Size() == 0 {
		return nil
	}

	if err = yaml.NewDecoder(file).Decode(structure); err != nil {
		return fmt.Errorf("cannot feed config file: %v", err)
	}

	return nil
}

func (yf YamlFeeder) Write(structure interface{}, merge bool) error {
	if len(yf.File) == 0 {
		return fmt.Errorf("filename for YAML cannot be empty")
	}

	// Create parent directories if not present
	err := os.MkdirAll(filepath.Dir(yf.File), 0o771)
	if err != nil {
		return pathError(err)
	}

	// Open the file (create if not present)
	f, err := os.OpenFile(yf.File, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}

	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("could not read file: %v", err)
	}

	from := yaml.Node{}
	if err := yaml.Unmarshal(data, &from); err != nil {
		return fmt.Errorf("could not unmarshal YAML: %s", err)
	}

	yml, err := yaml.Marshal(structure)
	if err != nil {
		return err
	}

	into := yaml.Node{}
	if err := yaml.Unmarshal(yml, &into); err != nil {
		return err
	}

	// When kind is 0, it is an uninitialized YAML structure (aka empty file)
	if from.Kind != 0 && merge {
		if err := yamlmerger.RecursiveMerge(&from, &into); err != nil {
			return fmt.Errorf("could not update config: %v", err)
		}
	}

	if err := f.Truncate(0); err != nil {
		return err
	}

	if _, err := f.Seek(0, 0); err != nil {
		return err
	}

	return yaml.NewEncoder(f).Encode(&into)
}
