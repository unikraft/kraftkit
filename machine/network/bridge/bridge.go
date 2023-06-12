package bridge

import (
	"encoding/gob"

	"github.com/vishvananda/netlink"
)

const (
	// DefaultMTU is the default MTU for new bridge interfaces.
	DefaultMTU = 1500
)

func init() {
	gob.Register(&netlink.Bridge{})
}
