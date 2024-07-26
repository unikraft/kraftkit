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
	TunnelProxyPorts   []string `local:"true" long:"tunnel-proxy-port" short:"p" usage:"Remote port exposed by the tunnelling service(s). (default start port is 4444)"`
	ProxyControlPort   uint     `local:"true" long:"tunnel-control-port" short:"P" usage:"Command-and-control port used by the tunneling service(s)." default:"4443"`
	TunnelServiceImage string   `local:"true" long:"tunnel-image" usage:"Tunnel service image" default:"official/utils/tunnel:latest"`
	Token              string   `noattribute:"true"`
	Metro              string   `noattribute:"true"`

	// parsedProxyPorts contains the parsed ProxyPorts converted from string to uint16
	parsedProxyPorts []uint16
	// instances (name/uuid/private-ip) gets turned into private-ip after fetching
	instances []string
	// localPorts to forward on the local machine
	localPorts []uint16
	// ctype is the connection type of the port to forward (tcp/udp)
	ctypes []string
	// instanceProxyPorts is the port to forward of the instance
	instanceProxyPorts []uint16
	// exposedProxyPorts is the port to expose the proxy on
	exposedProxyPorts []uint16
	// portIterator for when a single proxy port is provided
	portIterator uint16
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&TunnelOptions{}, cobra.Command{
		Short: "Forward a local port to an unexposed instance through an intermediate TLS tunnel service",
		Long: heredoc.Docf(`
			Forward a local port to an unexposed instance through an intermediate TLS
			tunnel service.

			When you need to access an instance on Unikraft Cloud which is not
			publicly exposed to the internet, you can use the
			%[1]skraft cloud tunnel%[1]s subcommand to forward from a local port to a
			port which the instance listens on.

			The %[1]skraft cloud tunnel%[1]s subcommand creates a secure tunnel
			between your local machine and the private instance(s).  The tunnel is
			created using an intermediate TLS tunnel service which is another instance
			running as a sidecar along with the target instance in the same private
			network.  The tunnel service listens on a publicly exposed port on the
			cloud and forwards the traffic to the private instance.

			When you run the %[1]skraft cloud tunnel%[1]s subcommand, you specify the
			local port to forward, the private instance to connect to, and the port on
			the private instance to connect to.

			It is also possible to customize the remote port which the tunnel service
			exposes and the command-and-control port used by the tunnel service.  By
			default, the remote port is %[1]s4444%[1]s and the command-and-control
			port is %[1]s4443%[1]s.
		`, "`"),
		Use:  "tunnel [FLAGS] [LOCAL_PORT:]<INSTANCE|PRIVATE_IP|PRIVATE_FQDN>:DEST_PORT[/TYPE] ...",
		Args: cobra.MinimumNArgs(1),
		Example: heredoc.Docf(`
			# Forward to the TCP port of %[1]s8080%[1]s of the unexposed instance
			# identified by its name "nginx" which then becomes locally accessible
			# also at %[1]s8080%[1]s:
			$ kraft cloud tunnel nginx:8080

			# Forward to the TCP port of 8080 of the unexposed instance based on its
			# private FQDN %[1]snginx.internal%[1]s which then becomes locally
			# accessible also at %[1]s8080%[1]s:
			$ kraft cloud tunnel nginx.internal:8080

			# Forward to the TCP port of %[1]s8080%[1]s of the unexposed instance
			# based on its private IP %[1]s172.16.28.8%[1]s which then becomes locally
			# accessible also at %[1]s8080%[1]s:
			$ kraft cloud tunnel 172.16.28.8:8080

			# Forward to the UDP port of %[1]s8123%[1]s of the unexposed instance
			# based on its private IP %[1]s172.16.22.2%[1]s which then becomes locally
			# accessible also at %[1]s8123%[1]s:
			$ kraft cloud tunnel 172.16.22.2:8123/udp

			# Forward to the TCP port of %[1]s8080%[1]s of the unexposed instance by
			# its name "nginx" which then becomes locally accessible at
			# %[1]s8333%[1]s:
			$ kraft cloud tunnel 8333:nginx:8080

			# Forward multiple ports from multiple instances
			$ kraft cloud tunnel 8080:my-instance1:8080/tcp 8443:my-instance2:8080/tcp

			# In the circumstance where the port you wish to connect to of the
			# instance is the same as the remote port exposed by the tunnelling
			# service (or the the command-and-control port of the tunneling service),
			# you can use the -p and -P flag to set alternative relay and command-
			# and-control ports.
			#
			# Tunnel to the instance 'my-instance' on port 8080 via the intermediate
			# 5500 port
			$ kraft cloud tunnel -p 5500 my-instance:8080
		`, "`"),
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
	if len(opts.TunnelProxyPorts) == 0 {
		opts.TunnelProxyPorts = []string{"4444"}
	}

	for _, port := range opts.TunnelProxyPorts {
		if parsed, err := strconv.ParseUint(port, 10, 16); err != nil {
			return fmt.Errorf("%q is not a valid port number", port)
		} else {
			opts.parsedProxyPorts = append(opts.parsedProxyPorts, uint16(parsed))
		}
	}

	if len(opts.TunnelProxyPorts) > 1 && len(opts.TunnelProxyPorts) != len(args) {
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

	rawInstances := opts.instances
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
		ctype:    opts.ctypes[0],
		auth:     authStr,
		name:     instID,
		nameAddr: fmt.Sprintf("%s:%d", rawInstances[0], opts.instanceProxyPorts[0]),
	}

	for i := range opts.localPorts {
		if i == 0 {
			continue
		}

		pr := Relay{
			lAddr:    net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(opts.localPorts[i]), 10)),
			rAddr:    net.JoinHostPort(sgFQDN, strconv.FormatUint(uint64(opts.exposedProxyPorts[i]), 10)),
			ctype:    opts.ctypes[i],
			auth:     authStr,
			name:     instID,
			nameAddr: fmt.Sprintf("%s:%d", rawInstances[i], opts.instanceProxyPorts[i]),
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

		if len(opts.TunnelProxyPorts) == 1 {
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
func populatePrivateIPs(ctx context.Context, cli kcinstances.InstancesService, targets []string) (ips []string, err error) {
	var instancesToGet []string
	var indexesToGet []int
	for i := range targets {
		ips = append(ips, targets[i])
		// If instance is not an IP (PrivateIP) or PrivateFQDN
		// assume it is a name/UUID and fetch the IP
		if net.ParseIP(targets[i]) == nil && !strings.HasSuffix(targets[i], ".internal") {
			instancesToGet = append(instancesToGet, targets[i])
			indexesToGet = append(indexesToGet, i)
		}
	}
	if len(instancesToGet) > 0 {
		instGetResp, err := cli.Get(ctx, instancesToGet...)
		if err != nil {
			return nil, fmt.Errorf("getting instances: %w", err)
		}

		instances, err := instGetResp.AllOrErr()
		if err != nil {
			return nil, fmt.Errorf("getting instances: %w", err)
		}

		for i, inst := range instances {
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
