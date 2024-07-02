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

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

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

	Source string `local:"true" long:"source" short:"s" usage:"Path to the data source (directory, Dockerfile)" default:"."`
	VolID  string `local:"true" long:"volume" short:"v" usage:"Identifier of an existing volume (name or UUID)"`
}

const volimportPort uint16 = 42069

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ImportOptions{}, cobra.Command{
		Short: "Import local data to a volume",
		Use:   "import [FLAGS]",
		Args:  cobra.NoArgs,
		Example: heredoc.Doc(`
			# Import data from a local directory "path/to/data" to a volume named "my-volume"
			$ kraft cloud volume import --source path/to/data --volume my-volume
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
		err := os.Remove(cpioPath)
		if err != nil {
			err = fmt.Errorf("removing temp CPIO archive: %w", err)
		}
		retErr = errors.Join(retErr, err)
	}()

	var volUUID string
	var volSize int64
	if volUUID, volSize, err = volumeSanityCheck(ctx, vcli, opts.VolID, cpioSize); err != nil {
		return err
	}

	var authStr string
	var instID string
	var instFQDN string

	paramodel, err = processTree(ctx, "Spawning temporary volume data import instance",
		func(ctx context.Context) error {
			if authStr, err = genRandAuth(); err != nil {
				return fmt.Errorf("generating random authentication string: %w", err)
			}
			instID, instFQDN, err = runVolimport(ctx, icli, volUUID, authStr)
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
		noInterrupt := context.Background()
		retErr = errors.Join(retErr,
			terminateVolimport(noInterrupt, icli, instID),
		)
	}()

	// FIXME(antoineco): due to a bug in KraftKit, paraprogress.Start() returns a
	// nil error upon context cancellation. We temporarily handle potential copy
	// errors ourselves here.
	var copyCPIOErr error

	paraprogress, err := paraProgress(ctx, fmt.Sprintf("Importing data (%s)", humanize.IBytes(uint64(cpioSize))),
		func(ctx context.Context, prompt func(), callback func(float64)) (retErr error) {
			instAddr := instFQDN + ":" + strconv.FormatUint(uint64(volimportPort), 10)
			conn, err := tls.Dial("tcp4", instAddr, nil)
			if err != nil {
				return fmt.Errorf("connecting to volume data import instance: %w", err)
			}
			defer func() {
				retErr = errors.Join(retErr, conn.Close())
			}()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			err = copyCPIO(ctx, conn, authStr, cpioPath, cpioSize, callback)
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
	if copyCPIOErr != nil {
		return copyCPIOErr
	}

	fancymap.PrintFancyMap(iostreams.G(ctx).Out, tui.TextGreen, "Import complete",
		fancymap.FancyMapEntry{
			Key:   "volume",
			Value: opts.VolID,
		},
		fancymap.FancyMapEntry{
			Key:   "imported",
			Value: humanize.IBytes(uint64(cpioSize)),
		},
		fancymap.FancyMapEntry{
			Key:   "capacity",
			Value: humanize.IBytes(uint64(volSize)),
		},
	)

	return nil
}

// processTree returns a TUI ProcessTree configured to run the given function
// with common options.
func processTree(ctx context.Context, txt string, fn processtree.SpinnerProcess) (*processtree.ProcessTree, error) {
	return processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(false),
		},
		processtree.NewProcessTreeItem(txt, "", fn),
	)
}

// paraProgress returns a TUI ParaProgress configured to run the given function.
func paraProgress(ctx context.Context, txt string, fn func(context.Context, func(), func(float64)) error) (*paraprogress.ParaProgress, error) {
	return paraprogress.NewParaProgress(ctx, []*paraprogress.Process{
		paraprogress.NewProcess(txt, fn),
	})
}
