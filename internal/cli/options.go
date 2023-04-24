// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/plugins"

	"kraftkit.sh/internal/httpclient"
)

type CliOptions struct {
	IOStreams      *iostreams.IOStreams
	Logger         *logrus.Logger
	ConfigManager  *config.ConfigManager[config.KraftKit]
	PackageManager packmanager.PackageManager
	PluginManager  *plugins.PluginManager
	HTTPClient     *http.Client
}

type CliOption func(*CliOptions) error

// WithDefaultLogger sets up the built in logger based on provided conifg found
// from the ConfigManager.
func WithDefaultLogger() CliOption {
	return func(copts *CliOptions) error {
		if copts.Logger != nil {
			return nil
		}

		// Configure the logger based on parameters set by in KraftKit's
		// configuration
		if copts.ConfigManager == nil {
			copts.Logger = log.L
			return nil
		}

		// Set up a default logger based on the internal TextFormatter
		logger := logrus.New()

		switch log.LoggerTypeFromString(copts.ConfigManager.Config.Log.Type) {
		case log.QUIET:
			formatter := new(logrus.TextFormatter)
			logger.Formatter = formatter

		case log.BASIC:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if copts.ConfigManager.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.FANCY:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if copts.ConfigManager.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.JSON:
			formatter := new(logrus.JSONFormatter)
			formatter.DisableTimestamp = true

			if copts.ConfigManager.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			}

			logger.Formatter = formatter
		}

		level, ok := log.Levels()[copts.ConfigManager.Config.Log.Level]
		if !ok {
			logger.Level = logrus.InfoLevel
		} else {
			logger.Level = level
		}

		if copts.IOStreams != nil {
			logger.SetOutput(copts.IOStreams.Out)
		}

		// Save the logger
		copts.Logger = logger

		return nil
	}
}

// WithDefaultConfigManager instantiates a configuration manager based on
// default options.
func WithDefaultConfigManager(cmd *cobra.Command) CliOption {
	return func(copts *CliOptions) error {
		cfg, err := config.NewDefaultKraftKitConfig()
		if err != nil {
			return err
		}
		cfgm, err := config.NewConfigManager(
			cfg,
			config.WithFile[config.KraftKit](config.DefaultConfigFile(), true),
		)
		if err != nil {
			return err
		}

		// Attribute all configuration flags and command-line argument values
		cmdfactory.AttributeFlags(cmd, cfgm.Config, os.Args...)

		// Did the user specify a non-standard config directory?  The following
		// check is possible thanks to the attribution of flags to the config file.
		// If a flag specifies changing the config directory, we must
		// re-instantiate the ConfigManager with the configuration from that
		// directory.
		if cpath := cfg.Paths.Config; cpath != "" && cpath != config.ConfigDir() {
			cfgm, err = config.NewConfigManager(
				cfg,
				config.WithFile[config.KraftKit](filepath.Join(cpath, "config.yaml"), true),
			)
			if err != nil {
				return err
			}
		}

		copts.ConfigManager = cfgm

		return nil
	}
}

// WithDefaultIOStreams instantiates ta new IO streams using environmental
// variables and host-provided configuration.
func WithDefaultIOStreams() CliOption {
	return func(copts *CliOptions) error {
		if copts.IOStreams != nil {
			return nil
		}

		io := iostreams.System()

		if copts.ConfigManager != nil {
			if copts.ConfigManager.Config.NoPrompt {
				io.SetNeverPrompt(true)
			}

			if pager := copts.ConfigManager.Config.Pager; pager != "" {
				io.SetPager(pager)
			}
		}

		// Pager precedence
		// 1. KRAFTKIT_PAGER
		// 2. pager from config
		// 3. PAGER
		if kkPager, kkPagerExists := os.LookupEnv("KRAFTKIT_PAGER"); kkPagerExists {
			io.SetPager(kkPager)
		}

		copts.IOStreams = io

		return nil
	}
}

// WithHTTPClient sets a previously instantiated http.Client to be used within
// the command.
func WithHTTPClient(httpClient *http.Client) CliOption {
	return func(copts *CliOptions) error {
		copts.HTTPClient = httpClient
		return nil
	}
}

// WithDefaultHTTPClient initializes a HTTP client using host-provided
// configuration.
func WithDefaultHTTPClient() CliOption {
	return func(copts *CliOptions) error {
		if copts.HTTPClient != nil {
			return nil
		}

		if copts.ConfigManager == nil {
			return fmt.Errorf("cannot access config manager")
		}

		if copts.IOStreams == nil {
			return fmt.Errorf("cannot access IO streams")
		}

		httpClient, err := httpclient.NewHTTPClient(
			copts.IOStreams,
			copts.ConfigManager.Config.HTTPUnixSocket,
			true,
		)
		if err != nil {
			return err
		}

		copts.HTTPClient = httpClient

		return nil
	}
}

// WithDefaultPluginManager returns an initialized plugin manager using the
// host-provided configuration plugin path.
func WithDefaultPluginManager() CliOption {
	return func(copts *CliOptions) error {
		if copts.PluginManager != nil {
			return nil
		}

		if copts.ConfigManager == nil {
			return fmt.Errorf("cannot access config manager")
		}

		copts.PluginManager = plugins.NewPluginManager(copts.ConfigManager.Config.Paths.Plugins, nil)

		return nil
	}
}
