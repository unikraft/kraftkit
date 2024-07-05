// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package tunnel

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type TunnelOptions struct {
	token string
	metro string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&TunnelOptions{}, cobra.Command{
		Short: "Forward a local port to a service through a TLS tunnel",
		Use:   "tunnel [FLAGS] SERVICE [LOCAL_PORT:]REMOTE_PORT",
		Args:  cobra.ExactArgs(2),
		Example: heredoc.Doc(`
			# Forward the local port 8443 to the port 443 of the "my-service" service.
			$ kraft cloud tunnel my-service 8443:443
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *TunnelOptions) Pre(cmd *cobra.Command, _ []string) error {
	if err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token); err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}
	return nil
}

func (opts *TunnelOptions) Run(ctx context.Context, args []string) error {
	sgID := args[0]

	lport, rport, err := parsePorts(args[1])
	if err != nil {
		return err
	}

	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	cli := kraftcloud.NewServicesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	).WithMetro(opts.metro)

	fqdn, err := serviceSanityCheck(ctx, cli, sgID, rport)
	if err != nil {
		return err
	}

	r := Relay{
		// TODO(antoineco): allow dual-stack by creating two separate listeners.
		// Alternatively, we could have defaulted to the address "::" to create a
		// tcp46 socket, but listening on all addresses is an insecure default.
		lAddr: net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(lport), 10)),
		rAddr: net.JoinHostPort(fqdn, strconv.FormatUint(uint64(rport), 10)),
	}
	return r.Up(ctx)
}

// serviceSanityCheck verifies that the given service can be tunneled to.
// In case of success, the (public) FQDN of service is returned.
func serviceSanityCheck(ctx context.Context, cli kcservices.ServicesService, sgID string, rport uint16) (fqdn string, err error) {
	sgGetResp, err := cli.Get(ctx, sgID)
	if err != nil {
		return "", fmt.Errorf("getting service '%s': %w", sgID, err)
	}
	sg, err := sgGetResp.FirstOrErr()
	if err != nil {
		return "", fmt.Errorf("getting service '%s': %w", sgID, err)
	}

	if len(sg.Domains) == 0 {
		return "", fmt.Errorf("service '%s' has no public domain", sgID)
	}

	var hasPort bool
	var exposedPorts []int
	for _, svc := range sg.Services {
		if svc.Port == int(rport) {
			hasPort = true
			break
		}
		exposedPorts = append(exposedPorts, svc.Port)
	}
	if !hasPort {
		return "", fmt.Errorf("service '%s' does not expose port %d. Ports exposed are %v", sgID, rport, exposedPorts)
	}

	return sg.Domains[0].FQDN, nil
}

// parsePorts parses a command line argument in the format [lport:]rport into
// two port numbers lport and rport. If lport isn't set, a random port will be
// used by the relay.
func parsePorts(portsArg string) (lport, rport uint16, err error) {
	ports := strings.SplitN(portsArg, ":", 2)

	if len(ports) == 1 {
		rport64, err := strconv.ParseUint(ports[0], 10, 16)
		if err != nil {
			return 0, 0, fmt.Errorf("%q is not a valid port number", ports[0])
		}
		return 0, uint16(rport64), nil
	}

	lport64, err := strconv.ParseUint(ports[0], 10, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("%q is not a valid port number", ports[0])
	}

	rport64, err := strconv.ParseUint(ports[1], 10, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("%q is not a valid port number", ports[1])
	}

	return uint16(lport64), uint16(rport64), nil
}
