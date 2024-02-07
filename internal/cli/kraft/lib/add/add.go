package add

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/pkg/pull"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/lib"
)

type AddOptions struct {
	Kraftfile string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	NoUpdate  bool   `long:"no-update" usage:"Do not update package index before running the build"`
	Workdir   string `long:"workdir" short:"w" usage:"workdir to add the package to"`
}

// Add adds a Unikraft library to the project directory and updates the Kraftfile
func Add(ctx context.Context, opts *AddOptions, args ...string) error {
	if opts == nil {
		opts = &AddOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&AddOptions{}, cobra.Command{
		Short:   "Add unikraft library to the project directory",
		Use:     "add [FLAGS] [PACKAGE|DIR]",
		Args:    cmdfactory.MinimumArgs(1, "library name is not specified"),
		Aliases: []string{"a"},
		Long: heredoc.Doc(`
			Pull a Unikraft component microlibrary from a remote location
			and add to the project directory
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "lib",
		},
		Example: heredoc.Doc(`
			# Add a local library to the project
			$ kraft lib add path/to/library

			# Add a library from a source repository
			$ kraft lib add github.com/unikraft/lib-nginx.git

			# Add from a manifest
			$ kraft lib add nginx:staging

			# Add a library from a registry
			$ kraft lib add unikraft.org/nginx:stable
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *AddOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}
	cmd.SetContext(ctx)
	return nil
}

func (opts *AddOptions) Run(ctx context.Context, args []string) error {
	var workdir, packName, packVersion string
	var packType unikraft.ComponentType
	var err error
	var library lib.LibraryConfig
	isPackUndefindable := false
	packageManager := packmanager.G(ctx)

	if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
		if err = packageManager.AddSource(ctx, args[0]); err != nil {
			return err
		}
		config.G[config.KraftKit](ctx).Unikraft.Manifests = append(
			config.G[config.KraftKit](ctx).Unikraft.Manifests,
			args[0],
		)
		if err := config.M[config.KraftKit](ctx).Write(true); err != nil {
			return err
		}
		if err = packageManager.Update(ctx); err != nil {
			return err
		}
		tempStrs := strings.Split(args[0], "/")
		packName = tempStrs[len(tempStrs)-1]
		packVersion = "default"
		args[0] = packName + ":" + packVersion
	} else {
		// Verifying if user demands for a library otherwise return error.
		packType, packName, packVersion, err = unikraft.GuessTypeNameVersion(args[0])
		if err != nil {
			isPackUndefindable = true
			tempStrs := strings.Split(args[0], "/")
			tempStr := tempStrs[len(tempStrs)-1]
			tempStrs = strings.Split(tempStr, ".")
			tempStr = tempStrs[0]

			packType, packName, packVersion, err = unikraft.GuessTypeNameVersion(tempStr)
			if err != nil {
				return err
			}
		}

		if packType != unikraft.ComponentTypeLib && packType != unikraft.ComponentTypeUnknown {
			return fmt.Errorf("specified package %s is not a library", args[0])
		}
	}

	if !isPackUndefindable && !strings.HasPrefix(args[0], "lib") {
		args[0] = "lib/" + args[0]
	}

	// Pulling library.
	if err = pull.Pull(ctx, &pull.PullOptions{}, args...); err != nil {
		return err
	}

	popts := []app.ProjectOption{}
	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	project, err := app.NewProjectFromOptions(
		ctx,
		append(popts, app.WithProjectWorkdir(workdir))...,
	)
	if err != nil {
		return err
	}

	packs, err := packageManager.Catalog(ctx,
		packmanager.WithName(packName),
		packmanager.WithTypes(unikraft.ComponentTypeLib),
		packmanager.WithVersion(packVersion),
		packmanager.WithUpdate(!opts.NoUpdate),
	)
	if err != nil {
		return err
	}

	if len(packs) == 0 {
		return fmt.Errorf("specified library not found")
	} else if len(packs) > 1 {
		return fmt.Errorf("found more than one library")
	}
	library, err = lib.NewLibraryFromPackage(ctx, packs[0])
	if err != nil {
		return err
	}

	if err = project.AddLibrary(ctx, library); err != nil {
		return err
	}

	return project.Save(ctx)
}
