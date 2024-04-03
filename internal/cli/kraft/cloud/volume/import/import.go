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
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"
	kcvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/retrytimeout"
	"kraftkit.sh/log"
)

type ImportOptions struct {
	Auth  *config.AuthConfig `noattribute:"true"`
	Token string             `noattribute:"true"`
	Metro string             `noattribute:"true"`

	Source string `local:"true" long:"source" short:"s" usage:"Path to the data source (directory, Dockerfile)" default:"."`
	VolID  string `local:"true" long:"volume" short:"v" usage:"Identifier of an existing volume (name or UUID)"`
}

const volinitPort uint16 = 42069

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ImportOptions{}, cobra.Command{
		Short: "Initialize a volume by importing local data",
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
		return fmt.Errorf("could not import volume data: %w", err)
	}
	return nil
}

// importVolumeData initializes a volume by importing local data.
func importVolumeData(ctx context.Context, opts *ImportOptions) (retErr error) {
	log.G(ctx).Info("Packaging source as a CPIO archive")

	cpioPath, cpioSize, err := buildCPIO(ctx, opts.Source)
	if err != nil {
		return err
	}
	defer func() {
		log.G(ctx).Info("Removing CPIO archive")

		err := os.Remove(cpioPath)
		if err != nil {
			err = fmt.Errorf("removing temp CPIO archive: %w", err)
		}
		retErr = errors.Join(retErr, err)
	}()

	cli := kraftcloud.NewClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
	)
	vcli := cli.Volumes().WithMetro(opts.Metro)
	icli := cli.Instances().WithMetro(opts.Metro)

	log.G(ctx).Info("Spawning temporary volume initialization instance")

	authStr, err := genRandAuth()
	if err != nil {
		return fmt.Errorf("generating random authentication string: %w", err)
	}
	instID, instFQDN, err := runVolInit(ctx, vcli, icli, opts.VolID, authStr)
	if err != nil {
		return err
	}
	defer func() {
		log.G(ctx).Info("Destroying temporary volume initialization instance")
		retErr = errors.Join(retErr, terminateVolInit(ctx, icli, instID))
	}()

	log.G(ctx).WithField("size_mb", cpioSize/(1024*1024)).Info("Uploading data...")

	instAddr := instFQDN + ":" + strconv.FormatUint(uint64(volinitPort), 10)
	conn, err := tls.Dial("tcp4", instAddr, nil)
	if err != nil {
		return fmt.Errorf("connecting to volume initialization instance: %w", err)
	}
	defer func() {
		retErr = errors.Join(retErr, conn.Close())
	}()

	if err = copyCPIO(conn, cpioSize, cpioPath, authStr); err != nil {
		return fmt.Errorf("copying data to volume initialization instance: %w", err)
	}

	log.G(ctx).Info("Done")

	return nil
}

// buildCPIO generates a CPIO archive from the data at the given source.
func buildCPIO(ctx context.Context, source string) (path string, size int64, err error) {
	if source == "." {
		source, err = os.Getwd()
		if err != nil {
			return "", -1, fmt.Errorf("getting current working directory: %w", err)
		}
	}

	cpio, err := initrd.New(ctx, source)
	if err != nil {
		return "", -1, fmt.Errorf("initializing temp CPIO archive: %w", err)
	}
	cpioPath, err := cpio.Build(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("building temp CPIO archive: %w", err)
	}
	cpioStat, err := os.Stat(cpioPath)
	if err != nil {
		return "", -1, fmt.Errorf("reading CPIO archive info at %s: %w", cpioPath, err)
	}

	return cpioPath, cpioStat.Size(), nil
}

// copyCPIO copies the CPIO archive at the given path over the provided tls.Conn.
// It uses the size parameter to ensure that the entirety of the data was copied.
func copyCPIO(conn *tls.Conn, size int64, path, auth string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = io.Copy(conn, strings.NewReader(auth)); err != nil {
		return err
	}

	n, err := io.Copy(conn, f)
	if err != nil {
		if !isNetClosedError(err) {
			return err
		}
		if n != size {
			return fmt.Errorf("incomplete write (%d/%d)", n, size)
		}
	}

	return nil
}

// runVolInit spawns a volume instance with the given volume attached.
func runVolInit(ctx context.Context,
	vcli kcvolumes.VolumesService, icli kcinstances.InstancesService, volID, authStr string,
) (instID, fqdn string, err error) {
	getvolResp, err := vcli.Get(ctx, volID)
	if err != nil {
		return "", "", fmt.Errorf("getting volume details: %w", err)
	}
	vol, err := getvolResp.FirstOrErr()
	if err != nil {
		return "", "", fmt.Errorf("getting volume details: %w", err)
	}

	crinstResp, err := icli.Create(ctx, kcinstances.CreateRequest{
		Image:    "volinit:latest",
		MemoryMB: ptr(32),
		Args: []string{
			"-p", strconv.FormatUint(uint64(volinitPort), 10),
			"-a", authStr,
		},
		ServiceGroup: &kcinstances.CreateRequestServiceGroup{
			Services: []kcservices.CreateRequestService{{
				Port:            int(volinitPort),
				DestinationPort: ptr(int(volinitPort)),
				Handlers:        []kcservices.Handler{kcservices.HandlerTLS},
			}},
		},
		Volumes: []kcinstances.CreateRequestVolume{{
			UUID: &vol.UUID,
			At:   ptr("/"),
		}},
		Autostart:     ptr(true),
		WaitTimeoutMs: ptr(int((3 * time.Second).Milliseconds())),
	})
	if err != nil {
		return "", "", fmt.Errorf("creating volume initialization instance: %w", err)
	}
	inst, err := crinstResp.FirstOrErr()
	if err != nil {
		return "", "", fmt.Errorf("creating volume initialization instance: %w", err)
	}

	return inst.UUID, inst.ServiceGroup.Domains[0].FQDN, nil
}

// terminateVolInit deletes the volinit instance once it has reached the "stopped" state.
func terminateVolInit(ctx context.Context, icli kcinstances.InstancesService, instID string) error {
	err := retrytimeout.RetryTimeout(3*time.Second, func() error {
		getinstResp, err := icli.Get(ctx, instID)
		if err != nil {
			return fmt.Errorf("getting status of volume initialization instance '%s': %w", instID, err)
		}
		inst, err := getinstResp.FirstOrErr()
		if err != nil {
			return fmt.Errorf("getting status of volume initialization instance '%s': %w", instID, err)
		}
		if inst.State != "stopped" {
			return fmt.Errorf("instance has not yet stopped (state: %s)", inst.State)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("waiting for volume initialization instance '%s' to stop: %w", instID, err)
	}

	delinstResp, err := icli.Delete(ctx, instID)
	if err != nil {
		return fmt.Errorf("deleting volume initialization instance '%s': %w", instID, err)
	}
	if _, err = delinstResp.FirstOrErr(); err != nil {
		return fmt.Errorf("deleting volume initialization instance '%s': %w", instID, err)
	}
	return nil
}

// isNetClosedError reports whether err is an error encountered while writing a
// response over the network, potentially when the server has gone away
func isNetClosedError(err error) bool {
	if oe := (*net.OpError)(nil); errors.As(err, &oe) && oe.Op == "write" {
		return true
	}
	return false
}

func ptr[T comparable](v T) *T { return &v }
