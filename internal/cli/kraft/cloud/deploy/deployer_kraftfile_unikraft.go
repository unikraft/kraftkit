package deploy

import (
	"context"
	"fmt"
	"strings"

	kcclient "sdk.kraft.cloud/client"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/internal/cli/kraft/build"
)

type deployerKraftfileUnikraft struct {
	args []string
}

func (deployer *deployerKraftfileUnikraft) Name() string {
	return "kraftfile-runtime"
}

func (deployer *deployerKraftfileUnikraft) String() string {
	if len(deployer.args) == 0 {
		return "run the cwd with Kraftfile"
	}

	return fmt.Sprintf("run the detected Kraftfile in the cwd and use '%s' as arg(s)", strings.Join(deployer.args, " "))
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

	deployer.args = args

	return true, nil
}

func (deployer *deployerKraftfileUnikraft) Deploy(ctx context.Context, opts *DeployOptions, args ...string) (*kcclient.ServiceResponse[kcinstances.GetResponseItem], []kcservices.GetResponseItem, error) {
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
		return nil, nil, fmt.Errorf("could not complete build: %w", err)
	}

	// Re-use the runtime deployer, which also handles packaging.
	return (*deployerKraftfileRuntime)(nil).Deploy(ctx, opts, args...)
}
