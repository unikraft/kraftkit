// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/tools/dockerfile-llb-frontend/build"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
)

type cmdConfig struct {
	LLBStdout    bool   // Controls whether we emit LLB to stdout or spawn a protobuf server.
	Platform     string // Maps to kraftkit's platform (qemu/xen/linuxu).
	Architecture string // Maps to kraftkit's architecture flag (x86_64/arm64).
}

func parseFlags() cmdConfig {
	var cfg cmdConfig

	flag.BoolVar(&cfg.LLBStdout, "llb-stdout", false, "output a LLB graph and exit")
	flag.StringVar(&cfg.Platform, "platform", build.DefaultPlatform, "specify the platform")
	flag.StringVar(&cfg.Architecture, "architecture", build.DefaultArch, "specify the architecture")
	flag.Parse()

	return cfg
}

// The plugin has two modes of operation.
//
// 1. Local.
//
// One can run the plugin from within a Unikraft app directory containing a kraft.yaml file.
//
// This is useful for debugging.
//
// 2. Remote/Docker front-end LLB plugin.
//
// The plugin can be build into a container image which then can be loaded via the special
// docker syntax. For example, for an image named kraftkit.sh/llb:
//
// #syntax=kraftkit.sh/llb:latest
//
// The image has to be available in the docker image list. If it's not, docker will try to download it.
// Running it as a plugin will make it spawn a protobuf server. The docker client will then communicate
// with the plugin over that protobuf connection.
func main() {
	cfg := parseFlags()
	ctx := appcontext.Context()

	if cfg.LLBStdout {
		err := graphToStdout(ctx, cfg.Platform, cfg.Architecture)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := grpcclient.RunFromEnvironment(ctx, build.WithDefaultBuildDriver()); err != nil {
		log.Fatal(err)
	}
}

// This emits a LLB graph.
//
// It can be piped into builtkitd like so:
//
// go run . --llb-stdout=true | buildctl debug dump-llb
//
// Similarly, a build can be invoked using that LLB graph with:
//
// go run . --llb-stdout=true | buildctl build --output type=docker,dest=helloworld_image --local context=./app-helloworld
func graphToStdout(ctx context.Context, platform, architecture string) error {
	buildConfig := build.DefaultBuildConfig()
	// Update the base config with values sourced from CLI flags.
	buildConfig.Platform = platform
	buildConfig.Architecture = architecture

	project, err := app.NewProjectFromOptions(ctx,
		app.WithProjectKraftfile(build.DefaultKraftfileName),
	)
	if err != nil {
		return fmt.Errorf("failed to build unikraft representation of kraftfile: %w", err)
	}

	// Source app name from the loaded kraftfile
	buildConfig.AppName = project.Name()

	buildResult := build.RunDefaultBuild(buildConfig)

	marshalledOutput, err := build.MarshalForPlatform(ctx, buildResult, llb.LinuxAmd64)
	if err != nil {
		return fmt.Errorf("failed to marshal the unikernel image's llb: %w", err)
	}

	err = llb.WriteTo(marshalledOutput, os.Stdout)
	if err != nil {
		return fmt.Errorf("failed to putput the graph to stdout: %w", err)
	}

	return nil
}
