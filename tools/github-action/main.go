// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/bootstrap"
	"kraftkit.sh/log"
	"kraftkit.sh/manifest"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"

	_ "kraftkit.sh/manifest"
	_ "kraftkit.sh/oci"
)

type GithubAction struct {
	// Input arguments for the action
	// Global flags
	Loglevel   string `long:"loglevel" env:"INPUT_LOGLEVEL" usage:"" default:"info"`
	RuntimeDir string `long:"runtimedir" env:"INPUT_RUNTIMEDIR" usage:"Path to store runtime artifacts"`
	Auths      string `long:"auths" env:"INPUT_AUTHS" usage:"Authentication details for services"`
	Manifests  string `long:"manifests" env:"INPUT_MANIFESTS" usage:"List of Unikraft source manifests"`

	// Project flags
	Workdir   string `long:"workdir" env:"INPUT_WORKDIR" usage:"Path to working directory (default is cwd)"`
	Kraftfile string `long:"kraftfile" env:"INPUT_KRAFTFILE" usage:"Path to Kraftfile or contents of Kraftfile"`

	// Build flags
	Arch          string `long:"arch" env:"INPUT_ARCH" usage:"Architecture to build for"`
	Build         bool   `long:"build" env:"INPUT_BUILD" usage:"Toggle building the unikernel"`
	GitCloneDepth int    `long:"git_clone_depth" env:"INPUT_GIT_CLONE_DEPTH" usage:"Depth of the Git clone"`
	ForceGit      bool   `long:"force_git" env:"INPUT_FORCE_GIT" usage:"Use Git when pulling sources"`
	Plat          string `long:"plat" env:"INPUT_PLAT" usage:"Platform to build for"`
	Target        string `long:"target" env:"INPUT_TARGET" usage:"Name of the target to build for"`

	// Running flags
	Execute bool   `long:"execute" env:"INPUT_EXECUTE" usage:"If to run the unikernel"`
	Timeout uint64 `long:"timeout" env:"INPUT_TIMEOUT" usage:"Timeout for the unikernel"`

	// Packaging flags
	Args       string `long:"args" env:"INPUT_ARGS" usage:"Arguments to pass to the unikernel"`
	Rootfs     string `long:"rootfs" env:"INPUT_ROOTFS" usage:"Include a rootfs at path"`
	RootfsType string `long:"rootfs_type" env:"INPUT_ROOTFS_TYPE" usage:"Type of rootfs to build (cpio/erofs)" default:"cpio"`
	Memory     string `long:"memory" env:"INPUT_MEMORY" usage:"Set the memory size"`
	Name       string `long:"name" env:"INPUT_NAME" usage:"Set the name of the output"`
	Output     string `long:"output" env:"INPUT_OUTPUT" usage:"Set the output path"`
	Push       bool   `long:"push" env:"INPUT_PUSH" usage:"Push the output"`
	Strategy   string `long:"strategy" env:"INPUT_STRATEGY" usage:"Merge strategy to use when packaging"`
	Dbg        bool   `long:"dbg" env:"INPUT_DBG" usage:"Use the debug kernel"`

	// Internal attributes
	project    app.Application
	target     target.Target
	initrdPath string
}

func (opts *GithubAction) execScript(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return nil
	}

	cmd := exec.CommandContext(ctx, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = opts.Workdir
	cmd.Env = os.Environ()

	return cmd.Run()
}

func (opts *GithubAction) Run(ctx context.Context, args []string) (err error) {
	if opts.RuntimeDir != "" {
		config.G[config.KraftKit](ctx).RuntimeDir = opts.RuntimeDir
	}

	if opts.Auths != "" {
		var auths map[string]config.AuthConfig
		if err := yaml.Unmarshal([]byte(opts.Auths), &auths); err != nil {
			return fmt.Errorf("could not parse auths: %w", err)
		}

		if config.G[config.KraftKit](ctx).Auth == nil {
			config.G[config.KraftKit](ctx).Auth = make(map[string]config.AuthConfig)
		}

		for domain, auth := range auths {
			config.G[config.KraftKit](ctx).Auth[domain] = auth
		}
	}

	if opts.Manifests != "" {
		var manifests []string
		if err := yaml.Unmarshal([]byte(opts.Manifests), &manifests); err != nil {
			return fmt.Errorf("could not parse manifests: %w", err)
		}
		config.G[config.KraftKit](ctx).Unikraft.Manifests = manifests
	}

	// Save configuration to disk such that uses of `before`, `run` and `after`
	// scripts can access the configuration via `kraft`.
	if err := config.M[config.KraftKit](ctx).Write(true); err != nil {
		return fmt.Errorf("could not write configuration: %w", err)
	}

	if (len(opts.Arch) > 0 || len(opts.Plat) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("target and platform/architecture are mutually exclusive")
	}

	workspace := os.Getenv("GITHUB_WORKSPACE")
	if workspace == "" {
		workspace = "/github/workspace"
	}

	if err := opts.execScript(ctx, fmt.Sprintf("%s/.kraftkit/before.sh", workspace)); err != nil {
		log.G(ctx).Errorf("could not run before script: %v", err)
		os.Exit(1)
	}

	defer func() {
		// Run the after script even if errors have occurred.
		if err2 := opts.execScript(ctx, fmt.Sprintf("%s/.kraftkit/after.sh", workspace)); err2 != nil {
			err = errors.Join(err, fmt.Errorf("could not run after script: %v", err2))
		}
	}()

	switch opts.Loglevel {
	case "debug":
		log.G(ctx).SetLevel(logrus.DebugLevel)
	case "trace":
		log.G(ctx).SetLevel(logrus.TraceLevel)
	}

	if len(opts.Workdir) == 0 {
		opts.Workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	// If the `run` attribute has been set, only execute this.
	runScript := fmt.Sprintf("%s/.kraftkit/run.sh", workspace)
	if _, err := os.Stat(runScript); err == nil {
		return opts.execScript(ctx, runScript)
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.Workdir),
	}

	// Check if the provided Kraftfile is set, and whether it's either a path or
	// an inline file.
	if len(opts.Kraftfile) > 0 {
		if _, err := os.Stat(opts.Kraftfile); err == nil {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			// Dump the contents to a file
			fi, err := os.CreateTemp("", "*.Kraftfile")
			if err != nil {
				return fmt.Errorf("could not create temporary file for Kraftfile: %w", err)
			}

			defer fi.Close()

			n, err := fi.Write([]byte(opts.Kraftfile))
			if err != nil {
				return fmt.Errorf("could not write to temporary Kraftfile: %w", err)
			}

			if n != len(opts.Kraftfile) {
				return fmt.Errorf("could not write entire Kraftfile to %s", fi.Name())
			}

			popts = append(popts, app.WithProjectKraftfile(fi.Name()))
		}
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	if err := bootstrap.InitKraftkit(ctx); err != nil {
		return fmt.Errorf("could not init kraftkit: %v", err)
	}

	ctx, err = packmanager.WithDefaultUmbrellaManagerInContext(ctx)
	if err != nil {
		return fmt.Errorf("could not init package manager: %v", err)
	}

	// Initialize at least the configuration options for a project
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil && errors.Is(err, app.ErrNoKraftfile) {
		return fmt.Errorf("cannot build project directory without a Kraftfile")
	} else if err != nil {
		return fmt.Errorf("could not initialize project directory: %w", err)
	}

	manifest.ForceGit = opts.ForceGit
	if opts.GitCloneDepth > 0 {
		manifest.GitCloneDepth = opts.GitCloneDepth
	}

	if opts.project.Template() != nil {
		// All the components of a Unikraft unikernel build begin as remote packages
		// which need fetch.  The first package which we need to fetch is the template
		// which will be later merged into the current project.  The `Catalog` method
		// will search for the package in the package manager.
		packages, err := packmanager.G(ctx).Catalog(ctx,
			packmanager.WithName(opts.project.Template().Name()),
			packmanager.WithTypes(opts.project.Template().Type()),
			packmanager.WithVersion(opts.project.Template().Version()),
			packmanager.WithSource(opts.project.Template().Source()),
			packmanager.WithRemote(true),
		)
		if err != nil {
			return fmt.Errorf("could not fetch project template: %w", err)
		}
		if len(packages) == 0 {
			return fmt.Errorf("could not find template '%s'", opts.project.Template().Name())
		}

		// Pull all the template's packages into the project's working directory's
		// build directory.
		for _, p := range packages {
			log.G(ctx).WithField("name", p.Name()).Info("pulling package")
			if err := p.Pull(ctx,
				pack.WithPullWorkdir(opts.Workdir),
			); err != nil {
				return fmt.Errorf("could not pull package %s: %w", p.Name(), err)
			}
		}

		// Now that the template has bes been fetched, we must merge it with the
		// current project.  Start by instating a new project from the template.
		templateProject, err := app.NewProjectFromOptions(ctx,
			app.WithProjectWorkdir(opts.project.Template().Path()),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return fmt.Errorf("could not initialize template project: %w", err)
		}

		// Now merge the template project with the current project.
		opts.project, err = opts.project.MergeTemplate(ctx, templateProject)
		if err != nil {
			return fmt.Errorf("could not merge template project: %w", err)
		}
	}

	// Filter project targets by any provided input arguments
	targets := target.Filter(
		opts.project.Targets(),
		opts.Arch,
		opts.Plat,
		opts.Target,
	)

	if len(targets) > 1 {
		// TODO(nderjung): We should support building multiple targets in the
		// future, but for now we disable this ability.  This is largely to do with
		// package management afterwards which does not yet support multi-target
		// artifacts.  Once this is supported, we can enable multiple target-builds
		// (and packaging).  Moreover, since it is possible to also execute the
		// unikernel after a successful build via this action, multiple targets
		// would also fail at this step.
		return fmt.Errorf("cannot build more than one target using action")
	} else if len(targets) == 0 {
		return fmt.Errorf("no targets found")
	}

	opts.target = targets[0]

	// Infer arguments implicitly if there is only one target.  If we've made it
	// this far, `target.Filter` only had one target to choose from.
	if opts.Plat == "" {
		opts.Plat = opts.target.Platform().Name()
	}
	if opts.Arch == "" {
		opts.Arch = opts.target.Architecture().Name()
	}

	if opts.Strategy != "" {
		found := false
		var strategies []string
		for _, strategy := range packmanager.MergeStrategies() {
			strategies = append(strategies, strategy.String())
			if strategy.String() == opts.Strategy {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("unknown merge strategy '%s': choice from %v", opts.Strategy, strategies)
		}
	} else {
		opts.Strategy = packmanager.StrategyMerge.String()
	}

	if err := opts.pull(ctx); err != nil {
		return fmt.Errorf("could not pull project components: %w", err)
	}

	if opts.Build {
		if err := opts.build(ctx); err != nil {
			return fmt.Errorf("could not build unikernel: %w", err)
		}
	}

	if opts.Execute {
		if err := opts.execute(ctx); err != nil {
			return fmt.Errorf("could not run unikernel: %w", err)
		}
	}

	if opts.Output != "" {
		if err := opts.packAndPush(ctx); err != nil {
			return fmt.Errorf("could not package unikernel: %w", err)
		}
	}

	workspace = os.Getenv("GITHUB_WORKSPACE")
	if workspace == "" {
		workspace = "/github/workspace"
	}

	return nil
}

func main() {
	cmd, err := cmdfactory.New(&GithubAction{}, cobra.Command{})
	if err != nil {
		fmt.Printf("prepare command: %s", err)
		os.Exit(1)
	}

	ctx := signals.SetupSignalContext()

	cfg, err := config.NewDefaultKraftKitConfig()
	if err != nil {
		fmt.Printf("could not prepare internal configuration: %s", err)
		os.Exit(1)
	}

	cfgm, err := config.NewConfigManager(
		cfg,
		config.WithFile[config.KraftKit](config.DefaultConfigFile(), true),
	)
	if err != nil {
		fmt.Printf("could initialize config manager: %s", err)
		os.Exit(1)
	}

	// Set up the config manager in the context if it is available
	ctx = config.WithConfigManager(ctx, cfgm)

	// Attempt to set Unikraft Cloud config if possible
	if newCtx, err := config.HydrateKraftCloudAuthInContext(ctx); err == nil {
		ctx = newCtx
	}

	cmd, args, err := cmd.Find(os.Args[1:])
	if err != nil {
		fmt.Printf("could not find flag: %s", err)
		os.Exit(1)
	}

	if err := cmdfactory.AttributeFlags(cmd, cfg, args...); err != nil {
		fmt.Printf("could not attribute flags: %s", err)
		os.Exit(1)
	}

	// Set up a default logger based on the internal TextFormatter
	logger := logrus.New()

	formatter := new(log.TextFormatter)
	formatter.ForceColors = true
	formatter.ForceFormatting = true
	formatter.FullTimestamp = true
	formatter.DisableTimestamp = true
	logger.Formatter = formatter
	logger.Level, err = logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		logger.Level = logrus.InfoLevel
	}

	// Set up the logger in the context if it is available
	ctx = log.WithLogger(ctx, logger)

	os.Exit(cmdfactory.Main(ctx, cmd))
}
