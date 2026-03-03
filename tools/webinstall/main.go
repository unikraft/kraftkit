// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/log"
)

//go:embed install.sh
var installScript string

const (
	DefaultScriptPath       = "/"
	DefaultLatestPath       = "/latest.txt"
	DefaultLatestGitHubAPI  = "https://api.github.com/repos/unikraft/kraftkit/releases/latest"
	DefaultStagingPath      = "/staging.txt"
	DefaultStagingGitHubAPI = "https://api.github.com/repos/unikraft/kraftkit/tags?per_page=100"
	DefaultChecksumURLFmt   = "https://github.com/unikraft/kraftkit/releases/download/v%s/kraftkit_%s_checksums.txt"
	DefaultPort             = 8080
	DefaultFreq             = 24 * time.Hour
)

type Webinstall struct {
	Freq     time.Duration `long:"freq" short:"F" usage:"The frequency (in hours) to check for updates" env:"WEBINSTALL_FREQ" default:"24h"`
	Port     int           `long:"port" short:"P" usage:"The port to serve the script" env:"WEBINSTALL_PORT" default:"8080"`
	Token    string        `long:"token" short:"T" usage:"The GitHub token for querying tags" env:"WEBINSTALL_TOKEN" default:""`
	LogLevel string        `long:"log-level" usage:"Set the log level verbosity" env:"WEBINSTALL_LOG_LEVEL" default:"info"`
}

func New() *cobra.Command {
	cmd, _ := cmdfactory.New(&Webinstall{}, cobra.Command{
		Short:                 `Serve a script to install kraftkit`,
		Use:                   "webinstall",
		Long:                  `Serve a script to install kraftkit that installs the correct packages`,
		DisableFlagsInUseLine: true,
		Example:               `webinstall -P 8080 -F 24`,
	})

	return cmd
}

func (opts *Webinstall) getKraftkitLatestVersion(ctx context.Context) (string, error) {
	log.G(ctx).Debug("checking for latest kraftkit version")

	// Create a request to github to get the latest release
	req, err := http.NewRequest("GET", DefaultLatestGitHubAPI, nil)
	if err != nil {
		return "", err
	}

	// Set headers to ensure we get the correct response
	req.Header.Set("Accept", "application/vnd.github+json")

	if opts.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", opts.Token))
	}

	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Send the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	// Read the response to a string
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	// Convert the json string to a map
	var result map[string]interface{}
	json.Unmarshal([]byte(body), &result)

	if message, ok := result["message"]; ok {
		return "", fmt.Errorf("error from GitHub API: %s", message)
	}

	if _, ok := result["tag_name"]; !ok {
		return "", fmt.Errorf("malformed GitHub API response, could not determine latest KraftKit version")
	}

	// Get the tag name from the map and remove the prepended 'v'
	return strings.TrimPrefix(result["tag_name"].(string), "v"), nil
}

func (opts *Webinstall) getKraftkitStagingVersion(ctx context.Context) (string, error) {
	log.G(ctx).Debug("checking for staging kraftkit version")

	// Create a request to github to get the latest release
	req, err := http.NewRequest("GET", DefaultStagingGitHubAPI, nil)
	if err != nil {
		return "", err
	}

	// Set headers to ensure we get the correct response
	req.Header.Set("Accept", "application/vnd.github+json")

	if opts.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", opts.Token))
	}

	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Send the request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	// Read the response to a string
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	// Convert the json string to a map
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return "", fmt.Errorf("could not parse GitHub API response: %w", err)
	}

	if len(result) == 0 {
		return "", fmt.Errorf("malformed GitHub API response, could not determine staging KraftKit")
	}

	for _, release := range result {
		v, ok := release["name"].(string)
		if !ok {
			log.G(ctx).
				WithField("error", err).
				Debug("malformed GitHub API response")
			continue
		}

		if !strings.HasPrefix(v, "v") || !strings.Contains(v, "-") {
			continue // Not a staging version, skip
		}

		version := strings.TrimPrefix(v, "v")

		// Check if this version has actual artifacts.
		// Create a request to github to get the release checksums.
		req, err := http.NewRequest("GET", fmt.Sprintf(DefaultChecksumURLFmt, version, version), nil)
		if err != nil {
			log.G(ctx).
				WithField("error", err).
				Debug("could not create request")
			continue
		}

		if opts.Token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", opts.Token))
		}

		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		// Send the request
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.G(ctx).
				WithField("error", err).
				Debug("performing the request")
			continue
		}

		if res.StatusCode != 200 {
			log.G(ctx).
				WithField("url", fmt.Sprintf(DefaultChecksumURLFmt, version, version)).
				WithField("code", res.StatusCode).
				Debug("tag does not have release")
			time.Sleep(30 * time.Second) // Sleep a bit to prevent spamming GitHub.
			continue
		}

		return version, nil
	}

	return "", fmt.Errorf("malformed GitHub API response, could not determine staging KraftKit version")
}

// doRootCmd starts the main system
func (opts *Webinstall) Run(ctx context.Context, args []string) error {
	// Set the defaults if empty
	if opts.Freq == 0 {
		opts.Freq = DefaultFreq
	}

	if opts.Port == 0 {
		opts.Port = DefaultPort
	}

	// Configure the log level
	logger := logrus.New()
	switch opts.LogLevel {
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	}

	ctx = log.WithLogger(ctx, logger)

	latestVersion, err := opts.getKraftkitLatestVersion(ctx)
	if err != nil {
		return err
	}
	stagingVersion, err := opts.getKraftkitStagingVersion(ctx)
	if err != nil {
		return err
	}

	// Get a time modified for the installScript
	nowScript := time.Now()

	// Get a time modified for the kraftkit version
	nowVersion := time.Now()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.G(ctx).Debug("context cancelled")
				return
			case <-time.After(opts.Freq):
			}

			latestVersion, err = opts.getKraftkitLatestVersion(ctx)
			if err != nil {
				log.G(ctx).Errorf("could not retrieve latest version: %v", err)
				continue
			}

			stagingVersion, err = opts.getKraftkitStagingVersion(ctx)
			if err != nil {
				log.G(ctx).Errorf("could not retrieve latest version: %v", err)
				continue
			}

			nowVersion = time.Now()
		}
	}()

	http.HandleFunc(DefaultScriptPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, DefaultScriptPath, nowScript, strings.NewReader(installScript))
	})

	http.HandleFunc(DefaultLatestPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, DefaultLatestPath, nowVersion, strings.NewReader(latestVersion))
	})

	http.HandleFunc(DefaultStagingPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, DefaultStagingPath, nowVersion, strings.NewReader(stagingVersion))
	})

	log.G(ctx).Infof("Listening on :%d...\n", opts.Port)

	// Start listening and serve the data
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil); err != nil {
			log.G(ctx).Error(err)
		}
	}()

	<-ctx.Done()

	return nil
}

func main() {
	cmdfactory.Main(signals.SetupSignalContext(), New())
}
