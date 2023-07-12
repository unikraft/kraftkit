// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package login

import (
	"bufio"
	"fmt"
	"strings"

	"golang.org/x/term"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"

	"github.com/juju/errors"
	"github.com/spf13/cobra"
)

type Login struct {
	User  string `long:"user" short:"u" usage:"Username" env:"KRAFTKIT_LOGIN_USER"`
	Token string `long:"token" short:"t" usage:"Authentication token" env:"KRAFTKIT_LOGIN_TOKEN"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Login{}, cobra.Command{
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

func (opts *Login) Run(cmd *cobra.Command, args []string) error {
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
			return errors.Annotate(err, "could not read password")
		}

		fmt.Fprint(iostreams.G(ctx).Out, "\n")

		opts.Token = string(btoken)
	}

	authConfig := config.AuthConfig{
		Token:    opts.Token,
		Endpoint: host,
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
