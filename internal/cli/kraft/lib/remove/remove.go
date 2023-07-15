package remove

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
)

type RemoveOptions struct {
	Kraftfile string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Workdir   string `long:"workdir" short:"w" usage:"workdir to remove lib from"`
}

// Remove a Unikraft library from the project directory.
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
	if opts == nil {
		opts = &RemoveOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Removes a library dependency from the project directory",
		Use:     "remove [FLAGS] LIB",
		Aliases: []string{"rm"},
		Args:    cmdfactory.MinimumArgs(1, "library name is not specified to remove from the project"),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "lib",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}
	cmd.SetContext(ctx)
	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	var workdir string
	var err error

	if len(opts.Workdir) > 0 {
		workdir = opts.Workdir
	}

	if workdir == "." || workdir == "./" || workdir == "" {
		workdir, err = os.Getwd()
	}
	if err != nil {
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

	if err = project.RemoveLibrary(ctx, args[0]); err != nil {
		return err
	}

	return project.Save(ctx)
}
