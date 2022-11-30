// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package cli

import (
	"fmt"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/plugins"

	"kraftkit.sh/internal/httpclient"
)

type CliOptions struct {
	ioStreams      *iostreams.IOStreams
	logger         *logrus.Entry
	configManager  func() (*config.ConfigManager, error)
	packageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	pluginManager  func() (*plugins.PluginManager, error)
	httpClient     func() (*http.Client, error)
}

type CliOption func(*CliOptions)

// WithDefaultLogger sets up the built in logger based on provided conifg found
// from the ConfigManager.
func WithDefaultLogger() CliOption {
	return func(copts *CliOptions) {
		if copts.logger != nil {
			return
		}

		if copts.configManager == nil {
			copts.logger = log.L
			return
		}

		// Set up a default logger based on the internal TextFormatter
		logger := logrus.New()

		// Configure the logger based on parameters set by in KraftKit's
		// configuration
		cfgm, err := copts.configManager()
		if err != nil {
			copts.logger = log.L
		}

		switch log.LoggerTypeFromString(cfgm.Config.Log.Type) {
		case log.QUIET:
			formatter := new(logrus.TextFormatter)
			logger.Formatter = formatter

		case log.BASIC:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if cfgm.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.FANCY:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if cfgm.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.JSON:
			formatter := new(logrus.JSONFormatter)
			formatter.DisableTimestamp = true

			if cfgm.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			}

			logger.Formatter = formatter
		}

		level, ok := log.Levels()[cfgm.Config.Log.Level]
		if !ok {
			logger.Level = logrus.InfoLevel
		} else {
			logger.Level = level
		}

		if copts.ioStreams != nil {
			logger.SetOutput(copts.ioStreams.Out)
		}

		// Save the logger
		copts.logger = logrus.NewEntry(logger)
	}
}

// WithConfigManager sets a previously instantiate ConfigManager to be used as
// part of the CLI options.
func WithConfigManager(cfgm *config.ConfigManager) CliOption {
	return func(copts *CliOptions) {
		copts.configManager = func() (*config.ConfigManager, error) {
			return cfgm, nil
		}
	}
}

// WithDefaultConfigManager instantiates a configuration manager based on
// default options.
func WithDefaultConfigManager() CliOption {
	return func(copts *CliOptions) {
		if copts.configManager != nil {
			return
		}

		var cfgm *config.ConfigManager
		var cfge error

		copts.configManager = func() (*config.ConfigManager, error) {
			if cfgm != nil || cfge != nil {
				return cfgm, cfge
			}

			cfgm, cfge := config.NewConfigManager(
				config.WithDefaultConfigFile(),
			)

			return cfgm, cfge
		}
	}
}

// WithIOStreams sets a previously instantiated iostreams.IOStreams structure to
// be used within the command.
func WithIOStreams(io *iostreams.IOStreams) CliOption {
	return func(copts *CliOptions) {
		copts.ioStreams = io
	}
}

// WithDefaultIOStreams instantiates ta new IO streams using environmental
// variables and host-provided configuration.
func WithDefaultIOStreams() CliOption {
	return func(copts *CliOptions) {
		if copts.ioStreams != nil {
			return
		}

		io := iostreams.System()

		if copts.configManager != nil {
			cfgm, err := copts.configManager()
			if err != nil {
				if cfgm.Config.NoPrompt {
					io.SetNeverPrompt(true)
				}

				if pager := cfgm.Config.Pager; pager != "" {
					io.SetPager(pager)
				}
			}
		}

		// Pager precedence
		// 1. KRAFTKIT_PAGER
		// 2. pager from config
		// 3. PAGER
		if kkPager, kkPagerExists := os.LookupEnv("KRAFTKIT_PAGER"); kkPagerExists {
			io.SetPager(kkPager)
		}

		copts.ioStreams = io
	}
}

// WithHTTPClient sets a previously instantiated http.Client to be used within
// the command.
func WithHTTPClient(httpClient *http.Client) CliOption {
	return func(copts *CliOptions) {
		copts.httpClient = func() (*http.Client, error) {
			return httpClient, nil
		}
	}
}

// WithDefaultHTTPClient initializes a HTTP client using host-provided
// configuration.
func WithDefaultHTTPClient() CliOption {
	return func(copts *CliOptions) {
		if copts.httpClient != nil {
			return
		}

		copts.httpClient = func() (*http.Client, error) {
			cfgm, err := copts.configManager()
			if err != nil {
				return nil, fmt.Errorf("cannot access config manager")
			}

			return httpclient.NewHTTPClient(
				copts.ioStreams,
				cfgm.Config.HTTPUnixSocket,
				true,
			)
		}
	}
}

// WithPackageManager sets a previously initialized package manager to be used
// with the command.
func WithPackageManager(pm packmanager.PackageManager) CliOption {
	return func(copts *CliOptions) {
		copts.packageManager = func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error) {
			return pm, nil
		}
	}
}

// WithDefaultPackageManager initializes a new package manager based on the
// umbrella package manager using host-provided configuration.
func WithDefaultPackageManager() CliOption {
	return func(copts *CliOptions) {
		if copts.packageManager != nil {
			return
		}

		copts.packageManager = func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error) {
			options, err := packmanager.NewPackageManagerOptions()
			if err != nil {
				return nil, err
			}

			return packmanager.NewUmbrellaManagerFromOptions(options)
		}
	}
}

// WithPluginManager sets a previously instantiated plugin manager to be used
// withthe command.
func WithPluginManager(pm *plugins.PluginManager) CliOption {
	return func(copts *CliOptions) {
		copts.pluginManager = func() (*plugins.PluginManager, error) {
			return pm, nil
		}
	}
}

// WithDefaultPluginManager returns an initialized plugin manager using the
// host-provided configuration plugin path.
func WithDefaultPluginManager() CliOption {
	return func(copts *CliOptions) {
		if copts.pluginManager != nil {
			return
		}

		copts.pluginManager = func() (*plugins.PluginManager, error) {
			cfgm, err := copts.configManager()
			if err != nil {
				return nil, err
			}

			return plugins.NewPluginManager(cfgm.Config.Paths.Plugins, nil), nil
		}
	}
}
