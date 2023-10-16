// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	gitplumbing "github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

type cloneProgress struct {
	onProgress     func(progress float64)
	once           *sync.Once
	completeParent chan struct{}
	completeWorker chan struct{}
}

func (p cloneProgress) Write(b []byte) (int, error) {
	if p.onProgress == nil {
		return len(b), nil
	}

	var prog string
	var scale float64

	// Trim prefix
	switch {
	case strings.HasPrefix(string(b), "Counting objects"):
		prog = strings.TrimPrefix(string(b), "Counting objects: ")
		scale = 0.1
	case strings.HasPrefix(string(b), "Compressing objects"):
		prog = strings.TrimPrefix(string(b), "Compressing objects: ")
		scale = 0.2
	case strings.HasPrefix(string(b), "Total "):
		p.once.Do(func() {
			go func() {
				for i := 0; i < 99; i++ {
					exponential := math.Pow(1.02, float64(2*i)) * 1000
					timeToWait := time.Duration(int(exponential) + rand.Intn(250))
					time.Sleep(timeToWait * time.Millisecond)

					p.onProgress(0.2 + 0.8*(float64(i)/100.0))

					select {
					case <-p.completeWorker:
						p.completeParent <- struct{}{}
						return
					default:
						continue
					}
				}
				<-p.completeWorker
				p.completeParent <- struct{}{}
			}()
		})

		return len(b), nil
	default:
		return len(b), nil
	}

	// Trim leading spaces
	progressString := strings.TrimLeft(prog, " ")

	// Trim everything after '%'
	percent := strings.IndexByte(prog, '%')
	if percent < 0 {
		return len(b), nil
	}
	progressString = progressString[:percent]

	// Convert to float
	var progress float64
	if _, err := fmt.Sscanf(progressString, "%f", &progress); err != nil {
		return len(b), err
	}

	// Call callback
	p.onProgress(scale * (progress / 100.0))

	return len(b), nil
}

// pullGit is used internally to pull a specific Manifest resource using if the
// Manifest has the repo defined within.
func pullGit(ctx context.Context, manifest *Manifest, opts ...pack.PullOption) error {
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	if len(popts.Workdir()) == 0 {
		return fmt.Errorf("cannot Git clone manifest package without working directory")
	}

	log.G(ctx).Debugf("using git to pull manifest package %s", manifest.Name)

	if len(manifest.Origin) == 0 {
		return fmt.Errorf("requesting Git with empty repository in manifest")
	}

	completeWorker := make(chan struct{})
	completeParent := make(chan struct{})

	copts := &git.CloneOptions{
		SingleBranch:      true,
		Tags:              git.NoTags,
		NoCheckout:        false,
		Depth:             1,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
		Progress: cloneProgress{
			onProgress:     popts.OnProgress,
			completeWorker: completeWorker,
			completeParent: completeParent,
			once:           &sync.Once{},
		},
	}
	path := manifest.Origin

	// Is this an SSH URL?
	if isSSHURL(path) {
		if strings.HasPrefix(path, "git@") {
			path = "ssh://" + path
		}
		copts.Auth, err = gitssh.NewSSHAgentAuth("git")
		if err != nil {
			return fmt.Errorf("could not create SSH agent auth: %w", err)
		}
	} else {
		if !strings.HasPrefix(path, "https://") {
			path = "https://" + path
		}
		u, err := url.Parse(path)
		if err != nil {
			return fmt.Errorf("could not parse URL: %w", err)
		}
		endpoint := u.Host

		if auth := popts.Auths(endpoint); auth != nil {
			if auth.User != "" && auth.Token != "" {
				copts.Auth = &githttp.BasicAuth{
					Username: auth.User,
					Password: auth.Token,
				}
			} else if auth.Token != "" {
				copts.Auth = &githttp.TokenAuth{
					Token: auth.Token,
				}
			}
		}
	}
	copts.URL = path

	version := ""

	if len(manifest.Channels) == 1 {
		version = manifest.Channels[0].Name
	} else if len(manifest.Versions) == 1 {
		version = manifest.Versions[0].Version
	}

	if version != "" {
		copts.ReferenceName = gitplumbing.NewBranchReferenceName(version)
	}

	local, err := unikraft.PlaceComponent(
		popts.Workdir(),
		manifest.Type,
		manifest.Name,
	)
	if err != nil {
		return fmt.Errorf("could not place component package: %w", err)
	}

	log.G(ctx).
		WithField("from", path).
		WithField("to", local).
		WithField("branch", version).
		Infof("git clone")

	_, err = git.PlainCloneContext(ctx, local, false, copts)
	switch {
	case errors.Is(err, git.ErrRepositoryAlreadyExists):
		reps, err := git.PlainOpen(local)
		if err != nil {
			return fmt.Errorf("could not open repository: %w", err)
		}
		err = reps.FetchContext(ctx, &git.FetchOptions{
			RemoteURL: copts.URL,
			Tags:      copts.Tags,
			Depth:     copts.Depth,
			Auth:      copts.Auth,
			Progress:  nil,
		})
		switch {
		case errors.Is(err, git.NoErrAlreadyUpToDate), errors.Is(err, git.ErrBranchExists), err == nil:
			log.G(ctx).Infof("successfully updated %s in %s", path, local)
			return nil
		default:
			return fmt.Errorf("could not clone repository: %w", err)
		}
	case err != nil:
		return fmt.Errorf("could not clone repository: %w", err)
	}

	// Wait for the go routine to finish
	completeWorker <- struct{}{}
	<-completeParent
	popts.OnProgress(1.0)

	log.G(ctx).Infof("successfully cloned %s into %s", path, local)

	return nil
}
