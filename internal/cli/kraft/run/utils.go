package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/nerdctl/pkg/strutil"
	"github.com/dustin/go-humanize"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	machinename "kraftkit.sh/machine/name"
	"kraftkit.sh/machine/network"
	"kraftkit.sh/machine/volume"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
)

// Are we publishing ports? E.g. -p/--ports=127.0.0.1:80:8080/tcp ...
func (opts *RunOptions) parsePorts(_ context.Context, machine *machineapi.Machine) error {
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

// Was a network specified? E.g. --network=kraft0
func (opts *RunOptions) parseNetworks(ctx context.Context, machine *machineapi.Machine) error {
	if opts.Network == "" {
		if opts.IP != "" {
			return fmt.Errorf("cannot assign IP address without providing --network")
		}
		return nil
	}

	networkServiceIterator, err := network.NewNetworkV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	// Try to discover the user-provided network.
	found, err := networkServiceIterator.Get(ctx, &networkapi.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: opts.Network,
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
	found, err = networkServiceIterator.Update(ctx, found)
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
func (opts *RunOptions) assignName(ctx context.Context, machine *machineapi.Machine) error {
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
func (opts *RunOptions) parseVolumes(ctx context.Context, machine *machineapi.Machine) error {
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

// Were any volumes supplied in the Kraftfile
func (opts *RunOptions) parseKraftfileVolumes(ctx context.Context, project app.Application, machine *machineapi.Machine) error {
	if project.Volumes() == nil {
		return nil
	}

	var err error
	controllers := map[string]volumeapi.VolumeService{}
	if machine.Spec.Volumes == nil {
		machine.Spec.Volumes = make([]volumeapi.Volume, 0)
	}

	for _, volcfg := range project.Volumes() {
		driver := volcfg.Driver()

		if len(driver) == 0 {
			for sname, strategy := range volume.Strategies() {
				if ok, _ := strategy.IsCompatible(volcfg.Source(), nil); !ok || err != nil {
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
		}

		if len(driver) == 0 {
			return fmt.Errorf("could not find compatible volume driver for %s", volcfg.Source())
		}

		vol, err := controllers[driver].Create(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: volcfg.Source(),
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      driver,
				Source:      volcfg.Source(),
				Destination: volcfg.Destination(),
				ReadOnly:    volcfg.ReadOnly(),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create volume: %w", err)
		}

		machine.Spec.Volumes = append(machine.Spec.Volumes, *vol)
	}

	return nil
}

// parse the provided `--rootfs` flag which ultimately is passed into the
// dynamic Initrd interface which either looks up or constructs the archive
// based on the value of the flag.
func (opts *RunOptions) prepareRootfs(ctx context.Context, machine *machineapi.Machine) error {
	// If the user has supplied an initram path, set this now, this overrides any
	// preparation and is considered higher priority compared to what has been set
	// prior to this point.
	if opts.Rootfs == "" || machine.Status.InitrdPath != "" {
		return nil
	}

	machine.Status.InitrdPath = filepath.Join(
		opts.workdir,
		unikraft.BuildDir,
		fmt.Sprintf(initrd.DefaultInitramfsArchFileName, machine.Spec.Architecture),
	)

	ramfs, err := initrd.New(ctx,
		opts.Rootfs,
		initrd.WithOutput(machine.Status.InitrdPath),
		initrd.WithCacheDir(filepath.Join(
			opts.workdir,
			unikraft.BuildDir,
			"rootfs-cache",
		)),
		initrd.WithArchitecture(machine.Spec.Architecture),
	)
	if err != nil {
		return fmt.Errorf("could not prepare initramfs: %w", err)
	}

	if _, err = ramfs.Build(ctx); err != nil {
		return err
	}

	if fi, err := os.Stat(machine.Status.InitrdPath); err == nil {
		// Warn if the initrd path is greater than allocated memory
		memRequest := machine.Spec.Resources.Requests[corev1.ResourceMemory]
		if memRequest.Value() < fi.Size() {
			log.G(ctx).Warnf("requested memory (%s) is less than initramfs (%s)",
				humanize.Bytes(uint64(memRequest.Value())),
				humanize.Bytes(uint64(fi.Size())),
			)
		}
	}

	return nil
}
