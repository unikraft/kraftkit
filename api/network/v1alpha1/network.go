package v1alpha1

import (
	"kraftkit.sh/unikraft/export/v0/uknetdev"
)

func ParseNetwork(networkAttr *NetworkAttr) (uknetdev.NetdevIp, error) {
	return uknetdev.NetdevIp{
		CIDR:     networkAttr.CIDR,
		Gateway:  networkAttr.Gateway,
		DNS0:     networkAttr.DNS0,
		DNS1:     networkAttr.DNS1,
		Hostname: networkAttr.Hostname,
		Domain:   networkAttr.Domain,
	}, nil
}
