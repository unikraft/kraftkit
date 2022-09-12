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

package cmdfactory

import (
	"context"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/httpclient"
	"kraftkit.sh/internal/logger"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/plugins"
)

type FactoryOption func(*Factory)

type Factory struct {
	RootCmd        *cobra.Command
	IOStreams      *iostreams.IOStreams
	PluginManager  func() (*plugins.PluginManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	Logger         func() (log.Logger, error)
	HttpClient     func() (*http.Client, error)
}

// New creates a new Factory object
func New(opts ...FactoryOption) *Factory {
	f := &Factory{
		ConfigManager: configManagerFunc(),
	}

	// Depends on Config
	f.IOStreams = ioStreams(f)

	// Depends on Config, IOStreams
	f.HttpClient = httpClientFunc(f)

	// Depends on Config, IOStreams
	f.Logger = loggerFunc(f)

	// Depends on Config, HttpClient, and IOStreams
	f.PluginManager = pluginManagerFunc(f)

	// Loop through each option
	for _, opt := range opts {
		// Call the option giving the instantiated *Factory as the argument
		opt(f)
	}

	// Force the terminal if desired
	if spec := os.Getenv("KRAFTKIT_FORCE_TTY"); spec != "" {
		f.IOStreams.ForceTerminal(spec)
	}

	// Enable running kraftkit from Windows File Explorer's address bar.  Without
	// this, the user is told to stop and run from a terminal.  With this, a user
	// can take any action directly from explorer.
	if len(os.Args) > 1 && os.Args[1] != "" {
		cobra.MousetrapHelpText = ""
	}

	return f
}

func WithPackageManager() FactoryOption {
	return func(f *Factory) {
		// Depends on Config, HttpClient, and IOStreams
		f.PackageManager = packageManagerFunc(f)
	}
}

func configManagerFunc() func() (*config.ConfigManager, error) {
	var cfgm *config.ConfigManager
	var cfge error

	return func() (*config.ConfigManager, error) {
		if cfgm != nil || cfge != nil {
			return cfgm, cfge
		}

		cfgm, cfge = config.NewConfigManager(
			config.WithDefaultConfigFile(),
			config.WithEnv(),
		)

		return cfgm, cfge
	}
}

func ioStreams(f *Factory) *iostreams.IOStreams {
	io := iostreams.System()
	cfgm, err := f.ConfigManager()
	if err != nil {
		return io
	}

	if cfgm.Config.NoPrompt {
		io.SetNeverPrompt(true)
	} else if (io.ColorEnabled() || io.IsStdoutTTY()) && cfgm.Config.Log.Type == "" {
		cfgm.Config.Log.Type = logger.LoggerTypeToString(logger.FANCY)
	}

	// Pager precedence
	// 1. KRAFTKIT_PAGER
	// 2. pager from config
	// 3. PAGER
	if kkPager, kkPagerExists := os.LookupEnv("KRAFTKIT_PAGER"); kkPagerExists {
		io.SetPager(kkPager)
	} else if pager := cfgm.Config.Pager; pager != "" {
		io.SetPager(pager)
	}

	return io
}

func httpClientFunc(f *Factory) func() (*http.Client, error) {
	return func() (*http.Client, error) {
		cfgm, err := f.ConfigManager()
		if err != nil {
			return nil, err
		}

		return httpclient.NewHTTPClient(
			f.IOStreams,
			cfgm.Config.HTTPUnixSocket,
			true,
		)
	}
}

func loggerFunc(f *Factory) func() (log.Logger, error) {
	return func() (log.Logger, error) {
		cfgm, err := f.ConfigManager()
		if err != nil {
			return nil, err
		}

		l := logger.NewLogger(f.IOStreams.Out, f.IOStreams.ColorScheme())
		l.SetLevel(logger.LogLevelFromString(cfgm.Config.Log.Level))

		return l, nil
	}
}

func packageManagerFunc(f *Factory) func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error) {
	return func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error) {
		cfgm, err := f.ConfigManager()
		if err != nil {
			return nil, err
		}

		log, err := f.Logger()
		if err != nil {
			return nil, err
		}

		// Add access to global config and the instantiated logger to the options
		opts = append(opts, []packmanager.PackageManagerOption{
			packmanager.WithConfigManager(cfgm),
			packmanager.WithLogger(log),
		}...)

		options, err := packmanager.NewPackageManagerOptions(
			context.TODO(),
			opts...,
		)
		if err != nil {
			return nil, err
		}

		umbrella, err := packmanager.NewUmbrellaManagerFromOptions(options)
		if err != nil {
			return nil, err
		}

		return umbrella, nil
	}
}

func pluginManagerFunc(f *Factory) func() (*plugins.PluginManager, error) {
	var pm *plugins.PluginManager
	var perr error

	return func() (*plugins.PluginManager, error) {
		if pm != nil || perr != nil {
			return pm, perr
		}

		var cfgm *config.ConfigManager
		cfgm, perr = f.ConfigManager()
		if perr != nil {
			return nil, perr
		}

		var l log.Logger
		l, perr = f.Logger()
		if perr != nil {
			return nil, perr
		}

		pm = plugins.NewPluginManager(cfgm.Config.Paths.Plugins, l)

		return pm, perr
	}
}
