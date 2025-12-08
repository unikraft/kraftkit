// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package buildkit

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"strings"

	"kraftkit.sh/config"
	"kraftkit.sh/log"

	"github.com/moby/buildkit/client"
	bkappdefaults "github.com/moby/buildkit/util/appdefaults"
	dockerclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	tlog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
	_ "github.com/moby/buildkit/client/connhelper/kubepod"
	_ "github.com/moby/buildkit/client/connhelper/nerdctlcontainer"
	_ "github.com/moby/buildkit/client/connhelper/podmancontainer"
	_ "github.com/moby/buildkit/client/connhelper/ssh"
)

func ConnectToBuildkit(ctx context.Context) (c *client.Client, cleanup func(), rerr error) {
	var buildkitInfo *client.Info

	// Check if there is a buildkit host configured
	buildkitAddr := config.G[config.KraftKit](ctx).BuildKitHost
	if buildkitAddr != "" {
		var err error
		c, err = client.New(ctx, buildkitAddr)
		if err != nil {
			return nil, nil, fmt.Errorf("creating buildkit client: %w", err)
		}
		buildkitInfo, err = c.Info(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("connecting to buildkit client: %w", err)
		}
		log.G(ctx).WithField("addr", buildkitAddr).Info("using configured buildkit")
	}

	// Check if the default buildkit socket is available
	if c == nil {
		var err error
		c, err = client.New(ctx, bkappdefaults.Address)
		if err != nil {
			return nil, nil, fmt.Errorf("creating default buildkit client: %w", err)
		}
		buildkitInfo, err = c.Info(ctx)
		if err != nil {
			c.Close()
			c = nil
		}
		if c != nil {
			log.G(ctx).Info("using default buildkit socket")
		}
	}

	// Check if the docker buildkit host can be used
	if c == nil {
		var err error
		c, buildkitInfo, err = dockerBuildkit(ctx)
		if err != nil {
			return nil, nil, err
		}
		if c != nil {
			log.G(ctx).Info("using docker buildkit")
		}
	}

	// If no other buildkits found, create an ephemeral container
	if c == nil {
		buildkitVersion := getBuildkitVersion(ctx)
		log.G(ctx).
			WithField("version", buildkitVersion).
			Info("creating ephemeral buildkit container")

		testcontainers.DefaultLoggingHook = testcontainersLoggingHook
		printf := &testcontainersPrintf{ctx}
		tlog.SetDefault(printf)

		// Trap any errors with a helpful message for how to use buildkit
		var connerr error
		defer func() {
			if connerr == nil {
				return
			}

			log.G(ctx).Warnf("could not connect to BuildKit client '%s' is BuildKit running?", buildkitAddr)
			log.G(ctx).Warn("")
			log.G(ctx).Warn("By default, KraftKit will look for a native install which")
			log.G(ctx).Warn("is located at /run/buildkit/buildkit.sock.  Alternatively, you")
			log.G(ctx).Warn("can run BuildKit in a container (recommended for macOS users)")
			log.G(ctx).Warn("which you can do by running:")
			log.G(ctx).Warn("")
			log.G(ctx).Warn("  docker run --rm -d --name buildkit --privileged moby/buildkit:" + buildkitVersion)
			log.G(ctx).Warn("")
			log.G(ctx).Warn("Depending on your container runtime, you should connect to Buildkit via:")
			log.G(ctx).Warn("")
			log.G(ctx).Warn("  export KRAFTKIT_BUILDKIT_HOST=docker-container://buildkit # for docker")
			log.G(ctx).Warn("or")
			log.G(ctx).Warn("  export KRAFTKIT_BUILDKIT_HOST=podman-container://buildkit # for podman")
			log.G(ctx).Warn("")
			log.G(ctx).Warn("For more usage instructions visit: https://unikraft.org/buildkit")
			log.G(ctx).Warn("")
		}()

		// Port 0 means "give me any free port"
		addr, err := net.ResolveTCPAddr("tcp", ":0")
		if err != nil {
			return nil, nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, nil, err
		}

		port := l.Addr().(*net.TCPAddr).Port
		_ = l.Close()

		buildkitd, err := startBuildkit(ctx, buildkitVersion, port, printf)
		if err != nil {
			return nil, nil, fmt.Errorf("creating buildkit container: %w", err)
		}

		if buildkitd == nil {
			return nil, nil, fmt.Errorf("could not start ephemeral BuildKit container")
		}

		cleanup = func() {
			err := buildkitd.Terminate(ctx)
			if err != nil && !strings.Contains(err.Error(), "context cancelled") {
				log.G(ctx).
					WithError(err).
					Debug("terminating buildkit container")
			}
		}
		defer func() {
			if rerr != nil {
				cleanup()
			}
		}()

		buildkitAddr = fmt.Sprintf("tcp://localhost:%d", port)

		c, connerr = client.New(ctx, buildkitAddr)
		if connerr != nil {
			return nil, nil, fmt.Errorf("creating container buildkit client: %w", connerr)
		}
		buildkitInfo, connerr = c.Info(ctx)
		if connerr != nil {
			return nil, nil, fmt.Errorf("connecting to buildkit client: %w", connerr)
		}
	}

	log.G(ctx).
		WithField("addr", buildkitAddr).
		WithField("version", buildkitInfo.BuildkitVersion.Version).
		Debug("using buildkit")
	return c, cleanup, nil
}

// see logic from https://github.com/docker/buildx/blob/master/driver/docker/driver.go
func dockerBuildkit(ctx context.Context) (*client.Client, *client.Info, error) {
	d, err := dockerclient.New(dockerclient.FromEnv)
	if err != nil {
		return nil, nil, nil
	}

	_, err = d.ServerVersion(ctx, dockerclient.ServerVersionOptions{})
	if err != nil {
		return nil, nil, nil
	}

	opts := []client.ClientOpt{
		client.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return d.DialHijack(ctx, "/grpc", "h2c", nil)
		}), client.WithSessionDialer(func(ctx context.Context, proto string, meta map[string][]string) (net.Conn, error) {
			return d.DialHijack(ctx, "/session", proto, meta)
		}),
	}

	c, err := client.New(ctx, "", opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("creating docker buildkit client: %w", err)
	}

	// we need features that are *only* supported when the containerd
	// snapshotter is enabled :(
	var hasCtrdSnapshotter bool
	workers, err := c.ListWorkers(ctx)
	if err != nil {
		return nil, nil, c.Close()
	}
	for _, w := range workers {
		if _, ok := w.Labels["org.mobyproject.buildkit.worker.snapshotter"]; ok {
			hasCtrdSnapshotter = true
		}
	}
	if !hasCtrdSnapshotter {
		return nil, nil, c.Close()
	}

	info, err := c.Info(ctx)
	if err != nil {
		return nil, nil, c.Close()
	}
	return c, info, nil
}

func startBuildkit(ctx context.Context, buildkitVersion string, port int, printf *testcontainersPrintf) (testcontainers.Container, error) {
	// Trap any panics that occur when instantiating BuildKit through the
	// testcontainers library. This is known happen if Docker is not installed.
	// For more information see:
	//
	// https://github.com/unikraft/kraftkit/issues/2001
	defer func() {
		if r := recover(); r != nil {
			log.G(ctx).Warn("could not start BuildKit ephemeral container")
			log.G(ctx).Warn("this can be caused by Docker is either not running or inaccessible")
			log.G(ctx).Warn("")
			log.G(ctx).Warn("if you think this was caused by something else, please open an issue at:")
			log.G(ctx).Warn("https://github.com/unikraft/kraftkit/issues")
		}
	}()
	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		Logger:  printf,
		ContainerRequest: testcontainers.ContainerRequest{
			AlwaysPullImage: true,
			Image:           "moby/buildkit:" + buildkitVersion,
			WaitingFor:      wait.ForLog(fmt.Sprintf("running server on [::]:%d", port)),
			Privileged:      true,
			ExposedPorts:    []string{fmt.Sprintf("%d:%d/tcp", port, port)},
			Cmd:             []string{"--addr", fmt.Sprintf("tcp://0.0.0.0:%d", port)},
			Mounts: testcontainers.ContainerMounts{
				{
					Source: testcontainers.GenericVolumeMountSource{
						Name: "kraftkit-buildkit-cache",
					},
					Target: "/var/lib/buildkit",
				},
			},
		},
	})
}

func getBuildkitVersion(ctx context.Context) string {
	buildkitVersion := "latest"
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range bi.Deps {
			if dep.Path == "github.com/moby/buildkit" {
				buildkitVersion = dep.Version
				break
			}
		}
		log.G(ctx).Debug("could not determine BuildKit version from module list")
	}
	return buildkitVersion
}

var testcontainersLoggingHook = func(logger tlog.Logger) testcontainers.ContainerLifecycleHooks {
	shortContainerID := func(c testcontainers.Container) string {
		return c.GetContainerID()[:12]
	}

	return testcontainers.ContainerLifecycleHooks{
		PreCreates: []testcontainers.ContainerRequestHook{
			func(ctx context.Context, req testcontainers.ContainerRequest) error {
				logger.Printf("creating container for image %s", req.Image)
				return nil
			},
		},
		PostCreates: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				logger.Printf("container created: %s", shortContainerID(c))
				return nil
			},
		},
		PreStarts: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				logger.Printf("starting container: %s", shortContainerID(c))
				return nil
			},
		},
		PostStarts: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				logger.Printf("container started: %s", shortContainerID(c))
				return nil
			},
		},
		PreStops: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				logger.Printf("stopping container: %s", shortContainerID(c))
				return nil
			},
		},
		PostStops: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				logger.Printf("container stopped: %s", shortContainerID(c))
				return nil
			},
		},
		PreTerminates: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				logger.Printf("terminating container: %s", shortContainerID(c))
				return nil
			},
		},
		PostTerminates: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				logger.Printf("container terminated: %s", shortContainerID(c))
				return nil
			},
		},
	}
}

type testcontainersPrintf struct {
	ctx context.Context
}

func (t *testcontainersPrintf) Printf(format string, v ...any) {
	if config.G[config.KraftKit](t.ctx).Log.Level == "trace" {
		log.G(t.ctx).Tracef(format, v...)
	}
}
