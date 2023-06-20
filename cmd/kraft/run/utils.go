package run

import (
	"context"
	"fmt"

	"github.com/containerd/nerdctl/pkg/strutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	machinename "kraftkit.sh/machine/name"
)

// Are we publishing ports? E.g. -p/--ports=127.0.0.1:80:8080/tcp ...
func (opts *Run) parsePorts(_ context.Context, machine *machineapi.Machine) error {
	if len(opts.Ports) == 0 {
		return nil
	}

	opts.Ports = strutil.DedupeStrSlice(opts.Ports)
	for _, port := range opts.Ports {
		parsed, err := machineapi.ParsePort(port)
		if err != nil {
			return err
		}
		machine.Spec.Ports = append(machine.Spec.Ports, parsed...)
	}

	return nil
}

// Was a network specified? E.g. --network=bridge:kraft0
func (opts *Run) parseNetworks(ctx context.Context, machine *machineapi.Machine) error {
	if opts.Network == "" {
		return nil
	}

	// Try to discover the user-provided network.
	found, err := opts.networkController.Get(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: opts.networkName,
		},
	})
	if err != nil {
		return err
	}

	// Generate the UID pre-emptively so that we can uniquely reference the
	// network interface which will allow us to clean it up later. Additionally,
	// it's OK if the IP or MAC address are empty, the network controller will
	// populate values if they are unset and will populate with new values
	// following the returning from the Update operation.
	newIface := networkapi.NetworkInterfaceTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			UID: uuid.NewUUID(),
		},
		Spec: networkapi.NetworkInterfaceSpec{
			IP:         opts.IP,
			MacAddress: opts.MacAddress,
		},
	}

	// Update the list of interfaces
	if found.Spec.Interfaces == nil {
		found.Spec.Interfaces = []networkapi.NetworkInterfaceTemplateSpec{}
	}
	found.Spec.Interfaces = append(found.Spec.Interfaces, newIface)

	// Update the network with the new interface.
	found, err = opts.networkController.Update(ctx, found)
	if err != nil {
		return err
	}

	// Only use the single new interface.
	for _, iface := range found.Spec.Interfaces {
		if iface.UID == newIface.UID {
			newIface = iface
			break
		}
	}

	// Set the interface on the machine.
	found.Spec.Interfaces = []networkapi.NetworkInterfaceTemplateSpec{newIface}
	machine.Spec.Networks = []networkapi.NetworkSpec{found.Spec}

	return nil
}

// assignName determines the machine instance's name either from a provided
// argument or randomly generates one.
func (opts *Run) assignName(ctx context.Context, machine *machineapi.Machine) error {
	if opts.Name == "" {
		machine.ObjectMeta.Name = machinename.NewRandomMachineName(0)
		return nil
	}

	// Check if this name has been previously used
	machines, err := opts.machineController.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	for _, found := range machines.Items {
		if opts.Name == found.Name {
			return fmt.Errorf("machine instance name already in use: %s", opts.Name)
		}
	}

	machine.ObjectMeta.Name = opts.Name

	return nil
}
