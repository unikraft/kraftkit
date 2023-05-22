// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package stop

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/waitgroup"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	"kraftkit.sh/machine/driveropts"
)

type Stop struct {
	All bool `long:"all" usage:"Remove all machines"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Stop{}, cobra.Command{
		Short: "Stop one or more running unikernels",
		Use:   "stop [FLAGS] MACHINE [MACHINE [...]]",
		Args:  cobra.MinimumNArgs(0),
		Long: heredoc.Doc(`
			Stop one or more running unikernels`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

var (
	observations = waitgroup.WaitGroup[machine.MachineID]{}
	drivers      = make(map[machinedriver.DriverType]machinedriver.Driver)
)

func (opts *Stop) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	store, err := machine.NewMachineStoreFromPath(config.G[config.KraftKit](ctx).RuntimeDir)
	if err != nil {
		return fmt.Errorf("could not access machine store: %v", err)
	}

	mcfgs, err := store.ListAllMachineConfigs()
	if err != nil {
		return fmt.Errorf("could not list machines: %v", err)
	}

	var mids []machine.MachineID

	for _, mid1 := range args {
		found := false
		for _, mid2 := range mcfgs {
			if mid1 == mid2.ID.ShortString() || mid1 == mid2.ID.String() || mid1 == string(mid2.Name) {
				mids = append(mids, mid2.ID)
				found = true
			}
		}

		if !found {
			return fmt.Errorf("could not find machine %s", mid1)
		}
	}

	if len(args) == 0 && opts.All {
		mids = []machine.MachineID{}
		for _, mcfg := range mcfgs {
			mids = append(mids, mcfg.ID)
		}
	}

	for _, mid := range mids {
		mid := mid // loop closure

		if observations.Contains(mid) {
			continue
		}

		observations.Add(mid)

		go func() {
			observations.Add(mid)

			log.G(ctx).Infof("stopping %s", mid.ShortString())

			mcfg := &machine.MachineConfig{}
			if err := store.LookupMachineConfig(mid, mcfg); err != nil {
				log.G(ctx).Errorf("could not look up machine config: %v", err)
				observations.Done(mid)
				return
			}

			driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

			if _, ok := drivers[driverType]; !ok {
				driver, err := machinedriver.New(driverType,
					driveropts.WithMachineStore(store),
					driveropts.WithRuntimeDir(config.G[config.KraftKit](ctx).RuntimeDir),
				)
				if err != nil {
					log.G(ctx).Errorf("could not instantiate machine driver for %s: %v", mid.ShortString(), err)
					observations.Done(mid)
					return
				}

				drivers[driverType] = driver
			}

			driver := drivers[driverType]

			if err := driver.Stop(ctx, mid); err != nil {
				log.G(ctx).Errorf("could not stop machine %s: %v", mid.ShortString(), err)
			} else {
				log.G(ctx).Infof("stopped %s", mid.ShortString())
			}

			observations.Done(mid)
		}()
	}

	observations.Wait()

	return nil
}
