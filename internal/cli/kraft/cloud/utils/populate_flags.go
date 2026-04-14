// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/selection"

	unikraftcloud "sdk.kraft.cloud"
)

type Metro struct {
	Location string
	Code     string
}

var _ fmt.Stringer = (*Metro)(nil)

func (m Metro) String() string {
	return fmt.Sprintf("%s (%s)", m.Code, m.Location)
}

func PopulateMetroToken(cmd *cobra.Command, metro, token *string, allowInsecure *bool) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	if metro != nil {
		*metro = cmd.Flag("metro").Value.String()
		if *metro == "" {
			*metro = os.Getenv("UNIKRAFTCLOUD_METRO")

			if *metro == "" {
				*metro = os.Getenv("KRAFTCLOUD_METRO")
			}

			if *metro == "" {
				*metro = os.Getenv("KC_METRO")
			}

			if *metro == "" {
				*metro = os.Getenv("UKC_METRO")
			}

			if *metro == "" && !config.G[config.KraftKit](cmd.Context()).NoPrompt {
				client := unikraftcloud.NewMetrosClient()

				metros, err := client.List(cmd.Context(), false)
				if err != nil {
					return fmt.Errorf("could not list metros: %w", err)
				}

				candidates := make([]Metro, len(metros))
				for i, m := range metros {
					candidates[i].Code = m.Code
					candidates[i].Location = m.Location
				}

				candidate, err := selection.Select("metro not explicitly set: which one would you like to use?", candidates...)
				if err != nil {
					return err
				}

				*metro = candidate.Code

				log.G(cmd.Context()).Infof("run `export UKC_METRO=%s` or use the `--metro` flag to skip this prompt in the future", *metro)
			}

			if *metro == "" {
				return fmt.Errorf("unikraft cloud metro is unset, try setting `UKC_METRO`, or use the `--metro` flag")
			}
		}

		log.G(cmd.Context()).WithField("metro", *metro).Debug("using")
	}

	if token != nil {
		*token = cmd.Flag("token").Value.String()
		if *token != "" {
			log.G(cmd.Context()).WithField("token", *token).Debug("using")
		} else {
			*token = os.Getenv("UNIKRAFTCLOUD_TOKEN")

			if *token == "" {
				*token = os.Getenv("KRAFTCLOUD_TOKEN")
			}

			if *token == "" {
				*token = os.Getenv("KC_TOKEN")
			}

			if *token == "" {
				*token = os.Getenv("UKC_TOKEN")
			}

			if *token != "" {
				log.G(cmd.Context()).WithField("token", *token).Debug("using")
			}
		}
	}

	if allowInsecure != nil {
		*allowInsecure = cmd.Flag("allow-insecure").Value.String() == "true"
		if *allowInsecure {
			log.G(cmd.Context()).WithField("allow-insecure", *allowInsecure).Debug("using")
		} else {
			*allowInsecure = os.Getenv("UKC_ALLOW_INSECURE") == "true"
		}
	}

	return nil
}
