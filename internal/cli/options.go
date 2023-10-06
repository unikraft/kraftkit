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
func WithDefaultLogger(cfg *config.KraftKit) CliOption {
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

		switch log.LoggerTypeFromString(cfg.Log.Type) {
		case log.QUIET:
			formatter := new(logrus.TextFormatter)
			logger.Formatter = formatter

		case log.BASIC:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if cfg.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.FANCY:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if cfg.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.JSON:
			formatter := new(logrus.JSONFormatter)
			formatter.DisableTimestamp = true

			if cfg.Log.Timestamps {
				formatter.DisableTimestamp = false
			}

			logger.Formatter = formatter
		}

		level, ok := log.Levels()[cfg.Log.Level]
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

		// Attribute all configuration flags and command-line argument values
		cmd, args, err := cmd.Find(os.Args[1:])
		if err != nil {
			return err
		}
		configDir := config.FetchConfigDirFromArgs(args)

		// Did the user specify a non-standard config directory?  The following
		// check is possible thanks to the attribution of flags to the config file.
		// If a flag specifies changing the config directory, we must
		// re-instantiate the ConfigManager with the configuration from that
		// directory.
		var cfgm *config.ConfigManager[config.KraftKit]
		if configDir != "" && configDir != config.ConfigDir() {
			cfgm, err = config.NewConfigManager(
				cfg,
				config.WithFile[config.KraftKit](filepath.Join(configDir, "config.yaml"), true),
			)
			if err != nil {
				return err
			}
		} else {
			cfgm, err = config.NewConfigManager(
				cfg,
				config.WithFile[config.KraftKit](config.DefaultConfigFile(), true),
			)
			if err != nil {
				return err
			}
		}

		if err := cmdfactory.AttributeFlags(cmd, cfg, args...); err != nil {
			return err
		}

		copts.ConfigManager = cfgm

		return nil
	}
}

func WithConfigManager(cmd *cobra.Command, cfgMgr *config.ConfigManager[config.KraftKit]) CliOption {
	return func(copts *CliOptions) error {
		// Attribute all configuration flags and command-line argument values
		// TODO(jake-ciolek): Passing the args externally or relying on the injected cmd
		//                    breaks flag attribution. So we do the cmd, args dance here again.
		//                    Figure out why.
		cmd, args, err := cmd.Find(os.Args[1:])
		if err != nil {
			return err
		}

		if err := cmdfactory.AttributeFlags(cmd, cfgMgr.Config, args...); err != nil {
			return err
		}
		copts.ConfigManager = cfgMgr

		return nil
	}
}

func ConfigManagerFromArgs(args []string) (*config.ConfigManager[config.KraftKit], error) {
	cfg, err := config.NewDefaultKraftKitConfig()
	if err != nil {
		return nil, err
	}

	configDir := config.FetchConfigDirFromArgs(args)

	// Did the user specify a non-standard config directory?  The following
	// check is possible thanks to the attribution of flags to the config file.
	// If a flag specifies changing the config directory, we must
	// re-instantiate the ConfigManager with the configuration from that
	// directory.
	var cfgm *config.ConfigManager[config.KraftKit]
	if configDir != "" && configDir != config.ConfigDir() {
		cfgm, err = config.NewConfigManager(
			cfg,
			config.WithFile[config.KraftKit](filepath.Join(configDir, "config.yaml"), true),
		)
		if err != nil {
			return nil, err
		}
	} else {
		cfgm, err = config.NewConfigManager(
			cfg,
			config.WithFile[config.KraftKit](config.DefaultConfigFile(), true),
		)
		if err != nil {
			return nil, err
		}
	}

	return cfgm, nil
}

// WithDefaultIOStreams instantiates ta new IO streams using environmental
// variables and host-provided configuration.
func WithDefaultIOStreams(cfg *config.KraftKit) CliOption {
	return func(copts *CliOptions) error {
		if copts.IOStreams != nil {
			return nil
		}

		io := iostreams.System()

		if cfg != nil {
			if cfg.NoPrompt {
				io.SetNeverPrompt(true)
			}

			if pager := cfg.Pager; pager != "" {
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
func WithDefaultHTTPClient(cfg *config.KraftKit) CliOption {
	return func(copts *CliOptions) error {
		if copts.HTTPClient != nil {
			return nil
		}

		if copts.IOStreams == nil {
			return fmt.Errorf("cannot access IO streams")
		}

		httpClient, err := httpclient.NewHTTPClient(
			copts.IOStreams,
			cfg.HTTPUnixSocket,
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
func WithDefaultPluginManager(cfg *config.KraftKit) CliOption {
	return func(copts *CliOptions) error {
		if copts.PluginManager != nil {
			return nil
		}

		if cfg == nil {
			return fmt.Errorf("cannot access config")
		}

		copts.PluginManager = plugins.NewPluginManager(cfg.Paths.Plugins, nil)

		return nil
	}
}
