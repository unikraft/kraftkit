package dump

import (
	"os"
	"path/filepath"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/core"

	"github.com/MakeNowJust/heredoc"
	"github.com/sanity-io/litter"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/unikraft/app"
)

type KConfigDump struct {
	Workdir string `long:"workdir" short:"w" usage:"Set a path to working directory to dump"`
}

func New() *cobra.Command {
	return cmdfactory.New(&KConfigDump{}, cobra.Command{
		Short:   "Dump KConfig Tree",
		Use:     "dump",
		Aliases: []string{"d"},
		Long: heredoc.Doc(`
			Dump KConfig Tree for Unikraft and dependencies`),
		Example: heredoc.Doc(`
			# Dump KConfig tree
			$ kraft kconfig dump`),

		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "misc",
		},
	})
}

func (k *KConfigDump) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var err error

	if len(args) == 0 {
		k.Workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		k.Workdir = args[0]
	}

	// Interpret the project directory
	project, err := app.NewProjectFromOptions(
		ctx,
		app.WithProjectWorkdir(k.Workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	components, err := project.Components()
	if err != nil {
		return err
	}

	tree, err := components[0].(core.UnikraftConfig).KConfigTree(
		&kconfig.KeyValue{Key: "UK_BASE", Value: filepath.Join(k.Workdir, ".unikraft/unikraft")},
		&kconfig.KeyValue{Key: "UK_APP", Value: k.Workdir},

		&kconfig.KeyValue{Key: "UK_NAME", Value: "test"},
		&kconfig.KeyValue{Key: "BUILD_DIR", Value: filepath.Join(k.Workdir, "build")},

		&kconfig.KeyValue{Key: "ELIB_DIR", Value: ""},
		&kconfig.KeyValue{Key: "EPLAT_DIR", Value: ""},

		// Should be populated by kraftkit
		&kconfig.KeyValue{Key: "UK_FULLVERSION", Value: "0.12.0"},
		&kconfig.KeyValue{Key: "UK_CODENAME", Value: "JANUS"},
		&kconfig.KeyValue{Key: "UK_ARCH", Value: "x86_64"},
		&kconfig.KeyValue{Key: "UK_PLAT", Value: "kvm"},

		// Needed in arch/Config.uk to figure determine arm and arm64 support
		&kconfig.KeyValue{Key: "CC", Value: "clang"},
	)
	if err != nil {
		return err
	}

	litter.Dump(tree)
	return nil
}
