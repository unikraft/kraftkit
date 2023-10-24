// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package login

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
)

type LoginOptions struct {
	User  string `long:"user" short:"u" usage:"Username" env:"KRAFTKIT_LOGIN_USER"`
	Token string `long:"token" short:"t" usage:"Authentication token" env:"KRAFTKIT_LOGIN_TOKEN"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&LoginOptions{}, cobra.Command{
		Short: "Provide authorization details for a remote service",
		Use:   "login [FLAGS] HOST",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "misc",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *LoginOptions) Run(cmd *cobra.Command, args []string) error {
	var err error
	host := args[0]
	ctx := cmd.Context()

	// Prompt the user from stdin for a username if neither a username nor a token
	// was provided
	if opts.User == "" && opts.Token == "" {
		fmt.Fprint(iostreams.G(ctx).Out, "Username (optional): ")

		reader := bufio.NewReader(iostreams.G(ctx).In)
		opts.User, err = reader.ReadString('\n')
		if err != nil {
			return err
		}

		opts.User = strings.TrimSpace(opts.User)
	}

	// Prompt the user from stdin for the token if it was not provided
	if opts.Token == "" {
		fmt.Print("Password: ")

		btoken, err := term.ReadPassword(int(iostreams.G(ctx).In.Fd()))
		if err != nil {
			return fmt.Errorf("could not read password: %v", err)
		}

		fmt.Fprint(iostreams.G(ctx).Out, "\n")

		opts.Token = string(btoken)
	}

	// Check if the provided token is a base64 encoded string containing both the
	// username and token
	if decoded, err := base64.StdEncoding.DecodeString(opts.Token); err == nil && len(decoded) > 0 {
		split := strings.SplitN(string(decoded), ":", 2)

		if len(split) == 2 {
			if len(opts.User) > 0 {
				return fmt.Errorf("cannot specify -u|--username with a -t|--token that contains a base64 encoding of a username and token")
			}

			opts.User = split[0]
			opts.Token = split[1]
		}
	}

	authConfig := config.AuthConfig{
		Token:     opts.Token,
		Endpoint:  host,
		VerifySSL: true,
	}

	if opts.User != "" {
		authConfig.User = opts.User
	} else if opts.Token != "" {
		authConfig.Token = opts.Token
	}

	if config.G[config.KraftKit](ctx).Auth == nil {
		config.G[config.KraftKit](ctx).Auth = make(map[string]config.AuthConfig)
	}
	config.G[config.KraftKit](ctx).Auth[host] = authConfig

	return config.M[config.KraftKit](ctx).Write(true)
}
