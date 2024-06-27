package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/ghrepo"
	"kraftkit.sh/manifest"
	"kraftkit.sh/pack"
	kcclient "sdk.kraft.cloud/client"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"
)

const treeSeparator = "/tree/"

type deployerKraftfileRepo struct {
	args []string
	url  string
}

func (d *deployerKraftfileRepo) Name() string {
	return "kraftfile-repo"
}

func (d *deployerKraftfileRepo) String() string {
	if len(d.args) == 0 {
		return "run the given link with a Kraftfile"
	}

	return fmt.Sprintf("run the detected Kraftfile in the given link after cloning and use '%s' as arg(s)", strings.Join(d.args, " "))
}

func (d *deployerKraftfileRepo) Deployable(ctx context.Context, opts *DeployOptions, args ...string) (bool, error) {
	url := args[0]

	if !strings.Contains(url, "github.com") {
		return false, nil
	}

	if strings.Contains(url, treeSeparator) {
		url = strings.Split(url, treeSeparator)[0]
	}

	_, err := ghrepo.NewFromURL(url)
	if err != nil {
		return false, err
	}

	d.url = args[0]
	d.args = args[1:]

	return true, nil
}

// getAllBranchesSorted returns all branches of a given repository sorted
// by size in descending order.
// If no token is specified, it will only have access to public repositories
func getAllBranchesSorted(ctx context.Context, owner, repo, token string) ([]string, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	var allBranches []*github.Branch
	opt := &github.BranchListOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		branches, resp, err := client.Repositories.ListBranches(ctx, owner, repo, opt)
		if err != nil {
			return nil, err
		}
		allBranches = append(allBranches, branches...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var branchNames []string
	for _, branch := range allBranches {
		branchNames = append(branchNames, branch.GetName())
	}

	// Sort all branches names by size in descending order
	// This is done to ensure that the longest name is the first one
	sort.Slice(branchNames, func(i, j int) bool {
		return len(branchNames[i]) > len(branchNames[j])
	})

	return branchNames, nil
}

func (d *deployerKraftfileRepo) Deploy(ctx context.Context, opts *DeployOptions, _ ...string) (*kcclient.ServiceResponse[kcinstances.GetResponseItem], *kcclient.ServiceResponse[kcservices.GetResponseItem], error) {
	var err error
	var ghProvider manifest.Provider

	link := d.url
	branch := ""
	path := "."

	if strings.Contains(d.url, treeSeparator) {
		split1 := strings.SplitN(d.url, treeSeparator, 2)
		link = split1[0]
	}

	repo, err := ghrepo.NewFromURL(link)
	if err != nil {
		return nil, nil, err
	}

	if strings.Contains(d.url, treeSeparator) {
		branchPath := strings.SplitN(d.url, treeSeparator, 2)[1]

		token := ""
		for key, auth := range config.G[config.KraftKit](ctx).Auth {
			if auth.Endpoint == "github.com" || key == "github.com" {
				token = auth.Token
				break
			}
		}
		branches, err := getAllBranchesSorted(ctx, repo.RepoOwner(), repo.RepoName(), token)
		if err != nil {
			return nil, nil, err
		}

		for _, branchName := range branches {
			if strings.HasPrefix(branchPath, branchName) {
				branch = branchName
				break
			}
		}

		if branch == "" {
			return nil, nil, fmt.Errorf("could not match branch from given url, are you sure the url is correct?")
		}

		path = strings.SplitN(branchPath, branch+"/", 2)[1]
	}

	ghProvider, err = manifest.NewGitHubProvider(
		ctx,
		link,
		manifest.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		manifest.WithUpdate(true))
	if err != nil {
		return nil, nil, err
	}

	var m *manifest.Manifest = &manifest.Manifest{
		Type:     "app",
		Name:     repo.RepoName(),
		Origin:   link,
		Provider: ghProvider,
		Channels: []manifest.ManifestChannel{
			{
				Name:     branch,
				Default:  true,
				Resource: link,
			},
		},
	}

	p, err := manifest.NewPackageFromManifest(
		m,
		manifest.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		manifest.WithUpdate(true),
	)
	if err != nil {
		return nil, nil, err
	}

	err = p.Pull(
		ctx,
		pack.WithPullWorkdir(opts.Workdir),
		pack.WithPullUnstructured(true),
	)
	if err != nil {
		return nil, nil, err
	}

	opts.Workdir = filepath.Join(opts.Workdir, repo.RepoName(), path)

	deployers := []deployer{
		&deployerKraftfileRuntime{},
		&deployerKraftfileUnikraft{},
	}

	for _, deployer := range deployers {
		if deployable, _ := deployer.Deployable(ctx, opts, d.args...); deployable {
			return deployer.Deploy(ctx, opts, d.args...)
		}
	}

	return nil, nil, fmt.Errorf("no deployer found for the given project link")
}
