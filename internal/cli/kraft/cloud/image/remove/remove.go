// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcimages "sdk.kraft.cloud/images"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type RemoveOptions struct {
	All    bool                   `long:"all" usage:"Remove all images"`
	Auth   *config.AuthConfig     `noattribute:"true"`
	Client kcimages.ImagesService `noattribute:"true"`
	Metro  string                 `noattribute:"true"`
	Token  string                 `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Remove an image",
		Use:     "remove [FLAGS] [USER/]NAME[:latest|@sha256:...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"rm", "delete", "del"},
		Long: heredoc.Doc(`
			Remove an image for your account.
		`),
		Example: heredoc.Doc(`
			# Remove an image from your account.
			$ kraft cloud image remove caddy

			# Remove an image from your account by tag.
			$ kraft cloud image remove caddy:latest

			# Remove an image from your account by digest.
			$ kraft cloud image remove caddy@sha256:2ba5324141...

			# Remove an image from your account with user.
			$ kraft cloud image remove user/caddy

			# Remove multiple images from your account.
			$ kraft cloud image remove caddy:latest nginx:latest caddy:other-latest

			# Remove all images from your account.
			$ kraft cloud image remove --all
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-image",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	if !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify an image name, or use the --all flag")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewImagesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if opts.All {
		imgListResp, err := opts.Client.WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("listing images: %w", err)
		}
		imgList, err := imgListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("listing images: %w", err)
		}
		if len(imgList) == 0 {
			return nil
		}

		var notFoundMessagesCount int
		for _, image := range imgList {
			if !strings.HasPrefix(image.Digest, strings.TrimSuffix(strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud")) {
				continue
			}

			log.G(ctx).Infof("removing %s", image.Digest)

			if err := opts.Client.WithMetro(opts.Metro).DeleteByName(ctx, image.Digest); err != nil {
				log.G(ctx).Warnf("could not delete image: %s", err.Error())

				if strings.Contains(err.Error(), "NOT_FOUND") {
					notFoundMessagesCount++
				}
			}
		}

		if notFoundMessagesCount > 0 {
			log.G(ctx).Warnf("some images were not found. This is expected if you have already removed them.")
		}
	}

	for _, arg := range args {
		if strings.Contains(arg, "/") {
			splits := strings.Split(arg, "/")
			arg = splits[len(splits)-1]
		}

		log.G(ctx).Infof("removing %s", arg)

		if err := opts.Client.WithMetro(opts.Metro).DeleteByName(ctx, arg); err != nil {
			if strings.Contains(err.Error(), "NOT_FOUND") {
				log.G(ctx).Warnf("%s not found. This is expected if you have already removed it.", arg)
			} else {
				return fmt.Errorf("could not remove image: %w", err)
			}
		}
	}

	return err
}
