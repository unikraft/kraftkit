package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kcclient "sdk.kraft.cloud/client"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/runtime"
	"kraftkit.sh/unikraft/target"
)

type deployerRootfs struct{}

func (deployer *deployerRootfs) Name() string {
	return "rootfs"
}

func (deployer *deployerRootfs) String() string {
	return ""
}

func (deployer *deployerRootfs) Deployable(ctx context.Context, opts *DeployOptions, args ...string) (bool, error) {
	if opts.Project == nil {
		// Do not capture the the project is not initialized, as we can still build
		// the unikernel using the Dockerfile provided with the `--rootfs`.
		_ = opts.initProject(ctx)
	}

	if opts.Project != nil && opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	// Maybe no `--rootfs` flag was provided, but there may be a local Dockerfile
	// in the working directory.  If so, we can use that as the rootfs.

	var rootfs string

	if len(opts.Rootfs) > 0 {
		rootfs = opts.Rootfs
	} else {
		rootfs = filepath.Join(opts.Workdir, "Dockerfile")

		if _, err := os.Stat(rootfs); err != nil {
			return false, fmt.Errorf("could not find Dockerfile")
		}
	}

	opts.Rootfs = rootfs

	if opts.Project == nil {
		rt := runtime.DefaultKraftCloudRuntime

		// NOTE(craciunoiuc): If we do not set values the default case will fail
		// When we add platform/architecture options we will replace this
		platform := plat.NewPlatformFromOptions(plat.WithName("kraftcloud"))

		if len(opts.Runtime) > 0 {
			rt = opts.Runtime
			platform = plat.NewPlatformFromOptions()

			// Sanitize the runtime for Unikraft Cloud.

			if !strings.Contains(rt, ":") {
				rt += ":latest"
			}

			if strings.HasPrefix(rt, "unikraft.io") {
				rt = "index." + rt
			} else if strings.Contains(rt, "/") && !strings.Contains(rt, "unikraft.io") {
				rt = "index.unikraft.io/" + rt
			} else if !strings.HasPrefix(rt, "index.unikraft.io") {
				rt = "index.unikraft.io/official/" + rt
			}
		}

		runtime, err := runtime.NewRuntime(ctx, rt,
			runtime.WithPlatform(platform.String()),
			runtime.WithArchitecture(arch.ArchitectureX86_64.String()),
		)
		if err != nil {
			return false, fmt.Errorf("could not create runtime: %w", err)
		}

		opts.Project, err = app.NewApplicationFromOptions(
			app.WithRuntime(runtime),
			app.WithName(opts.Name),
			app.WithTargets([]*target.TargetConfig{target.DefaultKraftCloudTarget}),
			app.WithCommand(args...),
			app.WithWorkingDir(opts.Workdir),
			app.WithRootfs(opts.Rootfs),
		)
		if err != nil {
			return false, fmt.Errorf("could not create unikernel application: %w", err)
		}
	}

	return true, nil
}

func (deployer *deployerRootfs) Deploy(ctx context.Context, opts *DeployOptions, args ...string) (*kcclient.ServiceResponse[kcinstances.GetResponseItem], *kcclient.ServiceResponse[kcservices.GetResponseItem], error) {
	return (&deployerKraftfileRuntime{}).Deploy(ctx, opts, args...)
}
