// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
	"kraftkit.sh/config"
	"kraftkit.sh/log"

	sfile "github.com/anchore/stereoscope/pkg/file"
	soci "github.com/anchore/stereoscope/pkg/image/oci"
	"github.com/cavaliergopher/cpio"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/session/filesync"
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
	args       []string
	dockerfile string
	env        []string
	files      []string
}

func fixedWriteCloser(wc io.WriteCloser) filesync.FileOutputFunc {
	return func(map[string]string) (io.WriteCloser, error) {
		return wc, nil
	}
}

// NewFromDockerfile accepts an input path which represents a Dockerfile that
// can be constructed via buildkit to become a CPIO archive.
func NewFromDockerfile(ctx context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	if !strings.Contains(strings.ToLower(path), "dockerfile") {
		return nil, fmt.Errorf("file is not a Dockerfile")
	}

	initrd := dockerfile{
		opts: InitrdOptions{
			workdir: filepath.Dir(path),
		},
		dockerfile: path,
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
	defer os.RemoveAll(outputDir)

	tarOutput, err := os.CreateTemp("", "")
	if err != nil {
		return "", fmt.Errorf("could not make temporary file: %w", err)
	}
	defer tarOutput.Close()
	defer os.RemoveAll(tarOutput.Name())

	ociOutput, err := os.CreateTemp("", "")
	if err != nil {
		return "", fmt.Errorf("could not make temporary file: %w", err)
	}
	defer ociOutput.Close()
	defer os.RemoveAll(ociOutput.Name())

	buildkitAddr := config.G[config.KraftKit](ctx).BuildKitHost
	copts := []client.ClientOpt{}

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
				AlwaysPullImage: true,
				Image:           "moby/buildkit:latest",
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
		Ref: identity.NewID(),
		Exports: []client.ExportEntry{
			{
				Type:   client.ExporterTar,
				Output: fixedWriteCloser(tarOutput),
			},
			{
				Type:   client.ExporterOCI,
				Output: fixedWriteCloser(ociOutput),
			},
		},
		CacheExports: cacheExports,
		LocalDirs: map[string]string{
			"context":    initrd.opts.workdir,
			"dockerfile": filepath.Dir(filepath.Join(initrd.opts.workdir, initrd.dockerfile)),
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
		d, err := progressui.NewDisplay(log.G(ctx).Writer(), progressui.AutoMode)
		if err != nil {
			return fmt.Errorf("could not create progress display: %w", err)
		}

		_, err = d.UpdateFrom(ctx, ch)
		if err != nil {
			return fmt.Errorf("could not display output progress: %w", err)
		}

		return nil
	})

	if err := eg.Wait(); err != nil {
		return "", fmt.Errorf("could not wait for err group: %w", err)
	}

	tarArchive, err := os.Open(tarOutput.Name())
	if err != nil {
		return "", fmt.Errorf("could not open output tarball: %w", err)
	}

	tarReader := tar.NewReader(tarArchive)

	hardlinks := make(map[string]string)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return "", fmt.Errorf("could not read tar header: %w", err)
		}

		targetPath := filepath.Join(outputDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return "", fmt.Errorf("could not create directory: %w", err)
			}
		case tar.TypeReg:
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return "", fmt.Errorf("could not open file: %w", err)
			}
			defer file.Close()
			if _, err := io.Copy(file, tarReader); err != nil {
				return "", fmt.Errorf("could not write file: %w", err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return "", fmt.Errorf("could not create symlink: %w", err)
			}
		case tar.TypeLink:
			hardlinks[header.Name] = header.Linkname
		case tar.TypeBlock:
			log.G(ctx).
				WithField("file", header.Name).
				Warn("ignoring block devices")
		case tar.TypeChar:
			log.G(ctx).
				WithField("file", header.Name).
				Warn("ignoring char devices")
		case tar.TypeFifo:
			log.G(ctx).
				WithField("file", header.Name).
				Warn("ignoring fifo files")
		default:
			return "", fmt.Errorf("unsupported file type: %c", header.Typeflag)
		}
	}

	// Create hardlinks
	// Note(craciunoiuc): This is done afterwards as the hardlink target might
	// not have been created yet by the archive unpacker.
	for src, dst := range hardlinks {
		targetPath := filepath.Join(outputDir, src)
		oldPath := filepath.Join(outputDir, dst)
		if err := os.Link(oldPath, targetPath); err != nil {
			return "", fmt.Errorf("could not create hardlink: %w", err)
		}
	}

	// parse the output directory with stereoscope
	tempgen := sfile.NewTempDirGenerator("kraftkit")
	if tempgen == nil {
		return "", fmt.Errorf("could not create temp dir generator")
	}

	provider := soci.NewArchiveProvider(tempgen, ociOutput.Name())
	if provider == nil {
		return "", fmt.Errorf("could not create image provider")
	}

	img, err := provider.Provide(ctx)
	if err != nil {
		return "", fmt.Errorf("could not provide image: %w", err)
	}

	err = img.Read()
	if err != nil {
		return "", fmt.Errorf("could not read image: %w", err)
	}

	initrd.args = append(initrd.args, img.Metadata.Config.Config.Entrypoint...)
	initrd.args = append(initrd.args, img.Metadata.Config.Config.Cmd...)
	initrd.env = img.Metadata.Config.Config.Env

	err = tempgen.Cleanup()
	if err != nil {
		return "", fmt.Errorf("could not cleanup temp dir generator: %w", err)
	}
	err = img.Cleanup()
	if err != nil {
		return "", fmt.Errorf("could not cleanup image: %w", err)
	}

	f, err := os.OpenFile(initrd.opts.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("could not open initramfs file: %w", err)
	}

	defer f.Close()

	writer := cpio.NewWriter(f)
	defer writer.Close()

	if err := walkFiles(ctx, outputDir, writer, &initrd.files); err != nil {
		return "", fmt.Errorf("could not walk output path: %w", err)
	}

	if initrd.opts.compress {
		if err := compressFiles(initrd.opts.output, writer, f); err != nil {
			return "", fmt.Errorf("could not compress files: %w", err)
		}
	}

	return initrd.opts.output, nil
}

// Files implements Initrd.
func (initrd *dockerfile) Files() []string {
	return initrd.files
}

// Env implements Initrd.
func (initrd *dockerfile) Env() []string {
	return initrd.env
}

// Args implements Initrd.
func (initrd *dockerfile) Args() []string {
	return initrd.args
}
