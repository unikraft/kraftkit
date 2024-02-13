package xen

import (
	"fmt"
	"testing"
)

// TODO: Delete before merge
func TestBasicSerialization(t *testing.T) {
	xcfg, err := NewXenConfig(
		WithCpu(1),
		WithMemory(1024),
		WithP9(P9Spec{
			Tag:  "p9",
			Path: "/tmp",
		}),
		WithNetwork(NetworkSpec{
			Mac:    "00:00:00:00:00:00",
			Ip:     "172.44.0.3",
			Bridge: "xenbr0",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	xcfgBytes, err := xcfg.MarshalXenSpec()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%v", string(xcfgBytes))
}
