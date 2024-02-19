package xen

import (
	"encoding/gob"
)

func init() {
	gob.Register(XenConfig{})
}
