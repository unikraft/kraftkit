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

package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

const (
	KRAFTKIT_CONFIG_DIR = "KRAFTKIT_CONFIG_DIR"
	XDG_CONFIG_HOME     = "XDG_CONFIG_HOME"
	XDG_STATE_HOME      = "XDG_STATE_HOME"
	XDG_DATA_HOME       = "XDG_DATA_HOME"
	APP_DATA            = "AppData"
	LOCAL_APP_DATA      = "LocalAppData"
)

// Config path precedence
// 1. KRAFTKIT_CONFIG_DIR
// 2. XDG_CONFIG_HOME
// 3. AppData (windows only)
// 4. HOME
func ConfigDir() string {
	var path string
	if a := os.Getenv(KRAFTKIT_CONFIG_DIR); a != "" {
		path = a
	} else if b := os.Getenv(XDG_CONFIG_HOME); b != "" {
		path = filepath.Join(b, "kraftkit")
	} else if c := os.Getenv(APP_DATA); runtime.GOOS == "windows" && c != "" {
		path = filepath.Join(c, "KraftKit")
	} else {
		d, _ := os.UserHomeDir()
		path = filepath.Join(d, ".config", "kraftkit")
	}

	// If the path does not exist and the KRAFTKIT_CONFIG_DIR flag is not set try
	// migrating config from default paths.
	if !dirExists(path) && os.Getenv(KRAFTKIT_CONFIG_DIR) == "" {
		_ = autoMigrateConfigDir(path)
	}

	return path
}

// State path precedence
// 1. XDG_STATE_HOME
// 2. LocalAppData (windows only)
// 3. HOME
func StateDir() string {
	var path string
	if a := os.Getenv(XDG_STATE_HOME); a != "" {
		path = filepath.Join(a, "kraftkit")
	} else if b := os.Getenv(LOCAL_APP_DATA); runtime.GOOS == "windows" && b != "" {
		path = filepath.Join(b, "KraftKit")
	} else {
		c, _ := os.UserHomeDir()
		path = filepath.Join(c, ".local", "state", "kraftkit")
	}

	// If the path does not exist try migrating state from default paths
	if !dirExists(path) {
		_ = autoMigrateStateDir(path)
	}

	return path
}

// Data path precedence
// 1. XDG_DATA_HOME
// 2. LocalAppData (windows only)
// 3. HOME
func DataDir() string {
	var path string
	if a := os.Getenv(XDG_DATA_HOME); a != "" {
		path = filepath.Join(a, "kraftkit")
	} else if b := os.Getenv(LOCAL_APP_DATA); runtime.GOOS == "windows" && b != "" {
		path = filepath.Join(b, "KraftKit")
	} else {
		c, _ := os.UserHomeDir()
		path = filepath.Join(c, ".local", "share", "kraftkit")
	}

	return path
}

var (
	errSamePath = errors.New("same path")
	errNotExist = errors.New("not exist")
)

// Check default path, os.UserHomeDir, for existing configs
// If configs exist then move them to newPath
func autoMigrateConfigDir(newPath string) error {
	path, err := os.UserHomeDir()
	if oldPath := filepath.Join(path, ".config", "kraftkit"); err == nil && dirExists(oldPath) {
		return migrateDir(oldPath, newPath)
	}

	return errNotExist
}

// Check default path, os.UserHomeDir, for existing state file (state.yml)
// If state file exist then move it to newPath
func autoMigrateStateDir(newPath string) error {
	path, err := os.UserHomeDir()
	if oldPath := filepath.Join(path, ".config", "kraftkit"); err == nil && dirExists(oldPath) {
		return migrateFile(oldPath, newPath, "state.yml")
	}

	return errNotExist
}

func migrateFile(oldPath, newPath, file string) error {
	if oldPath == newPath {
		return errSamePath
	}

	oldFile := filepath.Join(oldPath, file)
	newFile := filepath.Join(newPath, file)

	if !fileExists(oldFile) {
		return errNotExist
	}

	_ = os.MkdirAll(filepath.Dir(newFile), 0o755)
	return os.Rename(oldFile, newFile)
}

func migrateDir(oldPath, newPath string) error {
	if oldPath == newPath {
		return errSamePath
	}

	if !dirExists(oldPath) {
		return errNotExist
	}

	_ = os.MkdirAll(filepath.Dir(newPath), 0o755)
	return os.Rename(oldPath, newPath)
}

func dirExists(path string) bool {
	f, err := os.Stat(path)
	return err == nil && f.IsDir()
}

func fileExists(path string) bool {
	f, err := os.Stat(path)
	return err == nil && !f.IsDir()
}

func DefaultConfigFile() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func HomeDirPath(subdir string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	newPath := filepath.Join(homeDir, subdir)
	return newPath, nil
}

var ReadConfigFile = func(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, pathError(err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

var WriteConfigFile = func(filename string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(filename), 0o771)
	if err != nil {
		return pathError(err)
	}

	cfgFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600) // cargo coded from setup
	if err != nil {
		return err
	}
	defer cfgFile.Close()

	_, err = cfgFile.Write(data)
	return err
}

var BackupConfigFile = func(filename string) error {
	return os.Rename(filename, filename+".bak")
}

func pathError(err error) error {
	var pathError *os.PathError
	if errors.As(err, &pathError) && errors.Is(pathError.Err, syscall.ENOTDIR) {
		if p := findRegularFile(pathError.Path); p != "" {
			return fmt.Errorf("remove or rename regular file `%s` (must be a directory)", p)
		}
	}
	return err
}

func findRegularFile(p string) string {
	for {
		if s, err := os.Stat(p); err == nil && s.Mode().IsRegular() {
			return p
		}
		newPath := filepath.Dir(p)
		if newPath == p || newPath == string(filepath.Separator) || newPath == "." {
			break
		}
		p = newPath
	}
	return ""
}
