package bridge

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/erikh/ping"
	"github.com/vishvananda/netlink"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/machine/network/iputils"
)

// BridgeIPs returns all the IPs attached to the provided bridge
func BridgeIPs(bridge *netlink.Bridge) ([]string, error) {
	// get the neighbors
	var (
		list []netlink.Neigh
		err  error
	)

	list, err = netlink.NeighList(bridge.Index, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve IPv4 neighbor information for interface %s: %v", bridge.Name, err)
	}

	ips := make([]string, len(list))
	for i, entry := range list {
		ips[i] = entry.String()
	}

	return ips, nil
}

// For a given IP network, bridge (and its interface), allocate a free IP
// address.
func AllocateIP(ctx context.Context, ipnet *net.IPNet, iface *net.Interface, bridge *netlink.Bridge) (net.IP, error) {
	bridgeAddrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	allocatedIps, err := BridgeIPs(bridge)
	if err != nil {
		return nil, err
	}

	allocatedSet := set.NewStringSet(allocatedIps...)
	ip := ipnet.IP

search:
	for {
		ip = iputils.IncreaseIP(ip)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled")
		default:
		}

		switch {
		// If the IP is not within the provided network, it is not possible to
		// increment the IP so return with an error.
		case !ipnet.Contains(ip):
			return nil, fmt.Errorf("could not allocate IP address in %v", ipnet.String())

		// Skip the Bridge IP.
		case func() bool {
			for _, addr := range bridgeAddrs {
				itfIP, _, _ := net.ParseCIDR(addr.String())
				if ip.Equal(itfIP) {
					return true
				}
			}
			return false
		}():
			continue

		// Skip the broadcast IP address.
		case !iputils.IsUnicastIP(ip, ipnet.Mask):
			continue

		// Skip allocated IP addresses.
		case allocatedSet.Contains(ip.String()):
			continue

		// Use ICMP to check if the IP is in use as a final sanity check.
		case ping.Ping(&net.IPAddr{IP: ip, Zone: ""}, 150*time.Millisecond):
			continue

		default:
			break search
		}
	}

	return ip, nil
}
