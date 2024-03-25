// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcclient "sdk.kraft.cloud/client"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type RemoveOptions struct {
	All    bool                       `long:"all" usage:"Remove all services"`
	Auth   *config.AuthConfig         `noattribute:"true"`
	Client kcservices.ServicesService `noattribute:"true"`
	Metro  string                     `noattribute:"true"`
	Token  string                     `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Delete a service group",
		Use:     "remove [FLAGS] [NAME|UUID [NAME|UUID]...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"delete", "del", "rm"},
		Long:    "Delete a service group.",
		Example: heredoc.Doc(`
			# Remove a service group from your account by UUID.
			$ kraft cloud service remove fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Remove a service group from your account by name.
			$ kraft cloud service remove my-service-group

			# Remove multiple service groups from your account.
			$ kraft cloud service remove my-service-group my-other-service-group

			# Remove all service groups from your account.
			$ kraft cloud service remove --all
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-svc",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	if !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify an instance name or UUID, or use the --all flag")
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
		opts.Client = kraftcloud.NewServicesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	var delResp *kcclient.ServiceResponse[kcservices.DeleteResponseItem]

	if opts.All {
		sgListResp, err := opts.Client.WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("listing service groups: %w", err)
		}
		sgList, err := sgListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("listing service groups: %w", err)
		}
		if len(sgList) == 0 {
			return nil
		}

		uuids := make([]string, 0, len(sgList))
		for _, sgItem := range sgList {
			uuids = append(uuids, sgItem.UUID)
		}

		log.G(ctx).Infof("Removing %d service group(s)", len(uuids))

		if delResp, err = opts.Client.WithMetro(opts.Metro).DeleteByUUIDs(ctx, uuids...); err != nil {
			return fmt.Errorf("removing %d service group(s): %w", len(uuids), err)
		}
		if _, err = delResp.AllOrErr(); err != nil {
			return fmt.Errorf("removing %d service group(s): %w", len(uuids), err)
		}
		return nil
	}

	log.G(ctx).Infof("Removing %d service group(s)", len(args))

	allUUIDs := true
	allNames := true
	for _, arg := range args {
		if utils.IsUUID(arg) {
			allNames = false
		} else {
			allUUIDs = false
		}
		if !(allUUIDs || allNames) {
			break
		}
	}

	switch {
	case allUUIDs:
		if delResp, err = opts.Client.WithMetro(opts.Metro).DeleteByUUIDs(ctx, args...); err != nil {
			return fmt.Errorf("removing %d service group(s): %w", len(args), err)
		}
	case allNames:
		if delResp, err = opts.Client.WithMetro(opts.Metro).DeleteByNames(ctx, args...); err != nil {
			return fmt.Errorf("removing %d service group(s): %w", len(args), err)
		}
	default:
		for _, arg := range args {
			log.G(ctx).Infof("Removing %s", arg)

			if utils.IsUUID(arg) {
				delResp, err = opts.Client.WithMetro(opts.Metro).DeleteByUUIDs(ctx, arg)
			} else {
				delResp, err = opts.Client.WithMetro(opts.Metro).DeleteByNames(ctx, arg)
			}
			if err != nil {
				return fmt.Errorf("could not delete service group: %w", err)
			}
		}
	}
	if _, err = delResp.AllOrErr(); err != nil {
		return fmt.Errorf("removing service group(s): %w", err)
	}

	return nil
}
