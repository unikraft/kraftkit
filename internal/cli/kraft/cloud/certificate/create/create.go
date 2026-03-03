// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kccertificates "sdk.kraft.cloud/certificates"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type CreateOptions struct {
	Auth          *config.AuthConfig                 `noattribute:"true"`
	Client        kccertificates.CertificatesService `noattribute:"true"`
	Metro         string                             `noattribute:"true"`
	Chain         string                             `local:"true" long:"chain" short:"C" usage:"The chain of the certificate"`
	CN            string                             `local:"true" long:"cn" short:"c" usage:"The common name of the certificate"`
	Name          string                             `local:"true" size:"name" short:"n" usage:"The name of the certificate"`
	Output        string                             `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list,raw" default:"table"`
	PKey          string                             `local:"true" long:"pkey" short:"p" usage:"The private key of the certificate in PEM format"`
	Token         string                             `noattribute:"true"`
	allowInsecure bool
}

func isValidChain(chain []byte) error {
	for {
		block, rest := pem.Decode(chain)
		if block == nil {
			if len(rest) > 0 {
				return fmt.Errorf("could not parse PEM")
			}
			break
		}
		if _, err := x509.ParseCertificates(block.Bytes); err != nil {
			return fmt.Errorf("could not parse certificate: %w", err)
		}
		chain = rest
	}
	return nil
}

func isValidPrivateKey(pkey []byte) error {
	block, _ := pem.Decode(pkey)
	if block == nil {
		return fmt.Errorf("could not parse PEM")
	}

	if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return nil
	}

	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return nil
	}

	return fmt.Errorf("could not parse private key in PKCS1 or PKCS8 format")
}

// Create a KraftCloud certificate.
func Create(ctx context.Context, opts *CreateOptions) (*kccertificates.CreateResponseItem, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewCertificatesClient(
			kraftcloud.WithAllowInsecure(opts.allowInsecure),
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if _, err := os.Stat(opts.PKey); err != nil {
		if os.IsNotExist(err) {
			log.G(ctx).Info("reading private key from argument")
		} else {
			return nil, fmt.Errorf("could not read private key: %w", err)
		}
	} else {
		b, err := os.ReadFile(opts.PKey)
		if err != nil {
			return nil, fmt.Errorf("could not read private key: %w", err)
		}

		opts.PKey = string(b)
	}
	if err := isValidPrivateKey([]byte(opts.PKey)); err != nil {
		return nil, fmt.Errorf("invalid private key argument: %w", err)
	}

	if _, err := os.Stat(opts.Chain); err != nil {
		if os.IsNotExist(err) {
			log.G(ctx).Info("reading chain from argument")
		} else {
			return nil, fmt.Errorf("could not read chain: %w", err)
		}
	} else {
		b, err := os.ReadFile(opts.Chain)
		if err != nil {
			return nil, fmt.Errorf("could not read chain: %w", err)
		}

		opts.Chain = string(b)
	}
	if err := isValidChain([]byte(opts.Chain)); err != nil {
		return nil, fmt.Errorf("invalid chain argument: %w", err)
	}

	createResp, err := opts.Client.WithMetro(opts.Metro).Create(ctx, &kccertificates.CreateRequest{
		Chain: opts.Chain,
		CN:    opts.CN,
		Name:  opts.Name,
		PKey:  opts.PKey,
	})
	if err != nil {
		return nil, fmt.Errorf("creating certificate: %w", err)
	}
	create, err := createResp.FirstOrErr()
	if err != nil {
		return nil, fmt.Errorf("creating certificate: %w", err)
	}

	return create, nil
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create a certificate",
		Use:     "create [FLAGS]",
		Args:    cobra.NoArgs,
		Aliases: []string{"crt"},
		Long: heredoc.Doc(`
			Create a new certificate.
		`),
		Example: heredoc.Doc(`
			# Create a new certificate with a given common name, private key file and chain.
			$ kraft cloud certificate create --name my-cert --cn '*.example.com' --pkey 'private-key.pem' --chain 'chain.pem'
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-certificate",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	if opts.CN == "" {
		return fmt.Errorf("common name (CN) is required")
	}

	if opts.PKey == "" {
		return fmt.Errorf("private key is required")
	}

	if opts.Chain == "" {
		return fmt.Errorf("chain is required")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.allowInsecure)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, _ []string) error {
	certificate, err := Create(ctx, opts)
	if err != nil {
		return fmt.Errorf("could not create certificate: %w", err)
	}

	certResp, err := opts.Client.WithMetro(opts.Metro).Get(ctx, certificate.UUID)
	if err != nil {
		return fmt.Errorf("could not get certificate %s: %w", certificate.UUID, err)
	}

	return utils.PrintCertificates(ctx, opts.Output, *certResp)
}
