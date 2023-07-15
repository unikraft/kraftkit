package run

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/nerdctl/pkg/strutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	machinename "kraftkit.sh/machine/name"
	"kraftkit.sh/machine/volume"
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

// Was a volume specified? E.g. --volume=path:path
func (opts *Run) parseVolumes(ctx context.Context, machine *machineapi.Machine) error {
	if len(opts.Volumes) == 0 {
		return nil
	}

	var err error
	controllers := map[string]volumeapi.VolumeService{}
	machine.Spec.Volumes = []volumeapi.Volume{}

	for _, volLine := range opts.Volumes {
		var hostPath, mountPath string
		split := strings.Split(volLine, ":")
		if len(split) == 2 {
			hostPath = split[0]
			mountPath = split[1]
		} else {
			return fmt.Errorf("invalid syntax for --volume=%s expected --volume=<host>:<machine>", volLine)
		}

		var driver string

		for sname, strategy := range volume.Strategies() {
			if ok, _ := strategy.IsCompatible(hostPath, nil); !ok || err != nil {
				continue
			}

			if _, ok := controllers[sname]; !ok {
				controllers[sname], err = strategy.NewVolumeV1alpha1(ctx)
				if err != nil {
					return fmt.Errorf("could not prepare %s volume service: %w", sname, err)
				}
			}

			driver = sname
		}

		if len(driver) == 0 {
			return fmt.Errorf("could not find compatible volume driver for %s", hostPath)
		}

		vol, err := controllers[driver].Create(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: hostPath,
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      driver,
				Source:      hostPath,
				Destination: mountPath,
				ReadOnly:    false, // TODO(nderjung): Options are not yet supported.
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create volume: %w", err)
		}

		machine.Spec.Volumes = append(machine.Spec.Volumes, *vol)
	}

	return nil
}
