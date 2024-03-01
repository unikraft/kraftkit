// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
	"kraftkit.sh/config"
	"kraftkit.sh/log"

	"github.com/cavaliergopher/cpio"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
	_ "github.com/moby/buildkit/client/connhelper/kubepod"
	_ "github.com/moby/buildkit/client/connhelper/nerdctlcontainer"
	_ "github.com/moby/buildkit/client/connhelper/podmancontainer"
	_ "github.com/moby/buildkit/client/connhelper/ssh"
)

var testcontainersLoggingHook = func(logger testcontainers.Logging) testcontainers.ContainerLifecycleHooks {
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

func (t *testcontainersPrintf) Printf(format string, v ...interface{}) {
	if config.G[config.KraftKit](t.ctx).Log.Level == "trace" {
		log.G(t.ctx).Tracef(format, v...)
	}
}

type dockerfile struct {
	opts       InitrdOptions
	dockerfile string
	files      []string
	workdir    string
}

// NewFromDockerfile accepts an input path which represents a Dockerfile that
// can be constructed via buildkit to become a CPIO archive.
func NewFromDockerfile(ctx context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	if !strings.Contains(strings.ToLower(path), "dockerfile") {
		return nil, fmt.Errorf("file is not a Dockerfile")
	}

	initrd := dockerfile{
		opts:       InitrdOptions{},
		dockerfile: path,
		workdir:    filepath.Dir(path),
	}

	for _, opt := range opts {
		if err := opt(&initrd.opts); err != nil {
			return nil, err
		}
	}

	return &initrd, nil
}

// Build implements Initrd.
func (initrd *dockerfile) Build(ctx context.Context) (string, error) {
	if initrd.opts.output == "" {
		fi, err := os.CreateTemp("", "")
		if err != nil {
			return "", err
		}

		initrd.opts.output = fi.Name()
	}

	outputDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("could not make temporary directory: %w", err)
	}

	buildkitAddr := config.G[config.KraftKit](ctx).BuildKitHost
	copts := []client.ClientOpt{
		client.WithFailFast(),
	}

	c, _ := client.New(ctx, buildkitAddr, copts...)
	buildKitInfo, err := c.Info(ctx)
	if err != nil {
		log.G(ctx).Debugf("connecting to host buildkit client: %s", err)
		log.G(ctx).Info("creating ephemeral buildkit container")

		testcontainers.DefaultLoggingHook = testcontainersLoggingHook
		printf := &testcontainersPrintf{ctx}
		testcontainers.Logger = printf

		// Trap any errors with a helpful message for how to use buildkit
		defer func() {
			if err == nil {
				return
			}

			log.G(ctx).Warnf("could not connect to BuildKit client '%s' is BuildKit running?", buildkitAddr)
			log.G(ctx).Warn("")
			log.G(ctx).Warn("By default, KraftKit will look for a native install which")
			log.G(ctx).Warn("is located at /run/buildkit/buildkit.sock.  Alternatively, you")
			log.G(ctx).Warn("can run BuildKit in a container (recommended for macOS users)")
			log.G(ctx).Warn("which you can do by running:")
			log.G(ctx).Warn("")
			log.G(ctx).Warn("  docker run --rm -d --name buildkit --privileged moby/buildkit:latest")
			log.G(ctx).Warn("  export KRAFTKIT_BUILDKIT_HOST=docker-container://buildkit")
			log.G(ctx).Warn("")
			log.G(ctx).Warn("For more usage instructions visit: https://unikraft.org/buildkit")
			log.G(ctx).Warn("")
		}()

		// Generate a random port number between 4000 and 5000.  Try 10 times before
		// giving up.
		var port int
		attempts := 0

		for {
			if attempts > 10 {
				return "", fmt.Errorf("could not find an available port after 10 attempts")
			}

			port = rand.Intn(5000-4000+1) + 4000
			listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
			if err != nil {
				log.G(ctx).WithField("port", port).Debug("port is use")
				attempts++
				continue
			}

			listener.Close()
			break
		}

		buildkitd, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			Started: true,
			Logger:  printf,
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "moby/buildkit:latest",
				WaitingFor:   wait.ForLog(fmt.Sprintf("running server on [::]:%d", port)),
				Privileged:   true,
				ExposedPorts: []string{fmt.Sprintf("%d:%d/tcp", port, port)},
				Cmd:          []string{"--addr", fmt.Sprintf("tcp://0.0.0.0:%d", port)},
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
		if err != nil {
			return "", fmt.Errorf("creating buildkit container: %w", err)
		}
		defer func() {
			if err := buildkitd.Terminate(context.TODO()); err != nil {
				log.G(ctx).Warnf("terminating buildkit container: %s", err)
			}
		}()

		buildkitAddr = fmt.Sprintf("tcp://localhost:%d", port)

		c, _ = client.New(ctx, buildkitAddr, copts...)
		buildKitInfo, err = c.Info(ctx)
		if err != nil {
			return "", fmt.Errorf("connecting to container buildkit client: %w", err)
		}
	}

	log.G(ctx).
		WithField("addr", buildkitAddr).
		WithField("version", buildKitInfo.BuildkitVersion.Version).
		Debug("using buildkit")

	var cacheExports []client.CacheOptionsEntry
	if len(initrd.opts.cacheDir) > 0 {
		cacheExports = []client.CacheOptionsEntry{
			{
				Type: "local",
				Attrs: map[string]string{
					"dest": initrd.opts.cacheDir,
				},
			},
		}
	}

	solveOpt := &client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type:      client.ExporterLocal,
				OutputDir: outputDir,
			},
		},
		CacheExports: cacheExports,
		LocalDirs: map[string]string{
			"context":    initrd.workdir,
			"dockerfile": filepath.Dir(initrd.dockerfile),
		},
		Frontend: "dockerfile.v0",
		FrontendAttrs: map[string]string{
			"filename": filepath.Base(initrd.dockerfile),
		},
	}

	if initrd.opts.arch != "" {
		solveOpt.FrontendAttrs["platform"] = fmt.Sprintf("linux/%s", initrd.opts.arch)
	}

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		_, err := c.Solve(ctx, nil, *solveOpt, ch)
		if err != nil {
			return fmt.Errorf("could not solve: %w", err)
		}
		return nil
	})

	eg.Go(func() error {
		_, err = progressui.DisplaySolveStatus(ctx, nil, log.G(ctx).Writer(), ch)
		if err != nil {
			return fmt.Errorf("could not display output progress: %w", err)
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return "", fmt.Errorf("could not wait for err group: %w", err)
	}

	f, err := os.OpenFile(initrd.opts.output, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return "", fmt.Errorf("could not open initramfs file: %w", err)
	}

	defer f.Close()

	writer := cpio.NewWriter(f)
	defer writer.Close()

	// Recursively walk the output directory on successful build and serialize to
	// the output
	if err := filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("received error before parsing path: %w", err)
		}

		internal := strings.TrimPrefix(path, filepath.Clean(outputDir))
		if internal == "" {
			return nil // Do not archive empty paths
		}
		internal = "." + filepath.ToSlash(internal)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("could not get directory entry info: %w", err)
		}

		if d.Type().IsDir() {
			if err := writer.WriteHeader(&cpio.Header{
				Name: internal,
				Mode: cpio.FileMode(info.Mode().Perm()) | cpio.TypeDir,
			}); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

			return nil
		}

		initrd.files = append(initrd.files, internal)

		log.G(ctx).
			WithField("file", internal).
			Trace("archiving")

		var data []byte
		targetLink := ""
		if info.Mode()&os.ModeSymlink != 0 {
			targetLink, err = os.Readlink(path)
			data = []byte(targetLink)
		} else if d.Type().IsRegular() {
			data, err = os.ReadFile(path)
		} else {
			log.G(ctx).Warnf("unsupported file: %s", path)
			return nil
		}
		if err != nil {
			return fmt.Errorf("could not read file: %w", err)
		}

		header := &cpio.Header{
			Name:    internal,
			Mode:    cpio.FileMode(info.Mode().Perm()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		switch {
		case info.Mode().IsDir():
			header.Mode |= cpio.TypeDir

		case info.Mode().IsRegular():
			header.Mode |= cpio.TypeReg

		case info.Mode()&fs.ModeSymlink != 0:
			header.Mode |= cpio.TypeSymlink
			header.Linkname = targetLink
		}

		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf("writing cpio header for %q: %w", internal, err)
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("could not walk output path: %w", err)
	}

	return initrd.opts.output, nil
}

// Files implements Initrd.
func (initrd *dockerfile) Files() []string {
	return initrd.files
}
