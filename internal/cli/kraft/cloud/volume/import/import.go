// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package vimport // "v(olume)import"; "import" is a reserved keyword

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	ukcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/fancymap"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/tui"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
)

type ImportOptions struct {
	Auth  *config.AuthConfig `noattribute:"true"`
	Token string             `noattribute:"true"`
	Metro string             `noattribute:"true"`

	VolimportImage string `local:"true" long:"image" usage:"Volume import image to use" default:"official/utils/volimport:latest"`
	Force          bool   `local:"true" long:"force" short:"f" usage:"Force import, even if it might fail"`
	Source         string `local:"true" long:"source" short:"s" usage:"Path to the data source (directory, Dockerfile, Docker link, cpio file)" default:"."`
	Timeout        uint64 `local:"true" long:"timeout" short:"t" usage:"Timeout for the import process in seconds when unresponsive" default:"10"`
	VolID          string `local:"true" long:"volume" short:"v" usage:"Identifier of an existing volume (name or UUID)"`
}

const volimportPort uint16 = 42069

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ImportOptions{}, cobra.Command{
		Short: "Import local data to a persistent volume",
		Use:   "import [FLAGS]",
		Args:  cobra.NoArgs,
		Example: heredoc.Doc(`
			# Import data from a local directory "path/to/data" to a volume named "my-volume"
			$ kraft cloud volume import --source path/to/data --volume my-volume

			# Import data from a local Dockerfile "path/to/Dockerfile" to a volume named "my-volume"
			$ kraft cloud volume import --source path/to/Dockerfile --volume my-volume

			# Import data from a docker registry "docker.io/nginx:latest" to a volume named "my-volume"
			$ kraft cloud volume import --source docker.io/nginx:latest --volume my-volume

			# Import data from a local cpio file "path/to/file" to a volume named "my-volume"
			$ kraft cloud volume import --source path/to/file --volume my-volume
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-vol",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ImportOptions) Pre(cmd *cobra.Command, _ []string) error {
	if opts.VolID == "" {
		return fmt.Errorf("must specify a value for the --volume flag")
	}

	if finfo, err := os.Stat(opts.Source); err == nil && (!finfo.IsDir() && !strings.HasSuffix(opts.Source, "Dockerfile")) {
		return fmt.Errorf("local source path must be a directory or a Dockerfile")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *ImportOptions) Run(ctx context.Context, _ []string) error {
	var err error

	if opts.Auth == nil {
		if opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token); err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if err = importVolumeData(ctx, opts); err != nil {
		if IsLoggable(err) {
			return fmt.Errorf("could not import volume data: %w", err)
		}
		return errors.New("could not import volume data")
	}
	return nil
}

// importVolumeData imports local data to a volume.
func importVolumeData(ctx context.Context, opts *ImportOptions) (retErr error) {
	cli := kraftcloud.NewClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
	)
	icli := cli.Instances().WithMetro(opts.Metro)
	vcli := cli.Volumes().WithMetro(opts.Metro)

	var err error

	var cpioPath string
	var cpioSize int64

	paramodel, err := processTree(ctx, "Packaging source as a CPIO archive",
		func(ctx context.Context) error {
			cpioPath, cpioSize, err = buildCPIO(ctx, opts.Source)
			return err
		},
	)
	if err != nil {
		return err
	}
	if err = paramodel.Start(); err != nil {
		return NotLoggable(err) // already logged by runner
	}

	defer func() {
		if cpioPath != opts.Source {
			err := os.Remove(cpioPath)
			if err != nil {
				err = fmt.Errorf("removing temp CPIO archive: %w", err)
			}
			retErr = errors.Join(retErr, err)
		}
	}()

	var volUUID string
	if volUUID, _, err = volumeSanityCheck(ctx, vcli, opts.VolID, cpioSize); err != nil {
		return err
	}

	var authStr string
	var instFQDN string
	var instID string

	paramodel, err = processTree(ctx, "Spawning temporary volume data import instance",
		func(ctx context.Context) error {
			if authStr, err = utils.GenRandAuth(); err != nil {
				return fmt.Errorf("generating random authentication string: %w", err)
			}
			instID, instFQDN, err = runVolimport(ctx, icli, opts.VolimportImage, volUUID, authStr, opts.Timeout)
			return err
		},
	)
	if err != nil {
		return err
	}
	if err = paramodel.Start(); err != nil {
		return NotLoggable(err) // already logged by runner
	}

	// FIXME(antoineco): due to a bug in KraftKit, paraprogress.Start() returns a
	// nil error upon context cancellation. We temporarily handle potential copy
	// errors ourselves here.
	var copyCPIOErr error
	var freeSpace uint64
	var totalSpace uint64

	if log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) == log.FANCY {
		paraprogress, err := paraProgress(ctx, fmt.Sprintf("Importing data (%s)", humanize.IBytes(uint64(cpioSize))),
			func(ctx context.Context, callback func(float64)) (retErr error) {
				instAddr := instFQDN + ":" + strconv.FormatUint(uint64(volimportPort), 10)
				conn, err := tls.Dial("tcp4", instAddr, nil)
				if err != nil {
					return fmt.Errorf("connecting to volume data import instance send port: %w", err)
				}
				defer conn.Close()

				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				freeSpace, totalSpace, err = copyCPIO(ctx, conn, authStr, cpioPath, opts.Force, uint64(cpioSize), callback)
				copyCPIOErr = err
				return err
			},
		)
		if err != nil {
			return err
		}
		if err = paraprogress.Start(); err != nil {
			return err
		}
	} else {
		instAddr := instFQDN + ":" + strconv.FormatUint(uint64(volimportPort), 10)
		conn, err := tls.Dial("tcp4", instAddr, nil)
		if err != nil {
			return fmt.Errorf("connecting to volume data import instance send port: %w", err)
		}
		defer conn.Close()

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		freeSpace, totalSpace, err = copyCPIO(ctx, conn, authStr, cpioPath, opts.Force, uint64(cpioSize), func(progress float64) {})
		copyCPIOErr = err
		return err
	}
	if copyCPIOErr != nil {
		return copyCPIOErr
	}

	if log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) == log.FANCY {
		fancymap.PrintFancyMap(iostreams.G(ctx).Out, tui.TextGreen, "Import complete",
			fancymap.FancyMapEntry{
				Key:   "volume",
				Value: opts.VolID,
			},
			fancymap.FancyMapEntry{
				Key:   "free",
				Value: humanize.IBytes(freeSpace),
			},
			fancymap.FancyMapEntry{
				Key:   "total",
				Value: humanize.IBytes(totalSpace),
			},
		)
	} else {
		log.G(ctx).
			WithField("volume", opts.VolID).
			WithField("free", humanize.IBytes(freeSpace)).
			WithField("total", humanize.IBytes(totalSpace)).
			Info("Import complete")
	}

	// Stopping time can be anywhere between 1-1000ms, so we set a 1100ms timeout
	waitResp, err := cli.Instances().WithMetro(opts.Metro).Wait(ctx, ukcinstances.StateStopped, 1100, instID)
	if err != nil {
		return fmt.Errorf("waiting for volume data import instance to stop: %w", err)
	}

	if w, err := waitResp.FirstOrErr(); err == nil {
		if ukcinstances.State(w.State) == ukcinstances.StateRunning {
			return fmt.Errorf("volume data import instance did not stop yet: %s", w.State)
		} else {
			// Wait a bit for the instance to be deleted
			// NOTE(craciunoiuc): this should never be reached, but it's a safety net
			time.Sleep(100 * time.Millisecond)
			return nil
		}
	} else {
		if strings.Contains(err.Error(), "No instance") {
			return nil
		} else if strings.Contains(err.Error(), "Operation timed out") {
			return fmt.Errorf("timed out waiting for volume data import instance to stop: %w", err)
		} else {
			return fmt.Errorf("waiting for volume data import instance to stop: %w", err)
		}
	}
}

// processTree returns a TUI ProcessTree configured to run the given function
// with common options.
func processTree(ctx context.Context, txt string, fn processtree.SpinnerProcess) (*processtree.ProcessTree, error) {
	return processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(true),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(false),
		},
		processtree.NewProcessTreeItem(txt, "", fn),
	)
}

// paraProgress returns a TUI ParaProgress configured to run the given function.
func paraProgress(ctx context.Context, txt string, fn func(context.Context, func(float64)) error) (*paraprogress.ParaProgress, error) {
	return paraprogress.NewParaProgress(ctx, []*paraprogress.Process{
		paraprogress.NewProcess(txt, fn),
	})
}
