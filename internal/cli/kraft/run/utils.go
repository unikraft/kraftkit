package run

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/nerdctl/pkg/strutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/log"
	machinename "kraftkit.sh/machine/name"
	"kraftkit.sh/machine/network"
	"kraftkit.sh/machine/volume"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
)

// Are we publishing ports? E.g. -p/--ports=127.0.0.1:80:8080/tcp ...
func (opts *RunOptions) assignPorts(ctx context.Context, machine *machineapi.Machine) error {
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

	return utils.CheckPorts(ctx, opts.machineController, machine)
}

// Was a network specified? E.g. --network=kraft0
func (opts *RunOptions) parseNetworks(ctx context.Context, machine *machineapi.Machine) error {
	if opts.IP != "" && len(opts.Networks) != 1 {
		return fmt.Errorf("the --ip flag only works when providing exactly one network")
	}

	if len(opts.Networks) == 0 {
		return nil
	}

	machineNetworks := []networkapi.NetworkSpec{}

	for _, networkArg := range opts.Networks {

		// The network is specified in the format
		// network:[cidr[:gw[:dns0[:dns1[:hostname[:domain]]]]]]

		split := strings.SplitN(networkArg, ":", 2)
		networkName := split[0]

		networkServiceIterator, err := network.NewNetworkV1alpha1ServiceIterator(ctx)
		if err != nil {
			return err
		}

		// Try to discover the user-provided network.
		found, err := networkServiceIterator.Get(ctx, &networkapi.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name: networkName,
			},
		})
		if err != nil {
			return err
		}

		var interfaceSpec networkapi.NetworkInterfaceSpec

		if len(split) > 1 {
			fields := strings.Split(split[1], ":")
			if len(fields) > 0 && fields[0] != "" {
				interfaceSpec.CIDR = fields[0]
				ipMaskSplit := strings.SplitN(interfaceSpec.CIDR, "/", 2)
				if len(ipMaskSplit) != 2 {
					sz, _ := net.IPMask(net.ParseIP(found.Spec.Netmask).To4()).Size()
					interfaceSpec.CIDR = fmt.Sprintf("%s/%d", interfaceSpec.CIDR, sz)
				}
				opts.IP = ipMaskSplit[0]
			}

			if len(fields) > 1 {
				interfaceSpec.Gateway = fields[1]
			}
			if len(fields) > 2 {
				interfaceSpec.DNS0 = fields[2]
			}
			if len(fields) > 3 {
				interfaceSpec.DNS1 = fields[3]
			}
			if len(fields) > 4 {
				interfaceSpec.Hostname = fields[4]
			}
			if len(fields) > 5 {
				interfaceSpec.Domain = fields[5]
			}
		}

		if interfaceSpec.Gateway == "" {
			interfaceSpec.Gateway = found.Spec.Gateway
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
			Spec: interfaceSpec,
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

		found.Spec.Interfaces = []networkapi.NetworkInterfaceTemplateSpec{newIface}
		machineNetworks = append(machineNetworks, found.Spec)
	}

	// Set the interface on the machine.
	machine.Spec.Networks = machineNetworks

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

	if machine.Spec.Volumes == nil {
		machine.Spec.Volumes = make([]volumeapi.Volume, 0)
	}
	for _, volLine := range opts.Volumes {
		var volName, mountPath string
		split := strings.Split(volLine, ":")
		if len(split) == 2 {
			volName = split[0]
			mountPath = split[1]
		} else {
			return fmt.Errorf("invalid syntax for --volume=%s expected --volume=<host>:<machine>", volLine)
		}

		var driver string

		for sname, strategy := range volume.Strategies() {
			if ok, _ := strategy.IsCompatible(volName, nil); !ok || err != nil {
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
			return fmt.Errorf("could not find compatible volume driver for %s", volName)
		}

		// Check if this could be a named volume
		vol, err := controllers[driver].Get(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: volName,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to get volume: %w", err)
		}
		if vol != nil {
			vol.Spec.Destination = mountPath
			machine.Spec.Volumes = append(machine.Spec.Volumes, *vol)
			continue
		}

		vol, err = controllers[driver].Create(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%d", machine.ObjectMeta.Name, len(machine.Spec.Volumes)),
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      driver,
				Source:      volName,
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
					log.G(ctx).WithField("volume strategy", sname).Debug("found volume strategy")
					controllers[sname], err = strategy.NewVolumeV1alpha1(ctx)
					if err != nil {
						return fmt.Errorf("could not prepare %s volume service: %w", sname, err)
					}
				}

				driver = sname
			}
		} else {
			strategy, exists := volume.Strategies()[driver]
			if !exists {
				return fmt.Errorf("unknown volume driver %s specified", driver)
			}
			if ok, _ := strategy.IsCompatible(volcfg.Source(), nil); !ok || err != nil {
				return fmt.Errorf("volume driver %s is incompatible with source %s", driver, volcfg.Source())
			}
			if _, ok := controllers[driver]; !ok {
				log.G(ctx).WithField("volume strategy", driver).Debug("found volume strategy")
				controllers[driver], err = strategy.NewVolumeV1alpha1(ctx)
				if err != nil {
					return fmt.Errorf("could not prepare %s volume service: %w", driver, err)
				}
			}
		}

		if len(driver) == 0 {
			return fmt.Errorf("could not find compatible volume driver for %s", volcfg.Source())
		}

		// Check if this could be a named volume
		vol, err := controllers[driver].Get(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: volcfg.Source(),
			},
		})

		if err == nil && vol.Spec.Source != "" {
			vol.Spec.Destination = volcfg.Destination()
			machine.Spec.Volumes = append(machine.Spec.Volumes, *vol)
			continue
		}

		vol, err = controllers[driver].Create(ctx, &volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%d", machine.ObjectMeta.Name, len(machine.Spec.Volumes)),
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
		initrd.WithWorkdir(opts.workdir),
	)
	if err != nil {
		return fmt.Errorf("could not prepare initramfs: %w", err)
	}

	if machine.Spec.Env == nil {
		machine.Spec.Env = make(map[string]string)
	}
	treemodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			processtree.WithFailFast(true),
		},
		processtree.NewProcessTreeItem(
			"building rootfs",
			machine.Spec.Architecture,
			func(ctx context.Context) error {
				if _, err = ramfs.Build(ctx); err != nil {
					return err
				}

				for _, entry := range ramfs.Env() {
					k, v, ok := strings.Cut(entry, "=")
					if !ok {
						continue
					}

					machine.Spec.Env[k] = v
				}

				if len(machine.Spec.ApplicationArgs) == 0 {
					machine.Spec.ApplicationArgs = ramfs.Args()
				}

				return nil
			},
		),
	)
	if err != nil {
		return err
	}

	return treemodel.Start()
}

func (opts *RunOptions) parseKraftfileEnv(_ context.Context, project app.Application, machine *machineapi.Machine) error {
	if project.Env() == nil {
		return nil
	}

	if machine.Spec.Env == nil {
		machine.Spec.Env = make(map[string]string)
	}

	for k, v := range project.Env() {
		if v != "" {
			machine.Spec.Env[k] = v
			continue
		}

		if v, ok := os.LookupEnv(k); ok {
			machine.Spec.Env[k] = v
		}
	}

	return nil
}

func (opts *RunOptions) parseEnvs(_ context.Context, machine *machineapi.Machine) error {
	if machine.Spec.Env == nil {
		machine.Spec.Env = make(map[string]string)
	}

	for _, env := range opts.Env {
		k, v, ok := strings.Cut(env, "=")
		if ok {
			machine.Spec.Env[k] = v
		} else if v, ok := os.LookupEnv(k); ok {
			machine.Spec.Env[k] = v
		}
	}

	return nil
}
