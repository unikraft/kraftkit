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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"
	"strings"
	"encoding/json"
	"net/http"

	"kraftkit.sh/config"

	"github.com/spf13/cobra"
	"github.com/MakeNowJust/heredoc"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"kraftkit.sh/cmd/kraft/build"
	"kraftkit.sh/cmd/kraft/events"
	"kraftkit.sh/cmd/kraft/pkg"
	"kraftkit.sh/cmd/kraft/ps"
	"kraftkit.sh/cmd/kraft/rm"
	"kraftkit.sh/cmd/kraft/run"
	"kraftkit.sh/cmd/kraft/stop"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"

	// Additional initializers
	_ "kraftkit.sh/manifest"
	"kraftkit.sh/internal/version"
)

// The releases structure contains the relevant fields
// from a github API release query response
type releases []struct {
	HtmlUrl string `json:"html_url"`
	Id int `json:"id"`
	IsPreRelease bool `json:"prerelease"`
	Assets Assets `json:"assets"`
	Name string `json:"name"`
}

type Assets []struct {
	Name string `json:"name"`
	Url string `json:"browser_download_url"`
}

type kraftOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command line arguments
	checkPreRelease		bool
}

func main() {
	f := cmdfactory.New(
		cmdfactory.WithPackageManager(),
	)
	cmd, err := cmdutil.NewCmd(f, "kraft",
		cmdutil.WithSubcmds(
			pkg.PkgCmd(f),
			build.BuildCmd(f),
			ps.PsCmd(f),
			rm.RemoveCmd(f),
			run.RunCmd(f),
			stop.StopCmd(f),
			events.EventsCmd(f),
		),
	)
	if err != nil {
		panic("could not initialize root commmand")
	}

	opts := &kraftOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd.Short = "Build and use highly customized and ultra-lightweight unikernels"
	cmd.Long = heredoc.Docf(`

       .
      /^\     Build and use highly customized and ultra-lightweight unikernels.
     :[ ]:    
     | = |
    /|/=\|\   Documentation:    https://kraftkit.sh/
   (_:| |:_)  Issues & support: https://github.com/unikraft/kraftkit/issues
      v v 
      ' '`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return kraftRun(opts, f)
	}

	cmd.Flags().BoolVar(
		&opts.checkPreRelease,
		"check-prerelease",
		false,
		"Check for an available prerelease",
	)

	os.Exit(int(cmdutil.Execute(f, cmd)))
}

func kraftRun(opts *kraftOptions, cmdFactory *cmdfactory.Factory) error {
	var err error

	klog, err := opts.Logger()
	if err != nil {
		return err
	}


	// Check for a new prerelease
	if opts.checkPreRelease {
		updateAvail, err := checkForUpdate(cmdFactory, true)

		if err != nil {
			fmt.Errorf("Could not check for new prereleases: %s\n", err)
		} else if updateAvail {
			klog.Info("New version available!")
		}
	} else {

		// If the check for prerelease option is not given, check for an update
		updateAvail, err := checkForUpdate(cmdFactory, false)

		if err != nil {
			fmt.Errorf("Could not check for new releases: %s\n", err)
		} else if updateAvail {
			klog.Warn("New version available!")
		}
	}

	return nil
}

// Check for an update, return true is an update is available
// If the checkPrerelease param is true, check for a prerelease,
// else check for a release
func checkForUpdate(cmdFactory *cmdfactory.Factory, checkPrerelease bool) (bool, error) {
	cfgm, err := cmdFactory.ConfigManager()
	if err != nil {
		return false, err
	}

	var token string
	token = cfgm.Config.Auth["github"].Token

	resp, err := makeReleasesReq(token)
	if err != nil {
		return false, err
	}

	var rel_arr releases
	err = json.Unmarshal([]byte(resp), &rel_arr)
	if err != nil {
		return false, err
	}

	var index int
	if checkPrerelease {
		index = getLatestPreRelease(rel_arr)
	} else {
		index = getLatestRelease(rel_arr)
	}

	if index == -1 {
		return false, nil
	}

	if !strings.Contains(version.Version(), rel_arr[index].Name) {
		return true, nil
	}
	return false, nil
}

// Make a request to the github API to get an array of
// all available releases for the kraftkit repo
func makeReleasesReq(token string) (string, error) {
	timeout := time.Duration(5 + time.Second)
	client := http.Client {
		Timeout: timeout,
	}

	req, err := http.NewRequest("GET",
		"https://api.github.com/repos/unikraft/kraftkit/releases",
		nil)

	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	if token != "" {
		req.Header.Set("Authorization", "token " + token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// Get index of the first release in array
func getLatestRelease(releaseArray releases) int {
	for i := range releaseArray {
		if !releaseArray[i].IsPreRelease {
			return i
		}
	}

	return -1
}

// Get index of the first prerelease in array
func getLatestPreRelease(releaseArray releases) int {
	for i := range releaseArray {
		if releaseArray[i].IsPreRelease {
			return i
		}
	}

	return -1
}
