package kconfig

import (
	"github.com/spf13/cobra"
	"kraftkit.sh/cmd/kraft/kconfig/dump"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/log"
)

type KConfig struct{}

func New() *cobra.Command {
	// generate a Library command that does not do anything but allows me to register a subcommand called info

	cmd := cmdfactory.New(&KConfig{}, cobra.Command{
		Short: "Managing dependencies",
		Use:   "kconfig [FLAGS] [SUBCOMMAND|DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "misc",
		},
	})

	cmd.AddCommand(dump.New())

	return cmd
}

func (k *KConfig) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	log.G(ctx).Infof("Usage: kraft kconfig dump")
	return nil
}
