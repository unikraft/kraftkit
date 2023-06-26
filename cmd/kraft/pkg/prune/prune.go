package prune

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
)

type Prune struct {
	Name string `long:"name" short:"n" usage:"Specify the package name that has to be pruned" default:""`
	All  bool   `long:"all" short:"a" usage:"Prunes all the packages available on the host machine"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Prune{}, cobra.Command{
		Short:   "Prune unikraft package available on the disc",
		Use:     "prune [FLAGS] [PACKAGE|DIR]",
		Aliases: []string{"pr"},
		Long: heredoc.Doc(`
		Prunes unikraft package available locally on the host in the directory $HOME/.local/share/kraftkit/sources
		`),
		Example: heredoc.Doc(`
			# Prunes unikraft package nginx.
			$ kraft pkg prune nginx
			$ kraft pkg prune nginx:stable`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Prune) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	return nil
}

func (opts *Prune) Run(cmd *cobra.Command, args []string) error {
	var userPackage string
	if len(args) == 0 && opts.Name == "" && !opts.All {
		return fmt.Errorf("Package name is not specified to prune")
	} else if opts.All && (len(args) > 0 || opts.Name != "") {
		return fmt.Errorf("Package name and --all flags cannot be specified at once")
	} else if len(args) == 0 {
		userPackage = opts.Name
	} else {
		userPackage = args[0]
	}
	var version, packName string

	if !opts.All {
		packNameAndVersion := strings.Split(userPackage, ":")
		if len(packNameAndVersion) < 2 {
			version = "stable"
		} else {
			version = packNameAndVersion[1]
		}
		packName = packNameAndVersion[0]
	}

	ctx := cmd.Context()

	err := packmanager.G(ctx).Prune(ctx,
		packmanager.WithName(packName),
		packmanager.WithVersion(version),
		packmanager.WithAll(opts.All),
	)
	if err != nil {
		return err
	}

	return nil
}
