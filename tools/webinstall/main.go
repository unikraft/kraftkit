// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rancher/wrangler/pkg/signals"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
)

//go:embed install.sh
var installScript string

const (
	DefaultScriptPath = "/"
	DefaultLatestPath = "/latest.txt"
)

type Webinstall struct {
	Freq  time.Duration `long:"freq"  short:"F" usage:"The frequency (in hours) to check for updates" env:"WEBINSTALL_FREQ" default:"24h"`
	Port  int           `long:"port"  short:"P" usage:"The port to serve the script" env:"WEBINSTALL_PORT" default:"8080"`
	Token string        `long:"token" short:"T" usage:"The GitHub token for querying tags" env:"WEBINSTALL_TOKEN" default:""`
}

func New() *cobra.Command {
	return cmdfactory.New(&Webinstall{}, cobra.Command{
		Short:                 `Serve a script to install kraftkit`,
		Use:                   "webinstall",
		Long:                  `Serve a script to install kraftkit that installs the correct packages`,
		DisableFlagsInUseLine: true,
		Example:               `webinstall -P 8080 -F 24`,
	})
}

func (opts *Webinstall) getKraftkitVersion() (string, error) {
	// Create a request to github to get the latest release
	req, err := http.NewRequest("GET", "https://api.github.com/repos/unikraft/kraftkit/releases/latest", nil)
	if err != nil {
		return "", err
	}

	// Set headers to ensure we get the correct response
	req.Header.Set("Accept", "application/vnd.github+json")

	if opts.Token != "" {
		req.Header.Set("Authorization", opts.Token)
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

	// Get the tag name from the map and remove the prepended 'v'
	return result["tag_name"].(string)[1:], err
}

// doRootCmd starts the main system
func (opts *Webinstall) Run(cmd *cobra.Command, args []string) error {
	// Set the defaults if empty
	if opts.Freq == 0 {
		opts.Freq = 24 * time.Hour
	}

	if opts.Port == 0 {
		opts.Port = 8080
	}

	// Create a reader for the installScript
	scriptReader := strings.NewReader(installScript)

	// Create a reader for the kraftkit version
	version, err := opts.getKraftkitVersion()
	if err != nil {
		return err
	}
	versionReader := strings.NewReader(version)

	// Get a time modified for the installScript
	nowScript := time.Now()

	// Get a time modified for the kraftkit version
	nowVersion := time.Now()

	go func() {
		for {
			time.Sleep(opts.Freq)
			version, err := opts.getKraftkitVersion()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			versionReader = strings.NewReader(version)
			nowVersion = time.Now()
		}
	}()

	// Serve the installScript
	http.HandleFunc(DefaultScriptPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "install.sh", nowScript, scriptReader)
	})

	// Serve the kraftkit version
	http.HandleFunc(DefaultLatestPath, func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "latest.txt", nowVersion, versionReader)
	})

	fmt.Printf("Listening on :%d...\n", opts.Port)

	// Start listening and serve the data
	http.ListenAndServe(fmt.Sprintf(":%d", opts.Port), nil)

	return nil
}

func main() {
	cmdfactory.Main(signals.SetupSignalContext(), New())
}
