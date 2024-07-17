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
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type TunnelOptions struct {
	ProxyPorts         []string `local:"true" long:"proxy-ports" short:"p" usage:"Ports to use for the proxies. Default start port is 4444"`
	ProxyControlPort   uint     `local:"true" long:"proxy-control-port" short:"P" usage:"Port to use for the proxy control" default:"4443"`
	TunnelServiceImage string   `local:"true" long:"tunnel-service-image" usage:"Tunnel service image to use" default:"official/utils/tunnel:latest"`
	Token              string   `noattribute:"true"`
	Metro              string   `noattribute:"true"`

	// Parsed arguments
	// ProxyPorts converted from string to uint16
	parsedProxyPorts []uint16
	// instance name/uuid/private-ip - gets turned into private-ip after fetching
	instances []string
	// port to forward on the local machine
	localPorts []uint16
	// type of the port to forward (tcp/udp)
	ctypes []string
	// port to forward of the instance
	instanceProxyPorts []uint16
	// port to expose the proxy on
	exposedProxyPorts []uint16

	// port iterator for when a single proxy port is provided
	portIterator uint16
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&TunnelOptions{}, cobra.Command{
		Short: "Forward a local port to an instance through a TLS tunnel",
		Use:   "tunnel [FLAGS] [LOCAL_PORT:]INSTANCE:DEST_PORT[/TYPE] [[LOCAL_PORT:]INSTANCE:DEST_PORT[/TYPE]]...",
		Args:  cobra.MinimumNArgs(1),
		Example: heredoc.Doc(`
			# Forward the local port 8080 to the tcp port 8080 of the private instance 'my-instance'
			$ kraft cloud my-instance:8080

			# Forward the local port 8443 to the tcp port 8080 of the private instance 'my-instance'
			$ kraft cloud 8443:my-instance:8080/tcp

			# Forward multiple ports to multiple instances
			$ kraft cloud 8080:my-instance1:8080/tcp 8443:my-instance2:8080/tcp

			# Forward the local port 8080 to the tcp port 8080 of the private instance 'my-instance' and use port 5500 for the tunnel
			$ kraft cloud -p 5500 my-instance:8080

			# Forward the local ports 8080,8081 to the tcp ports 8080,8081 of the private instance 'my-instance' and use ports 5500,5505 for the tunnel
			$ kraft cloud -p 5500 -p 5505 my-instance:8080 my-instance2:8081

			# Forward the local ports 8080,8081 to the tcp ports 8080,8081 of the private instance 'my-instance' and use ports 5500,5501 for the tunnel
			$ kraft cloud my-instance:8080 my-instance2:8081
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
	if err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token); err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *TunnelOptions) Run(ctx context.Context, args []string) error {
	var err error

	// If no proxy ports are provided, default to 4444
	if len(opts.ProxyPorts) == 0 {
		opts.ProxyPorts = []string{"4444"}
	}

	for _, port := range opts.ProxyPorts {
		if parsed, err := strconv.ParseUint(port, 10, 16); err != nil {
			return fmt.Errorf("%q is not a valid port number", port)
		} else {
			opts.parsedProxyPorts = append(opts.parsedProxyPorts, uint16(parsed))
		}
	}

	if len(opts.ProxyPorts) > 1 && len(opts.ProxyPorts) != len(args) {
		return fmt.Errorf("supplied number of proxy ports must match the number of ports to forward")
	}

	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.Token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	if err := opts.parseArgs(ctx, args); err != nil {
		return fmt.Errorf("could not parse arguments: %w", err)
	}

	var authStr string
	cliInstance := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	).WithMetro(opts.Metro)

	opts.instances, err = populatePrivateIPs(ctx, cliInstance, opts.instances)
	if err != nil {
		return fmt.Errorf("could not populate private IPs: %w", err)
	}

	authStr, err = utils.GenRandAuth()
	if err != nil {
		return fmt.Errorf("could not generate random authentication string: %w", err)
	}

	instArgs := opts.formatProxyArgs(authStr)

	instID, sgFQDN, err := opts.runProxy(ctx, cliInstance, instArgs)
	if err != nil {
		return fmt.Errorf("could not run proxy: %w", err)
	}
	defer func() {
		err := terminateProxy(context.TODO(), cliInstance, instID)
		if err != nil {
			log.G(ctx).Errorf("could not terminate proxy: %v\n", err)
		}
	}()

	// Control relay used for keeping the connection up
	cr := Relay{
		rAddr: net.JoinHostPort(sgFQDN, strconv.FormatUint(uint64(opts.ProxyControlPort), 10)),
		auth:  authStr,
	}
	ready := make(chan struct{}, 1)
	go func() {
		err := cr.ControlUp(ctx, ready)
		if err != nil {
			log.G(ctx).Errorf("could not start control relay: %v\n", err)
		}
	}()
	// Wait for the control relay to be ready to be able to connect
	<-ready

	r := Relay{
		// TODO(antoineco): allow dual-stack by creating two separate listeners.
		// Alternatively, we could have defaulted to the address "::" to create a
		// tcp46 socket, but listening on all addresses is an insecure default.
		lAddr: net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(opts.localPorts[0]), 10)),
		rAddr: net.JoinHostPort(sgFQDN, strconv.FormatUint(uint64(opts.exposedProxyPorts[0]), 10)),

		// NOTE(craciunoiuc): Only TCP is supported at the moment. This refers to the
		// local listener, as the remote listener is always assumed to be TCP because
		// of TLS.
		ctype: opts.ctypes[0],
		auth:  authStr,
		name:  instID,
	}

	for i := range opts.localPorts {
		if i == 0 {
			continue
		}

		pr := Relay{
			lAddr: net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(opts.localPorts[i]), 10)),
			rAddr: net.JoinHostPort(sgFQDN, strconv.FormatUint(uint64(opts.exposedProxyPorts[i]), 10)),
			ctype: opts.ctypes[i],
			auth:  authStr,
			name:  instID,
		}

		go func() {
			err := pr.Up(ctx)
			if err != nil {
				log.G(ctx).Errorf("could not start relay: %v\n", err)
			}
		}()
	}

	return r.Up(ctx)
}

// generatePort generates a port number based on the startPort and the portIterator.
// This is used when a single proxy port is provided and multiple ports are to be forwarded.
func (opts *TunnelOptions) generatePort(startPort uint16) uint16 {
	defer func() {
		opts.portIterator++
	}()
	return startPort + opts.portIterator
}

// parseArgs parses the command line arguments into the instance, local port, remote port and connection type.
func (opts *TunnelOptions) parseArgs(ctx context.Context, args []string) error {
	for i, arg := range args {
		instance, lport, rport, ctype, err := parsePorts(ctx, arg)
		if err != nil {
			return err
		}

		opts.instances = append(opts.instances, instance)
		opts.localPorts = append(opts.localPorts, lport)
		opts.instanceProxyPorts = append(opts.instanceProxyPorts, rport)
		opts.ctypes = append(opts.ctypes, ctype)

		if len(opts.ProxyPorts) == 1 {
			opts.exposedProxyPorts = append(opts.exposedProxyPorts, opts.generatePort(opts.parsedProxyPorts[0]))
		} else {
			opts.exposedProxyPorts = append(opts.exposedProxyPorts, opts.parsedProxyPorts[i])
		}
	}

	return nil
}

// runProxy runs a proxy instance with the given arguments.
// Information related to the proxy instance is hardcoded, but the UUID is returned.
func (opts *TunnelOptions) runProxy(ctx context.Context, cli kcinstances.InstancesService, args []string) (string, string, error) {
	var parsedPorts []kcservices.CreateRequestService
	for i := range opts.exposedProxyPorts {
		parsedPorts = append(parsedPorts, kcservices.CreateRequestService{
			Port:            int(opts.exposedProxyPorts[i]),
			DestinationPort: ptr(int(opts.exposedProxyPorts[i])),
			Handlers: []kcservices.Handler{
				kcservices.HandlerTLS,
			},
		})
	}
	parsedPorts = append(parsedPorts, kcservices.CreateRequestService{
		Port:            int(opts.ProxyControlPort),
		DestinationPort: ptr(int(opts.ProxyControlPort)),
		Handlers: []kcservices.Handler{
			kcservices.HandlerTLS,
		},
	})

	crinstResp, err := cli.Create(ctx, kcinstances.CreateRequest{
		Image:    opts.TunnelServiceImage,
		MemoryMB: ptr(64),
		Args:     args,
		ServiceGroup: &kcinstances.CreateRequestServiceGroup{
			Services: parsedPorts,
		},
		Autostart:     ptr(true),
		WaitTimeoutMs: ptr(int((3 * time.Second).Milliseconds())),
		Features:      []kcinstances.Feature{kcinstances.FeatureDeleteOnStop},
	})
	if err != nil {
		return "", "", fmt.Errorf("creating proxy instance: %w", err)
	}
	inst, err := crinstResp.FirstOrErr()
	if err != nil {
		return "", "", fmt.Errorf("creating proxy instance: %w", err)
	}

	return inst.UUID, inst.ServiceGroup.Domains[0].FQDN, nil
}

// parsePorts parses a command line argument in the format [lport:]rport[/ctype] into
// two port numbers lport and rport. If lport isn't set, a random port will be
// used by the relay. If ctype isn't set, the connection will be assumed to be TCP.
func parsePorts(ctx context.Context, portsArg string) (instance string, lport, rport uint16, ctype string, err error) {
	types := strings.SplitN(portsArg, "/", 2)
	if len(types) == 2 {
		ctype = types[1]
	} else {
		ctype = "tcp"
	}

	if strings.ToLower(ctype) != "tcp" {
		log.G(ctx).Warn("only TCP connections are supported at the moment")
	}

	ports := strings.SplitN(types[0], ":", 3)

	if len(ports) == 2 {
		if _, err := strconv.ParseUint(ports[0], 10, 16); err == nil {
			return "", 0, 0, "", fmt.Errorf("%q is not a valid instance", ports[0])
		}

		rport64, err := strconv.ParseUint(ports[1], 10, 16)
		if err != nil {
			return "", 0, 0, "", fmt.Errorf("%q is not a valid port number", ports[1])
		}
		return ports[0], uint16(rport64), uint16(rport64), ctype, nil
	}

	lport64, err := strconv.ParseUint(ports[0], 10, 16)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("%q is not a valid port number", ports[0])
	}

	rport64, err := strconv.ParseUint(ports[2], 10, 16)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("%q is not a valid port number", ports[1])
	}

	return ports[1], uint16(lport64), uint16(rport64), ctype, nil
}

// formatProxyArgs formats the arguments to be passed to the proxy instance.
func (opts *TunnelOptions) formatProxyArgs(authStr string) []string {
	var connections []string

	for i := range opts.instances {
		connections = append(connections,
			fmt.Sprintf("TCP2%s:%s:%d:%d:%d",
				strings.ToUpper(opts.ctypes[i]),
				opts.instances[i],
				opts.instanceProxyPorts[i],
				opts.exposedProxyPorts[i],
				27,
			),
		)
	}

	var allConnections string
	for _, conn := range connections {
		allConnections += conn + "|"
	}
	allConnections = "[" + strings.TrimSuffix(allConnections, "|") + "]"

	return []string{
		// HEARTBEAT_PORT:CTLR_AUTH_TIMEOUT
		fmt.Sprintf("%d:%d", opts.ProxyControlPort, 5),
		// AUTH_TIMEOUT:AUTH_COOKIE
		fmt.Sprintf("%d:%s", 5, authStr),
		// EVS_TIMEOUT
		fmt.Sprintf("%d", 600),
		// [HOOKSTR0:<HOOKSTR0_ARGS>|HOOKSTR1:<HOOKSTR1_ARGS>...]
		allConnections,
	}
}

// populatePrivateIPs fetches the private IPs of the instances and replaces the instance names/uuids with the Private IPs.
func populatePrivateIPs(ctx context.Context, cli kcinstances.InstancesService, ips []string) ([]string, error) {
	var instancesToGet []string
	var indexesToGet []int
	for i := range ips {
		// If instance is not an IP (PrivateIP) or PrivateFQDN
		// assume it is a name/UUID and fetch the IP
		if net.ParseIP(ips[i]) == nil && !strings.HasSuffix(ips[i], ".internal") {
			instancesToGet = append(instancesToGet, ips[i])
			indexesToGet = append(indexesToGet, i)
		}
	}
	if len(instancesToGet) > 0 {
		instGetResp, err := cli.Get(ctx, instancesToGet...)
		if err != nil {
			return nil, fmt.Errorf("getting instances: %w", err)
		}

		for i, inst := range instGetResp.Data.Entries {
			if inst.PrivateIP == "" {
				return nil, fmt.Errorf("instance '%s' not found", instancesToGet[i])
			}
			ips[indexesToGet[i]] = inst.PrivateIP
		}
	}

	return ips, nil
}

// terminateProxy terminates the proxy instance with the given UUID.
func terminateProxy(ctx context.Context, icli kcinstances.InstancesService, instID string) error {
	delinstResp, err := icli.Delete(ctx, instID)
	if err != nil {
		return fmt.Errorf("deleting proxy instance '%s': %w", instID, err)
	}
	if _, err = delinstResp.FirstOrErr(); err != nil {
		return fmt.Errorf("deleting proxy instance '%s': %w", instID, err)
	}
	return nil
}

func ptr[T comparable](v T) *T { return &v }
