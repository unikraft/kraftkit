package bridge

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/erikh/ping"
	"github.com/vishvananda/netlink"
	"kraftkit.sh/internal/set"
)

// IpToBigInt converts a 4 bytes IP into a 128 bit integer.
func IPToBigInt(ip net.IP) *big.Int {
	x := big.NewInt(0)
	if ip4 := ip.To4(); ip4 != nil {
		return x.SetBytes(ip4)
	}
	if ip6 := ip.To16(); ip6 != nil {
		return x.SetBytes(ip6)
	}
	return nil
}

// BigIntToIP converts 128 bit integer into a 4 bytes IP address.
func BigIntToIP(v *big.Int) net.IP {
	return net.IP(v.Bytes())
}

// Increases IP address numeric value by 1.
func IncreaseIP(ip net.IP) net.IP {
	rawip := IPToBigInt(ip)
	rawip.Add(rawip, big.NewInt(1))
	return BigIntToIP(rawip)
}

// IsUnicastIP returns true if the provided IP address and network mask is a
// unicast address.
func IsUnicastIP(ip net.IP, mask net.IPMask) bool {
	// broadcast v4 ip
	if len(ip) == net.IPv4len && binary.BigEndian.Uint32(ip)&^binary.BigEndian.Uint32(mask) == ^binary.BigEndian.Uint32(mask) {
		return false
	}

	// global unicast
	return ip.IsGlobalUnicast()
}

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
		ip = IncreaseIP(ip)

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
		case !IsUnicastIP(ip, ipnet.Mask):
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
