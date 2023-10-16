// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft GmbH.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ghrepo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/cli/cli/v2/git"
)

// Interface describes an object that represents a GitHub repository
type Interface interface {
	RepoName() string
	RepoOwner() string
	RepoHost() string
}

// New instantiates a GitHub repository from owner and name arguments
func New(owner, repo string) Interface {
	return NewWithHost(owner, repo, "github.com")
}

// NewFromURL parses a given GitHub url and returns the populated Interface
func NewFromURL(path string) (Interface, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %s", err)
	}

	hostname := normalizeHostname(u.Host)
	if hostname != "github.com" {
		return nil, fmt.Errorf("host is not github.com")
	}

	parts := strings.Split(u.Path, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf(`expected the "[HOST/]OWNER/REPO" format, got %q`, path)
	}

	owner := parts[1]
	repo := strings.TrimSuffix(parts[2], ".git")

	return NewWithHost(owner, repo, hostname), nil
}

// NewWithHost is like New with an explicit host name
func NewWithHost(owner, repo, hostname string) Interface {
	return &ghRepo{
		owner:    owner,
		name:     repo,
		hostname: normalizeHostname(hostname),
	}
}

// FullName serializes a GitHub repository into an "OWNER/REPO" string
func FullName(r Interface) string {
	return fmt.Sprintf("%s/%s", r.RepoOwner(), r.RepoName())
}

var defaultHostOverride string

func defaultHost() string {
	if defaultHostOverride != "" {
		return defaultHostOverride
	}

	return "github.com"
}

// SetDefaultHost overrides the default GitHub hostname for FromFullName.
// TODO: remove after FromFullName approach is revisited
func SetDefaultHost(host string) {
	defaultHostOverride = host
}

// FromFullName extracts the GitHub repository information from the following
// formats: "OWNER/REPO", "HOST/OWNER/REPO", and a full URL.
func FromFullName(nwo string) (Interface, error) {
	return FromFullNameWithHost(nwo, defaultHost())
}

// FromFullNameWithHost is like FromFullName that defaults to a specific host
// for values that don't explicitly include a hostname.
func FromFullNameWithHost(nwo, fallbackHost string) (Interface, error) {
	if git.IsURL(nwo) {
		u, err := git.ParseURL(nwo)
		if err != nil {
			return nil, err
		}
		return FromURL(u)
	}

	parts := strings.SplitN(nwo, "/", 4)
	for _, p := range parts {
		if len(p) == 0 {
			return nil, fmt.Errorf(`expected the "[HOST/]OWNER/REPO" format, got %q`, nwo)
		}
	}
	switch len(parts) {
	case 3:
		return NewWithHost(parts[1], parts[2], parts[0]), nil
	case 2:
		return NewWithHost(parts[0], parts[1], fallbackHost), nil
	default:
		return nil, fmt.Errorf(`expected the "[HOST/]OWNER/REPO" format, got %q`, nwo)
	}
}

// FromURL extracts the GitHub repository information from a git remote URL
func FromURL(u *url.URL) (Interface, error) {
	if u.Hostname() == "" {
		return nil, fmt.Errorf("no hostname detected")
	}

	parts := strings.SplitN(strings.Trim(u.Path, "/"), "/", 3)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid path: %s", u.Path)
	}

	return NewWithHost(parts[0], strings.TrimSuffix(parts[1], ".git"), u.Hostname()), nil
}

func normalizeHostname(h string) string {
	return strings.ToLower(strings.TrimPrefix(h, "www."))
}

// IsSame compares two GitHub repositories
func IsSame(a, b Interface) bool {
	return strings.EqualFold(a.RepoOwner(), b.RepoOwner()) &&
		strings.EqualFold(a.RepoName(), b.RepoName()) &&
		normalizeHostname(a.RepoHost()) == normalizeHostname(b.RepoHost())
}

func GenerateRepoURL(repo Interface, p string, args ...interface{}) string {
	baseURL := fmt.Sprintf(
		"https://%s/%s/%s",
		repo.RepoHost(),
		repo.RepoOwner(),
		repo.RepoName(),
	)
	if p != "" {
		if path := fmt.Sprintf(p, args...); path != "" {
			return baseURL + "/" + path
		}
	}
	return baseURL
}

// TODO there is a parallel implementation for non-isolated commands
func FormatRemoteURL(repo Interface, protocol string) string {
	if protocol == "ssh" {
		return fmt.Sprintf("git@%s:%s/%s.git", repo.RepoHost(), repo.RepoOwner(), repo.RepoName())
	}

	return fmt.Sprintf("https://%s%s/%s.git", repo.RepoHost(), repo.RepoOwner(), repo.RepoName())
}

// BranchArchive returns the archive URL of a branch given an interface and a
// branch
func BranchArchive(repo Interface, branch string) string {
	return fmt.Sprintf(
		"https://%s/%s/%s/archive/refs/heads/%s.tar.gz",
		repo.RepoHost(),
		repo.RepoOwner(),
		repo.RepoName(),
		branch,
	)
}

// TagArchive returns the archive URL of a tag given an Interface and a tag
func TagArchive(repo Interface, tag string) string {
	return fmt.Sprintf(
		"https://%s/%s/%s/archive/refs/tags/%s.tar.gz",
		repo.RepoHost(),
		repo.RepoOwner(),
		repo.RepoName(),
		tag,
	)
}

// SHAArchive returns the archive URL of a given Git SHA given an Interface and
// a SHA
func SHAArchive(repo Interface, sha string) string {
	return fmt.Sprintf(
		"https://%s/%s/%s/archive/%s.tar.gz",
		repo.RepoHost(),
		repo.RepoOwner(),
		repo.RepoName(),
		sha,
	)
}

type ReleaseAsset struct {
	Name   string
	APIURL string `json:"url"`
}

type Release struct {
	Tag    string `json:"tag_name"`
	Assets []ReleaseAsset
}

func HasScript(httpClient *http.Client, repo Interface) (hs bool, err error) {
	url := fmt.Sprintf(
		"https://api.%s/repos/%s/%s/contents/%s",
		repo.RepoHost(),
		repo.RepoOwner(),
		repo.RepoName(),
		repo.RepoName(),
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return
	}

	hs = true
	return
}

// DownloadAsset downloads a single asset to the given file path.
func DownloadAsset(httpClient *http.Client, asset ReleaseAsset, destPath string) error {
	req, err := http.NewRequest("GET", asset.APIURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/octet-stream")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

// FetchLatestRelease finds the latest published release for a repository.
func FetchLatestRelease(httpClient *http.Client, baseRepo Interface) (*Release, error) {
	url := fmt.Sprintf(
		"https://api.%s/repos/%s/%s/releases/latest",
		baseRepo.RepoHost(),
		baseRepo.RepoOwner(),
		baseRepo.RepoName(),
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var r Release
	err = json.Unmarshal(b, &r)
	if err != nil {
		return nil, err
	}

	return &r, nil
}

type ghRepo struct {
	owner    string
	name     string
	hostname string
}

func (r ghRepo) RepoOwner() string {
	return r.owner
}

func (r ghRepo) RepoName() string {
	return r.name
}

func (r ghRepo) RepoHost() string {
	return r.hostname
}
