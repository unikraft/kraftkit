// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"fmt"
	"net/http"
	"os"

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
	ioStreams      *iostreams.IOStreams
	logger         *logrus.Entry
	configManager  *config.ConfigManager
	packageManager packmanager.PackageManager
	pluginManager  *plugins.PluginManager
	httpClient     *http.Client
}

type CliOption func(*CliOptions) error

// withDefaultLogger sets up the built in logger based on provided conifg found
// from the ConfigManager.
func withDefaultLogger() CliOption {
	return func(copts *CliOptions) error {
		if copts.logger != nil {
			return nil
		}

		if copts.configManager == nil {
			copts.logger = log.L
			return nil
		}

		// Set up a default logger based on the internal TextFormatter
		logger := logrus.New()

		// Configure the logger based on parameters set by in KraftKit's
		// configuration
		if copts.configManager == nil {
			copts.logger = log.L
		}

		switch log.LoggerTypeFromString(copts.configManager.Config.Log.Type) {
		case log.QUIET:
			formatter := new(logrus.TextFormatter)
			logger.Formatter = formatter

		case log.BASIC:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if copts.configManager.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.FANCY:
			formatter := new(log.TextFormatter)
			formatter.FullTimestamp = true
			formatter.DisableTimestamp = true

			if copts.configManager.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			} else {
				formatter.TimestampFormat = ">"
			}

			logger.Formatter = formatter

		case log.JSON:
			formatter := new(logrus.JSONFormatter)
			formatter.DisableTimestamp = true

			if copts.configManager.Config.Log.Timestamps {
				formatter.DisableTimestamp = false
			}

			logger.Formatter = formatter
		}

		level, ok := log.Levels()[copts.configManager.Config.Log.Level]
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

		return nil
	}
}

// withDefaultConfigManager instantiates a configuration manager based on
// default options.
func withDefaultConfigManager(cmd *cobra.Command) CliOption {
	return func(copts *CliOptions) error {
		cfgm, err := config.NewConfigManager(
			config.WithFile(config.ConfigFile(), true),
		)
		if err != nil {
			return err
		}

		// Attribute all configuration flags and command-line argument values
		cmdfactory.AttributeFlags(cmd, cfgm.Config, os.Args...)

		copts.configManager = cfgm

		return nil
	}
}

// withDefaultIOStreams instantiates ta new IO streams using environmental
// variables and host-provided configuration.
func withDefaultIOStreams() CliOption {
	return func(copts *CliOptions) error {
		if copts.ioStreams != nil {
			return nil
		}

		io := iostreams.System()

		if copts.configManager != nil {
			if copts.configManager.Config.NoPrompt {
				io.SetNeverPrompt(true)
			}

			if pager := copts.configManager.Config.Pager; pager != "" {
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

		copts.ioStreams = io

		return nil
	}
}

// withHTTPClient sets a previously instantiated http.Client to be used within
// the command.
func withHTTPClient(httpClient *http.Client) CliOption {
	return func(copts *CliOptions) error {
		copts.httpClient = httpClient
		return nil
	}
}

// withDefaultHTTPClient initializes a HTTP client using host-provided
// configuration.
func withDefaultHTTPClient() CliOption {
	return func(copts *CliOptions) error {
		if copts.httpClient != nil {
			return nil
		}

		if copts.configManager == nil {
			return fmt.Errorf("cannot access config manager")
		}

		if copts.ioStreams == nil {
			return fmt.Errorf("cannot access IO streams")
		}

		httpClient, err := httpclient.NewHTTPClient(
			copts.ioStreams,
			copts.configManager.Config.HTTPUnixSocket,
			true,
		)
		if err != nil {
			return err
		}

		copts.httpClient = httpClient

		return nil
	}
}

// withDefaultPackageManager initializes a new package manager based on the
// umbrella package manager using host-provided configuration.
func withDefaultPackageManager() CliOption {
	return func(copts *CliOptions) error {
		if copts.packageManager != nil {
			return nil
		}

		// TODO: Add configuration option that allows us to statically set a
		// preferred package manager.
		copts.packageManager = packmanager.NewUmbrellaManager()

		return nil
	}
}

// withDefaultPluginManager returns an initialized plugin manager using the
// host-provided configuration plugin path.
func withDefaultPluginManager() CliOption {
	return func(copts *CliOptions) error {
		if copts.pluginManager != nil {
			return nil
		}

		if copts.configManager == nil {
			return fmt.Errorf("cannot access config manager")
		}

		copts.pluginManager = plugins.NewPluginManager(copts.configManager.Config.Paths.Plugins, nil)

		return nil
	}
}
