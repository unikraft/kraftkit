package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/ghrepo"
	"kraftkit.sh/log"
	"kraftkit.sh/manifest"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft/app"
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
	if len(args) == 0 {
		return false, nil
	}

	url := args[0]
	baseUrl := url
	if strings.Contains(url, treeSeparator) {
		baseUrl = strings.Split(url, treeSeparator)[0]
	}
	_, err := ghrepo.NewFromURL(baseUrl)
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

// getDefaultBranch returns the default branch of a given repository as specified by GitHub
func getDefaultBranch(ctx context.Context, owner, repo, token string) (string, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	repository, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", err
	}

	if repository.DefaultBranch != nil {
		return *repository.DefaultBranch, nil
	}

	return "main", nil // fallback to main if not found
}

func (d *deployerKraftfileRepo) Deploy(ctx context.Context, opts *DeployOptions, _ ...string) (*kcclient.ServiceResponse[kcinstances.GetResponseItem], *kcclient.ServiceResponse[kcservices.GetResponseItem], error) {
	var manifests []*manifest.Manifest
	var m *manifest.Manifest
	// Setup link, branch, path, repo
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
	} else {
		// No explicit branch in URL, use GitHub's default branch
		token := ""
		for key, auth := range config.G[config.KraftKit](ctx).Auth {
			if auth.Endpoint == "github.com" || key == "github.com" {
				token = auth.Token
				break
			}
		}
		var err error
		branch, err = getDefaultBranch(ctx, repo.RepoOwner(), repo.RepoName(), token)
		if err != nil {
			log.G(ctx).Warnf("could not determine default branch: %v, defaulting to 'main'", err)
			branch = "main"
		}
		log.G(ctx).Debugf("Using GitHub default branch: %s", branch)
	}

	ghProvider, err := manifest.NewGitHubProvider(
		ctx,
		link,
		manifest.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		manifest.WithCacheDir(config.G[config.KraftKit](ctx).Paths.Sources),
	)
	if err != nil {
		return nil, nil, err
	}

	manifests, err = ghProvider.Manifests()
	if err != nil {
		return nil, nil, err
	}
	if len(manifests) == 0 {
		return nil, nil, fmt.Errorf("no manifest found in GitHub repo")
	}
	// Try to select the manifest matching the subdirectory (path) more robustly
	m = nil
	subdir := strings.Trim(path, "/")
	hasExplicitSubdir := strings.Contains(d.url, treeSeparator)
	log.G(ctx).Debugf("Selecting manifest for subdir '%s' (path: '%s'), branch='%s', hasExplicitSubdir=%v", subdir, path, branch, hasExplicitSubdir)
	for _, manifest := range manifests {
		manifestPath := ""
		// Try to extract the relative path from the manifest's Origin
		if strings.Contains(manifest.Origin, repo.RepoName()+"/") {
			manifestPath = strings.SplitN(manifest.Origin, repo.RepoName()+"/", 2)[1]
			manifestPath = strings.Trim(manifestPath, "/")
		}
		log.G(ctx).Debugf("Considering manifest: Name='%s', Origin='%s', extractedPath='%s'", manifest.Name, manifest.Origin, manifestPath)
		if manifestPath == subdir || manifest.Name == subdir {
			log.G(ctx).Infof("Selected manifest: Name='%s', Origin='%s' for subdir '%s'", manifest.Name, manifest.Origin, subdir)
			m = manifest
			break
		}
	}
	if m == nil {
		// fallback to first manifest - this is expected when a subdirectory is specified
		// since manifests describe the repo structure, not subdirectories
		m = manifests[0]
		log.G(ctx).Debugf("using first manifest: %s (Origin: %s)", m.Name, m.Origin)
	}

	// If a specific branch was extracted from the URL, replace manifest channels with a single channel for that branch
	if branch != "" {
		log.G(ctx).Debugf("Updating manifest channels to use branch '%s'", branch)
		m.Channels = []manifest.ManifestChannel{
			{
				Name:     branch,
				Default:  true,
				Resource: ghrepo.BranchArchive(repo, branch),
			},
		}
		// Clear versions if we have a specific branch, as channels take precedence
		m.Versions = []manifest.ManifestVersion{}
	}

	// Ensure at least one channel is marked as default
	if len(m.Channels) == 0 {
		// If no channels, create one using the default branch
		defaultBranch := "main"
		log.G(ctx).Debugf("Creating default channel for branch '%s'", defaultBranch)
		m.Channels = []manifest.ManifestChannel{
			{
				Name:     defaultBranch,
				Default:  true,
				Resource: ghrepo.BranchArchive(repo, defaultBranch),
			},
		}
		// Clear versions since we have a channel now
		m.Versions = []manifest.ManifestVersion{}
	} else if len(m.Channels) == 1 {
		// Ensure the single channel is marked as default
		m.Channels[0].Default = true
	}

	p, err := manifest.NewPackageFromManifest(
		m,
		manifest.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
		manifest.WithUpdate(true),
		manifest.WithCacheDir(config.G[config.KraftKit](ctx).Paths.Sources),
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

	// Try to set the Kraftfile path for correct detection in subdirectories
	found := false
	for _, candidate := range app.DefaultFileNames {
		candidatePath := filepath.Join(opts.Workdir, candidate)
		if fi, err := os.Stat(candidatePath); err == nil && !fi.IsDir() {
			opts.Kraftfile = candidatePath
			found = true
			break
		}
	}
	if !found {
		// fallback to default (may trigger error later, but is explicit)
		opts.Kraftfile = filepath.Join(opts.Workdir, "Kraftfile")
	}

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
