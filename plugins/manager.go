// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
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
// SOFTWARE.

package plugins

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	goplugin "plugin"
	"runtime"
	"strings"

	"github.com/cli/safeexec"
	"github.com/juju/errors"
	"gopkg.in/yaml.v3"

	"kraftkit.sh/internal/findsh"
	"kraftkit.sh/log"
)

type PluginManager struct {
	dataDir    string
	lookPath   func(string) (string, error)
	findSh     func() (string, error)
	newCommand func(string, ...string) *exec.Cmd
	platform   func() (string, string)
	log        log.Logger
}

func NewPluginManager(dataDir string, l log.Logger) *PluginManager {
	return &PluginManager{
		dataDir:    dataDir,
		lookPath:   safeexec.LookPath,
		findSh:     findsh.Find,
		newCommand: exec.Command,
		log:        l,
		platform: func() (string, string) {
			ext := ".so"

			if runtime.GOOS == "windows" {
				ext = ".dll"
			}

			return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH), ext
		},
	}
}

func (pm *PluginManager) parsePluginFile(fi fs.FileInfo) (Plugin, error) {
	exePath := filepath.Join(pm.dataDir, fi.Name())
	ext := Plugin{
		isLocal: true,
	}

	if !isSymlink(fi.Mode()) {
		// if this is a regular file, its contents is the local directory of the
		// plugin
		p, err := readPathFromFile(filepath.Join(pm.dataDir, fi.Name()))
		if err != nil {
			return ext, err
		}

		exePath = filepath.Join(p, fi.Name())
	}

	ext.path = exePath
	return ext, nil
}

func (pm *PluginManager) parseBinaryPluginDir(fi fs.FileInfo) (Plugin, error) {
	exePath := filepath.Join(pm.dataDir, fi.Name(), fi.Name())
	ext := Plugin{
		path: exePath,
		kind: BinaryKind,
	}

	manifestPath := filepath.Join(pm.dataDir, fi.Name(), PluginManifestFile)
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return ext, errors.Annotatef(err, "could not open %s for reading", manifestPath)
	}

	var man PluginManifest
	err = yaml.Unmarshal(manifest, &man)
	if err != nil {
		return ext, errors.Annotatef(err, "could not parse %s", manifestPath)
	}

	// repo := ghrepo.NewWithHost(bm.Owner, bm.Name, bm.Host)
	// remoteURL := ghrepo.GenerateRepoURL(repo, "")
	// ext.url = remoteURL
	ext.currentVersion = man.Version
	return ext, nil
}

// getRemoteUrl determines the remote URL for non-local git plugins.
func (pm *PluginManager) getRemoteUrl(plugin string) string {
	gitExe, err := pm.lookPath("git")
	if err != nil {
		return ""
	}

	// TODO: add #filter= for sparse checkout of sub-directory

	gitDir := "--git-dir=" + filepath.Join(pm.dataDir, plugin, ".git")
	cmd := pm.newCommand(gitExe, gitDir, "config", "remote.origin.url")
	url, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(url))
}

// getCurrentVersion determines the current version for non-local git plugins.
func (m *PluginManager) getCurrentVersion(plugin string) string {
	gitExe, err := m.lookPath("git")
	if err != nil {
		return ""
	}

	gitDir := "--git-dir=" + filepath.Join(m.dataDir, plugin, ".git")
	cmd := m.newCommand(gitExe, gitDir, "rev-parse", "HEAD")
	localSha, err := cmd.Output()
	if err != nil {
		return ""
	}

	return string(bytes.TrimSpace(localSha))
}

func (pm *PluginManager) parseGitPluginDir(fi fs.FileInfo) (Plugin, error) {
	exePath := filepath.Join(pm.dataDir, fi.Name(), fi.Name())
	remoteUrl := pm.getRemoteUrl(fi.Name())
	currentVersion := pm.getCurrentVersion(fi.Name())
	return Plugin{
		path:           exePath,
		url:            remoteUrl,
		isLocal:        false,
		currentVersion: currentVersion,
		kind:           GitKind,
	}, nil
}

func (m *PluginManager) parsePluginDir(fi fs.FileInfo) (Plugin, error) {
	if _, err := os.Stat(filepath.Join(m.dataDir, fi.Name(), PluginManifestFile)); err == nil {
		return m.parseBinaryPluginDir(fi)
	}

	return m.parseGitPluginDir(fi)
}

func (pm *PluginManager) List() ([]Plugin, error) {
	var results []Plugin

	if f, err := os.Stat(pm.dataDir); err != nil || !f.IsDir() {
		if err := os.MkdirAll(pm.dataDir, 0o755); err != nil {
			return results, errors.Errorf("%v: %s", err, pm.dataDir)
		}
	}

	entries, err := os.ReadDir(pm.dataDir)
	if err != nil {
		return nil, err
	}

	for _, f := range entries {
		if !strings.HasPrefix(f.Name(), PluginNamePrefix) {
			continue
		}

		var plugin Plugin
		var err error
		finfo, err := f.Info()
		if err != nil {
			return nil, err
		}
		if f.IsDir() {
			plugin, err = pm.parsePluginDir(finfo)
			if err != nil {
				return nil, err
			}

			results = append(results, plugin)
		} else {
			plugin, err = pm.parsePluginFile(finfo)
			if err != nil {
				return nil, err
			}

			results = append(results, plugin)
		}
	}

	return results, nil
}

func (pm *PluginManager) Install(repo string) error {
	return errors.New("not implemented")
}

func (pm *PluginManager) InstallLocal(repo string) error {
	return errors.New("not implemented")
}

func (pm *PluginManager) Dispatch() error {
	plugins, err := pm.List()
	if err != nil {
		return err
	}

	for _, plugin := range plugins {
		pm.log.Tracef("dispatching plugin: %s", plugin.Path())

		_, err := goplugin.Open(plugin.Path())
		if err != nil {
			pm.log.Error("could not open plugin: %s", err)
		}
	}

	return nil
}

func isSymlink(m os.FileMode) bool {
	return m&os.ModeSymlink != 0
}

// reads the product of makeSymlink on Windows
func readPathFromFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}

	defer f.Close()

	b := make([]byte, 1024)
	n, err := f.Read(b)

	return strings.TrimSpace(string(b[:n])), err
}
