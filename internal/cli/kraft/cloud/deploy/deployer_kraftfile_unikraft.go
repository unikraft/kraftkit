package deploy

import (
	"context"
	"fmt"

	"kraftkit.sh/internal/cli/kraft/build"
	kraftcloudinstances "sdk.kraft.cloud/instances"
)

type deployerKraftfileUnikraft struct{}

func (deployer *deployerKraftfileUnikraft) String() string {
	return "kraftfile-runtime"
}

func (deployer *deployerKraftfileUnikraft) Deployable(ctx context.Context, opts *DeployOptions, args ...string) (bool, error) {
	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.Project.Runtime() != nil {
		return false, nil
	}

	if opts.Project.Unikraft(ctx) == nil {
		return false, fmt.Errorf("cannot package without unikraft attribute")
	}

	return true, nil
}

func (deployer *deployerKraftfileUnikraft) Deploy(ctx context.Context, opts *DeployOptions, args ...string) ([]kraftcloudinstances.Instance, error) {
	if err := build.Build(ctx, &build.BuildOptions{
		Architecture: "x86_64",
		DotConfig:    opts.DotConfig,
		ForcePull:    opts.ForcePull,
		Jobs:         opts.Jobs,
		KernelDbg:    opts.KernelDbg,
		NoCache:      opts.NoCache,
		NoConfigure:  opts.NoConfigure,
		NoFast:       opts.NoFast,
		NoFetch:      opts.NoFetch,
		NoUpdate:     opts.NoUpdate,
		Platform:     "kraftcloud",
		Rootfs:       opts.Rootfs,
		SaveBuildLog: opts.SaveBuildLog,
		Workdir:      opts.Workdir,
	}); err != nil {
		return nil, fmt.Errorf("could not complete build")
	}

	// Re-use the runtime deployer, which also handles packaging.
	runtimeDeployer := &deployerKraftfileRuntime{}
	return runtimeDeployer.Deploy(ctx, opts, args...)
}
