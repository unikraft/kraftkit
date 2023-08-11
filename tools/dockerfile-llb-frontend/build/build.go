// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package build implements the core buildkit-based unikraft app build process.
package build

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/solver/result"
	"kraftkit.sh/tools/dockerfile-llb-frontend/image"
	"kraftkit.sh/unikraft/app"
)

// Configuration holds data used to control the build process.
type Configuration struct {
	ContextName       string // This identifies the context name passed from the BuildKit deamon, usually "context".
	KraftfileName     string // The filename of the kraftkit config. We'll look for it in the BuildKit context.
	KraftkitWorkspace string // Where in the interim container to place the app and perform the build.
	OutputDir         string // Where the built app can be found (in the build container).
	BaseKraftkitImage string // We use kraftkit's image as the base of our builds. This variable defines it.
	Platform          string // Target platform to build for (qemu/linuxu etc...).
	Architecture      string // Target architecture to build for (x86_64, arm64, etc...).
	AppName           string // Used to resolve the location of the build artifact.
}

const (
	// DefaultContextName is the key under which the client can reach state pushed from BuildKit's daemon.
	DefaultContextName = "context"
	// DefaultKraftfileName is where we source the app's config from.
	DefaultKraftfileName = "kraft.yaml"
	// DefaultPlatform defines the platform to build the image for by default.
	DefaultPlatform = "qemu"
	// DefaultArch defines the default CPU architecture.
	DefaultArch = "x86_64"
	// DefaultKraftkitWorkspace is the build directory we create on top of the base kraftkit image.
	DefaultKraftkitWorkspace = "/kraftkit-workspace"
	// KraftImageName is the name of the base kraftkit image.
	KraftImageName = "kraftkit.sh/base:latest"
	// DefaultOutputDir is the build image where the compiled unikernel gets placed.
	DefaultOutputDir = DefaultKraftkitWorkspace + "/.unikraft/build/"
)

// Driver is an interface defining functions necessary to drive a build.
type Driver interface {
	Init(client.Client, *Configuration) error
	Run(ctx context.Context) (*client.Result, error)
}

// DefaultBuildkitDriver handles building a unikraft app using buildkit.
type DefaultBuildkitDriver struct {
	Config *Configuration
	Client client.Client
	Solver BuildKitSolver
}

// BuildKitSolver defines a way to send request and receive BuildKit build responses.
type BuildKitSolver interface {
	Solve(context.Context, client.Client, *llb.Definition) (*result.Result[client.Reference], error)
}

// DefaultBuildkitSolver allows us to swap out the buildkit-talking part for tests.
type DefaultBuildkitSolver struct{}

// Solve sends the definition to buildkit and returns a result.
func (s DefaultBuildkitSolver) Solve(ctx context.Context, c client.Client, definition *llb.Definition) (*result.Result[client.Reference], error) {
	return c.Solve(ctx, client.SolveRequest{
		Definition: definition.ToPB(),
	})
}

// Init prepares the driver for work.
func (driver *DefaultBuildkitDriver) Init(client client.Client, config *Configuration) error {
	if client == nil {
		return fmt.Errorf("cannot initialize driver with a nil client")
	}
	if config == nil {
		return fmt.Errorf("cannot initialize driver with a nil configuration")
	}
	driver.Client = client
	driver.Config = config
	driver.Solver = DefaultBuildkitSolver{}

	return nil
}

// Run performs the build parametrized by the pre-initialized config.
func (driver *DefaultBuildkitDriver) Run(ctx context.Context) (*client.Result, error) {
	project, err := driver.getApplication(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed getting kraft config from BuildKit's context %w", err)
	}

	// Set the app name based on the resolved Application
	driver.Config.AppName = project.Name()

	buildOutput := RunDefaultBuild(*driver.Config)

	result, err := driver.sendToBuildkit(ctx, buildOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to execute build with buildkit: %w", err)
	}

	// We then annotate the result with additional metadata.
	// BuildKit will pick it up from the returned client.Result.
	// (todo) This section shall be expanded once we focus on the run part.
	return driver.annotateBuildResult(result)
}

func (driver *DefaultBuildkitDriver) sendToBuildkit(ctx context.Context, state *llb.State) (*result.Result[client.Reference], error) {
	// We marshal the state to send it back to BuildKit
	definition, err := MarshalForPlatform(ctx, state, llb.LinuxAmd64)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling plucked state: %w", err)
	}

	result, err := driver.Solver.Solve(ctx, driver.Client, definition)
	if err != nil {
		return nil, fmt.Errorf("failed to solve the copied unikernel image definition from %v: %w", definition, err)
	}

	return result, nil
}

func (driver DefaultBuildkitDriver) annotateBuildResult(result *result.Result[client.Reference]) (*result.Result[client.Reference], error) {
	ref, err := result.SingleRef()
	if err != nil {
		return nil, fmt.Errorf("failed to get the referece to BuildKit's result: %w", err)
	}

	result.SetRef(ref)

	config, err := json.Marshal(image.NewImageConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the base image config: %w", err)
	}

	// This annotates the image with values from the client's runtime.GOOS and runtime.GOARCH.
	platformSpec := platforms.Format(platforms.DefaultSpec())

	result.AddMeta(fmt.Sprintf("%s/%s", exptypes.ExporterImageConfigKey, platformSpec), config)

	return result, nil
}

// getApplication builds up a kraftkit Application description by requesting the kraft.yaml
// file from BuildKit.
func (driver *DefaultBuildkitDriver) getApplication(ctx context.Context) (app.Application, error) {
	src := llb.Local(driver.Config.ContextName,
		llb.IncludePatterns([]string{driver.Config.KraftfileName}),
		llb.WithCustomName("[internal] "+driver.Config.KraftfileName),
	)

	definition, err := src.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state into definition: %w", err)
	}

	result, err := driver.Solver.Solve(ctx, driver.Client, definition)
	if err != nil {
		return nil, fmt.Errorf("failed to solve client request from definition %v: %w", definition, err)
	}

	kraftFile, err := fileFromResult(ctx, result, driver.Config.KraftfileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read file from result: %w", err)
	}
	project, err := app.NewProjectFromOptions(ctx,
		app.WithProjectKraftfileFromBytes(kraftFile),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build unikraft representation of kraftfile: %w", err)
	}

	return project, nil
}

// WithDriver is the function muxed by the plugin's protobuf server.
// It returns a function that then builds the base unikraft image, containing the compiled app.
// Then we pluck the unikernel file and return it as a container image.
// There's some metadata added along the way.
// Allows parametrizing with a driver for easier testing.
func WithDriver(driver Driver, config Configuration) func(context.Context, client.Client) (*client.Result, error) {
	return func(ctx context.Context, c client.Client) (*client.Result, error) {
		// This is the core of our process.
		if err := driver.Init(c, &config); err != nil {
			return nil, fmt.Errorf("failed to initalize the driver: %w", err)
		}
		return driver.Run(ctx)
	}
}

// WithDefaultBuildDriver provides a sensible, ready-to-use default build driver.
func WithDefaultBuildDriver() func(context.Context, client.Client) (*client.Result, error) {
	return WithDriver(&DefaultBuildkitDriver{}, DefaultBuildConfig())
}

type step struct {
	name string
	fn   func(llb.State, Configuration) llb.State
}

var defaultSteps = [...]step{
	{name: "source kraftkit image", fn: sourceKraftkitImage},
	{name: "add unikraft app", fn: addUnikraftApp},
	{name: "run kraftkit build", fn: execKraftkitBuild},
	{name: "copy unikernel image", fn: copyUnikernelImage},
}

// RunDefaultBuild runs a build using sensible defaults.
func RunDefaultBuild(config Configuration) *llb.State {
	return runBuild(config, defaultSteps[:])
}

func runBuild(config Configuration, steps []step) *llb.State {
	// Start with a clean state.
	llb := llb.Scratch()

	for _, s := range steps {
		llb = s.fn(llb, config)
	}

	return &llb
}

func sourceKraftkitImage(_ llb.State, config Configuration) llb.State {
	return llb.Image(config.BaseKraftkitImage)
}

// This step copies over app files so that they can be used with kraftkit.
func addUnikraftApp(base llb.State, config Configuration) llb.State {
	local := llb.Local(config.ContextName)

	// Add unikraft app files
	return base.Dir("/").File(llb.Copy(local, "./", config.KraftkitWorkspace))
}

// Run the actual kraftkit build command.
func execKraftkitBuild(base llb.State, config Configuration) llb.State {
	return base.Dir(config.KraftkitWorkspace).
		Run(llb.Shlex(kraftkitBuildCommand(config.Platform, config.Architecture))).
		Root()
}

// We don't want kraftkit nor the base app files in the final image.
// We use this to pluck the desired output.
func copyUnikernelImage(base llb.State, config Configuration) llb.State {
	return llb.Scratch().
		File(llb.Copy(base, outputPath(config.AppName, config), outputName(config.AppName, config)))
}

func fileFromResult(ctx context.Context, result *result.Result[client.Reference], filename string) ([]byte, error) {
	ref, err := result.SingleRef()
	if err != nil {
		return nil, fmt.Errorf("failed to get ref from response: %w", err)
	}

	return ref.ReadFile(ctx, client.ReadRequest{
		Filename: filename,
	})
}

// MarshalForPlatform converts our memory-local LLB state into a form BuildKit daemon can use for solving on its end.
func MarshalForPlatform(ctx context.Context, source *llb.State, platform llb.ConstraintsOpt) (*llb.Definition, error) {
	definition, err := source.Marshal(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling definition for platform %v: %w", platform, err)
	}

	return definition, nil
}

// DefaultBuildConfig returns a sensible build configuration which should work with most apps out of the box.
func DefaultBuildConfig() Configuration {
	return Configuration{
		ContextName:       DefaultContextName,
		KraftfileName:     DefaultKraftfileName,
		KraftkitWorkspace: DefaultKraftkitWorkspace,
		OutputDir:         DefaultOutputDir,
		BaseKraftkitImage: KraftImageName,
		Platform:          DefaultPlatform,
		Architecture:      DefaultArch,
	}
}

func kraftkitBuildCommand(platform, architecture string) string {
	return fmt.Sprintf("kraft build . %s", targetSelector(platform, architecture))
}

func targetSelector(platform, architecture string) string {
	return fmt.Sprintf("-p %s -m %s", platform, architecture)
}

func outputPath(project string, config Configuration) string {
	return path.Join(config.OutputDir, outputName(project, config))
}

func outputName(project string, config Configuration) string {
	return fmt.Sprintf("%s_%s-%s", project, config.Platform, config.Architecture)
}
