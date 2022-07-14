// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.
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

package cmdutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/MakeNowJust/heredoc"
	"github.com/cli/safeexec"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.unikraft.io/kit/pkg/iostreams"

	"go.unikraft.io/kit/config"
	"go.unikraft.io/kit/internal/cmdfactory"
	"go.unikraft.io/kit/internal/errs"
	"go.unikraft.io/kit/internal/run"
	"go.unikraft.io/kit/internal/version"

	versionCmd "go.unikraft.io/kit/internal/cmd/version"
)

type CmdOption func(*cobra.Command)

var flagOverrides = make(map[string][]*pflag.Flag)

// NewCmd generates a template `*cobra.Command` with sensible defaults and
// ensures consistency between all binaries within KraftKit.
func NewCmd(cmdFactory *cmdfactory.Factory, cmdName string, opts ...CmdOption) (*cobra.Command, error) {
	cmd := &cobra.Command{}

	// Attach this command if not set
	if cmdFactory.RootCmd == nil {
		cmdFactory.RootCmd = cmd
	}

	cmd.Use = fmt.Sprintf("%s [SUBCOMMAND] [FLAGS]", cmdName)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.DisableFlagsInUseLine = true
	cmd.Annotations = map[string]string{
		"help:environment": heredoc.Docf(`
			See '%s help environment' for the list of supported environment variables.
		`, cmdFactory.RootCmd.Name()),
	}

	cmd.SetOut(cmdFactory.IOStreams.Out)
	cmd.SetErr(cmdFactory.IOStreams.ErrOut)

	cmd.PersistentFlags().Bool("help", false, "Show help for command")
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		rootHelpFunc(cmdFactory, cmd, args)
	})

	cmd.SetUsageFunc(rootUsageFunc)
	cmd.SetFlagErrorFunc(rootFlagErrorFunc)

	formattedVersion := versionCmd.Format(version.Version(), version.BuildTime())
	cmd.SetVersionTemplate(formattedVersion)
	cmd.Version = formattedVersion

	cmd.Flags().Bool("version", false, fmt.Sprintf(
		"Show %s version", cmdFactory.RootCmd.Name(),
	))

	// TODO: Group these flags together.
	// See: https://github.com/spf13/cobra/issues/1327 for implementation example.
	cmd.PersistentFlags().Bool("no-prompt", false, "Do not prompt for interaction (assumes no)")
	cmd.PersistentFlags().Bool("yes", false, "Automatically approve any yes/no prompts during execution")
	cmd.PersistentFlags().Bool("show-timestamps", false, "Automatically approve any yes/no prompts during execution")
	cmd.PersistentFlags().Var(
		NewEnumFlag(config.AllowedValues("log.level"), config.Default("log.level")),
		"log-level",
		"Set the log verbosity level",
	)
	cmd.PersistentFlags().Var(
		NewEnumFlag(config.AllowedValues("log.type"), config.Default("log.type")),
		"log-type",
		"Set the logger type",
	)

	pm, err := cmdFactory.PluginManager()
	if err != nil {
		return nil, fmt.Errorf("could not access package manager: %v", err)
	}

	// provide completions for aliases and extensions
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var results []string
		plugins, err := pm.List()
		if err == nil {
			for _, plugin := range plugins {
				for _, aliasName := range plugin.Aliases() {
					if strings.HasPrefix(aliasName, toComplete) {
						results = append(results, aliasName)
					}
				}
			}
		}

		return results, cobra.ShellCompDirectiveNoFileComp
	}

	// Iterate through each option
	for _, opt := range opts {
		// Call the option giving the instantiated *cobra.Command as the argument
		opt(cmd)
	}

	// TODO: Authentication

	return cmd, nil
}

// WithSubcmds iterates over a list of instantiated *cobra.Command and adds
// to the parent.
func WithSubcmds(subCmds ...*cobra.Command) CmdOption {
	return func(cmd *cobra.Command) {
		for _, subCmd := range subCmds {
			cmd.AddCommand(subCmd)
		}
	}
}

// HasCommand returns true if args resolve to a built-in command
func HasCommand(cmd *cobra.Command, args []string) bool {
	c, _, err := cmd.Traverse(args)
	return err == nil && c != cmd
}

func Execute(cmdFactory *cmdfactory.Factory, cmd *cobra.Command) errs.ExitCode {
	hasDebug := os.Getenv("DEBUG") != ""
	stderr := cmdFactory.IOStreams.ErrOut

	cfgm, err := cmdFactory.ConfigManager()
	if err != nil {
		fmt.Fprintf(stderr, "could not access config manager: %v\n", err)
		return errs.ExitError
	}

	pm, err := cmdFactory.PluginManager()
	if err != nil {
		fmt.Fprintf(stderr, "could not access package manager: %v\n", err)
		return errs.ExitError
	}

	expandedArgs := []string{}
	if len(os.Args) > 0 {
		expandedArgs = os.Args[1:]
	}

	// Dynamically introduce functionality provided by all installed plugins into
	// the runtime of the root command, allowing any plugin to extend any
	// functionality made available through generic providers delivered by any
	// KraftKit package.  `Dispatch`ing the plugins will invoke the `init` program
	// within the plugin.
	if err := pm.Dispatch(); err != nil {
		fmt.Fprintf(stderr, "failed to connect to plugin manager: %s", err)
		return errs.ExitError
	}

	// Add flag overrides which can be provided by plugins
	for arg, flags := range flagOverrides {
		args := strings.Fields(arg)
		subCmd, _, err := cmd.Traverse(args[1:])
		if args[0] == cmd.Name() && err == nil {
			for _, flag := range flags {
				subCmd.Flags().AddFlag(flag)
			}
		}
	}

	// translate `{binname} help <command>` to `{binname} <command> --help` for
	// extensions
	if len(expandedArgs) == 2 && expandedArgs[0] == "help" && !HasCommand(cmd, expandedArgs[1:]) {
		expandedArgs = []string{expandedArgs[1], "--help"}
	}

	if !HasCommand(cmd, expandedArgs) {
		originalArgs := expandedArgs
		isShell := false

		argsForExpansion := append([]string{cmd.Name()}, expandedArgs...)
		expandedArgs, isShell, err := expand.ExpandAlias(cfgm.Config, argsForExpansion, nil)
		if err != nil {
			fmt.Fprintf(stderr, "failed to process aliases:  %s\n", err)
			return errs.ExitError
		}

		if hasDebug {
			fmt.Fprintf(stderr, "%v -> %v\n", originalArgs, expandedArgs)
		}

		if isShell {
			exe, err := safeexec.LookPath(expandedArgs[0])
			if err != nil {
				fmt.Fprintf(stderr, "failed to run external command: %s", err)
				return errs.ExitError
			}

			externalCmd := exec.Command(exe, expandedArgs[1:]...)
			externalCmd.Stderr = os.Stderr
			externalCmd.Stdout = os.Stdout
			externalCmd.Stdin = os.Stdin
			preparedCmd := run.PrepareCmd(externalCmd)

			err = preparedCmd.Run()
			if err != nil {
				var execError *exec.ExitError
				if errors.As(err, &execError) {
					return errs.ExitCode(execError.ExitCode())
				}

				fmt.Fprintf(stderr, "failed to run external command: %s\n", err)
				return errs.ExitError
			}

			return errs.ExitOK
		}
	}

	// Pre-emptively parse flags so we can update the configuration.  The caveat
	// of setting this up here is that additional logging may occur prior to this
	// moment.  It won't be much since it is just internal system and not the
	// functionality provided by the command itself.
	// Additional note: even though we run `cfg.Set`, we do not write these
	// values, so they are ephemeral incarnations of a CLI-first priority.
	if _, _, err := cmd.Traverse(expandedArgs); err != nil {
		fmt.Fprintf(stderr, "failed to parse command-line arguments: %s\n", err)
		return errs.ExitError
	}

	// TODO: Iterate overr all all `config.Config` and create flags for each into
	// a "global" flags set available across all commands and sub-commands.

	logLevel := cmd.PersistentFlags().Lookup("log-level").Value.String()
	if logLevel != "" {
		cfgm.Config.Log.Level = logLevel
	}

	logType := cmd.PersistentFlags().Lookup("log-type").Value.String()
	if logType != "" {
		cfgm.Config.Log.Type = logType
	}

	if cmd.PersistentFlags().Lookup("show-timestamps").Value.String() == "true" {
		cfgm.Config.Log.Timestamps = true

		// Use basic logger type if requesting --show-timestamps
		cfgm.Config.Log.Level = "basic"
	}

	if cmd.PersistentFlags().Lookup("no-prompt").Value.String() == "true" {
		cfgm.Config.NoPrompt = true
	}

	authError := errors.New("authError")

	// Because new CLI options can be dynamically injected, inject all (expanded)
	// arguments once more before executing the command.
	cmd.SetArgs(expandedArgs)

	if cmd, err := cmd.ExecuteC(); err != nil {
		var pagerPipeError *iostreams.ErrClosedPagerPipe
		if err == ErrSilent {
			return errs.ExitError
		} else if IsUserCancellation(err) {
			if errors.Is(err, terminal.InterruptErr) {
				// ensure the next shell prompt will start on its own line
				fmt.Fprint(stderr, "\n")
			}
			return errs.ExitCancel
		} else if errors.Is(err, authError) {
			return errs.ExitAuth
		} else if errors.As(err, &pagerPipeError) {
			// ignore the error raised when piping to a closed pager
			return errs.ExitOK
		}

		printError(stderr, err, cmd, hasDebug)

		if strings.Contains(err.Error(), "Incorrect function") {
			fmt.Fprintln(stderr, "You appear to be running in MinTTY without pseudo terminal support.")
			fmt.Fprintf(stderr, "To learn about workarounds for this error, run: %s help mintty\n", cmdFactory.RootCmd.Name())
			return errs.ExitError
		}

		return errs.ExitError
	}

	if HasFailed() {
		return errs.ExitError
	}

	return errs.ExitOK
}

func printError(out io.Writer, err error, cmd *cobra.Command, debug bool) {
	fmt.Fprintln(out, err)

	var flagError *FlagError
	if errors.As(err, &flagError) || strings.HasPrefix(err.Error(), "unknown command ") {
		if !strings.HasSuffix(err.Error(), "\n") {
			fmt.Fprintln(out)
		}
		fmt.Fprintln(out, cmd.UsageString())
	}
}

func RegisterFlag(cmdline string, flag *pflag.Flag) {
	flagOverrides[cmdline] = append(flagOverrides[cmdline], flag)
}
