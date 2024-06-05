package create

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/confirm"
	"kraftkit.sh/tui/textinput"
	"kraftkit.sh/unikraft/lib/template"
)

type CreateOptions struct {
	ProjectName     string `long:"project-name" usage:"Set the project name to the template"`
	LibraryName     string `long:"library-name" usage:"Set the library name to the template"`
	LibraryKName    string `long:"library-kname" usage:"Set the library kname to the template"`
	Version         string `long:"version" short:"v" usage:"Set the library version to the template"`
	Description     string `long:"description"  usage:"Set the description to the template"`
	AuthorName      string `long:"author-name" usage:"Set the author name to the template"`
	AuthorEmail     string `long:"author-email" usage:"Set the author email to the template"`
	InitialBranch   string `long:"initial-branch" usage:"Set the initial branch name to the template"`
	CopyrightHolder string `long:"copyright-holder" usage:"Set the copyright holder name to the template"`
	Origin          string `long:"origin" usage:"Source code origin URL"`
	NoProvideCMain  bool   `long:"no-provide-c-main" usage:"Do not provide C main to the template"`
	GitInit         bool   `long:"git-init" usage:"Init git through the creating library"`
	WithPatchdir    bool   `long:"patch-dir" usage:"provide patch directory to the template"`
	UpdateRefs      bool   `long:"update-refs" usage:"Softly pack the component so that it is available via kraft list"`
	ProjectPath     string `long:"project-path" usage:"Where to create library"`
}

// Create creates a library template.
func Create(ctx context.Context, opts *CreateOptions, args ...string) error {
	if opts == nil {
		opts = &CreateOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Initialize a library from a template",
		Use:     "create [FLAGS] [NAME]",
		Aliases: []string{"init"},
		Long: heredoc.Doc(`
			Creates a library template
		`),
		Example: heredoc.Doc(`
			# Create a library template
			$ kraft lib create

			# Create a library template with a name
			$ kraft lib create sample-project
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "lib",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}
	cmd.SetContext(ctx)
	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	var err error

	if len(args) > 0 {
		opts.ProjectName = args[0]
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if !config.G[config.KraftKit](ctx).NoPrompt {
		if !opts.GitInit {
			opts.GitInit, err = confirm.NewConfirm("Do you want to intialise library with git:", func() {})
			if err != nil {
				return err
			}
		}

		if len(opts.ProjectName) == 0 {
			opts.ProjectName, err = textinput.NewTextInput(
				"Project name:",
				"Project name cannot be empty",
				"",
			)
			if err != nil {
				return err
			}
		}

		if len(opts.ProjectPath) == 0 {
			opts.ProjectPath, err = textinput.NewTextInput(
				"Work directory:",
				"Where to create library template",
				cwd,
			)
			if err != nil {
				return err
			}
		}

		if !opts.UpdateRefs {
			opts.UpdateRefs, err = confirm.NewConfirm("Do you want to package it:", func() {})
			if err != nil {
				return err
			}
		}

		if len(opts.LibraryName) == 0 {
			opts.LibraryName, err = textinput.NewTextInput(
				"Library name:",
				"Library name cannot be empty",
				"lib-"+opts.ProjectName,
			)
			if err != nil {
				return err
			}
		}

		if len(opts.LibraryKName) == 0 {
			opts.LibraryKName, err = textinput.NewTextInput(
				"Library kname:",
				"Library kname cannot be empty",
				"LIB"+strings.ToUpper(strings.ReplaceAll(opts.ProjectName, "-", "")),
			)
			if err != nil {
				return err
			}
		}

		if len(opts.Description) == 0 {
			opts.Description, err = textinput.NewTextInput(
				"Description:",
				"Description",
				"",
			)
			if err != nil {
				return err
			}
		}

		if len(opts.Version) == 0 {
			opts.Version, err = textinput.NewTextInput(
				"Version:",
				"x.y.z",
				"1.0.0",
			)
			if err != nil {
				return err
			}
		}

		if len(opts.AuthorName) == 0 {
			opts.AuthorName, err = textinput.NewTextInput(
				"Author name:",
				"Author name cannot be empty",
				os.Getenv("USER"),
			)
			if err != nil {
				return err
			}
		}

		if len(opts.AuthorEmail) == 0 {
			initialValue := ""
			cmd := exec.Command("git", "config", "--get", "user.email")
			emailBytes, err := cmd.CombinedOutput()
			if err == nil {
				initialValue = string(emailBytes)
			}
			opts.AuthorEmail, err = textinput.NewTextInput(
				"Author email:",
				"Author email cannot be empty",
				initialValue,
			)
			if err != nil {
				return err
			}
		}

		if opts.GitInit && len(opts.InitialBranch) == 0 {
			opts.InitialBranch, err = textinput.NewTextInput(
				"Initial branch:",
				"Initial branch",
				"staging",
			)
			if err != nil {
				return err
			}
		}

		if len(opts.Origin) == 0 {
			opts.Origin, err = textinput.NewTextInput(
				"Origin url:",
				"Enter origin url",
				"",
			)
			if err != nil {
				return err
			}
		}

		if len(opts.CopyrightHolder) == 0 {
			opts.CopyrightHolder, err = textinput.NewTextInput(
				"Copyright holder:",
				"Copyright holder cannot be empty",
				opts.AuthorName,
			)
			if err != nil {
				return err
			}
		}
	} else {
		var errs []string

		if len(opts.ProjectName) == 0 {
			errs = append(errs, fmt.Errorf("project name cannot be empty").Error())
		}

		if len(opts.Version) == 0 {
			errs = append(errs, fmt.Errorf("version cannot be empty").Error())
		}

		if len(opts.AuthorName) == 0 {
			errs = append(errs, fmt.Errorf("author name cannot be empty").Error())
		}

		if len(opts.AuthorEmail) == 0 {
			errs = append(errs, fmt.Errorf("author email cannot be empty").Error())
		}

		if len(errs) > 0 {
			return fmt.Errorf(strings.Join(errs, "\n"))
		}

		if len(opts.LibraryName) == 0 {
			opts.LibraryName = "lib-" + opts.ProjectName
		}

		if len(opts.LibraryKName) == 0 {
			opts.LibraryKName = "LIB" + strings.ToUpper(strings.ReplaceAll(opts.ProjectName, "-", ""))
		}

		if len(opts.ProjectPath) == 0 {
			opts.ProjectPath = cwd
		}

		if len(opts.CopyrightHolder) == 0 {
			opts.CopyrightHolder = opts.AuthorName
		}

		if opts.GitInit {
			opts.InitialBranch = "staging"
		}
	}

	// Creating instance of Template
	templ, err := template.NewTemplate(ctx,
		template.WithGitInit(opts.GitInit),
		template.WithProjectName(opts.ProjectName),
		template.WithLibName(opts.LibraryName),
		template.WithLibKName(opts.LibraryKName),
		template.WithVersion(opts.Version),
		template.WithDescription(opts.Description),
		template.WithAuthorName(opts.AuthorName),
		template.WithAuthorEmail(opts.AuthorEmail),
		template.WithInitialBranch(opts.InitialBranch),
		template.WithCopyrightHolder(opts.CopyrightHolder),
		template.WithProvideCMain(!opts.NoProvideCMain),
		template.WithPatchdir(opts.WithPatchdir),
		template.WithOriginUrl(opts.Origin),
	)
	if err != nil {
		return err
	}

	if err = templ.Generate(ctx, opts.ProjectPath); err != nil {
		return err
	}

	// Packaging softly.
	if opts.UpdateRefs {
		packageManager := packmanager.G(ctx)
		if err = packageManager.AddSource(ctx, path.Join(opts.ProjectPath, opts.ProjectName)); err != nil {
			return err
		}
		config.G[config.KraftKit](ctx).Unikraft.Manifests = append(
			config.G[config.KraftKit](ctx).Unikraft.Manifests,
			path.Join(opts.ProjectPath, opts.ProjectName),
		)
		if err := config.M[config.KraftKit](ctx).Write(true); err != nil {
			return err
		}
		if err = packageManager.Update(ctx); err != nil {
			return err
		}
	}

	return nil
}
