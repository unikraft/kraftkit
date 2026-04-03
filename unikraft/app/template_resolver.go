// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH.
// Licensed under the BSD-3-Clause License (the "License").

package app

import (
	"context"
	"fmt"
	"os"

	kraftfilev07 "unikraft.com/x/kraftfile"

	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/template"
)

type TemplatePackageSelector func([]pack.Package) (pack.Package, error)

type TemplateResolutionOptions struct {
	allowPull    bool
	forcePull    bool
	workdir      string
	queryOptions []packmanager.QueryOption
	pullOptions  []pack.PullOption
	selector     TemplatePackageSelector
}

type TemplateResolutionOption func(*TemplateResolutionOptions) error

func WithTemplateResolutionAllowPull(allow bool) TemplateResolutionOption {
	return func(opts *TemplateResolutionOptions) error {
		opts.allowPull = allow
		return nil
	}
}

func WithTemplateResolutionForcePull(force bool) TemplateResolutionOption {
	return func(opts *TemplateResolutionOptions) error {
		opts.forcePull = force
		return nil
	}
}

func WithTemplateResolutionWorkdir(workdir string) TemplateResolutionOption {
	return func(opts *TemplateResolutionOptions) error {
		opts.workdir = workdir
		return nil
	}
}

func WithTemplateResolutionQueryOptions(queryOptions ...packmanager.QueryOption) TemplateResolutionOption {
	return func(opts *TemplateResolutionOptions) error {
		opts.queryOptions = append(opts.queryOptions, queryOptions...)
		return nil
	}
}

func WithTemplateResolutionPullOptions(pullOptions ...pack.PullOption) TemplateResolutionOption {
	return func(opts *TemplateResolutionOptions) error {
		opts.pullOptions = append(opts.pullOptions, pullOptions...)
		return nil
	}
}

func WithTemplateResolutionSelector(selector TemplatePackageSelector) TemplateResolutionOption {
	return func(opts *TemplateResolutionOptions) error {
		opts.selector = selector
		return nil
	}
}

// ResolveTemplate resolves the template declared in a v0.7 Kraftfile,
// pulling it if necessary, and merges the template document with the provided
// project document. Project fields always take precedence over template fields,
// following upstream Kraftfile.Merge semantics.
//
// The template's Kraftfile must declare spec v0.7 or later; an error is returned
// otherwise. Returns doc unchanged when no template is declared.
func ResolveTemplate(ctx context.Context, doc *kraftfilev07.Kraftfile, opts ...TemplateResolutionOption) (*kraftfilev07.Kraftfile, error) {
	if doc == nil || doc.Template == nil {
		return doc, nil
	}

	ropts := &TemplateResolutionOptions{
		allowPull: true,
	}
	for _, opt := range opts {
		if err := opt(ropts); err != nil {
			return nil, err
		}
	}

	// Convert kraftfilev07.Template → TemplateConfig. TransformFromSchema queries
	// the local pack store, so Path() is set when the template is already pulled.
	templateConfig, err := v07TemplateFromDocument(ctx, doc.Template)
	if err != nil {
		return nil, fmt.Errorf("could not initialize template config: %w", err)
	}
	if templateConfig == nil {
		return doc, nil
	}

	if shouldPullTemplate(templateConfig.Path(), ropts.forcePull) {
		if !ropts.allowPull {
			return nil, fmt.Errorf("template is not available locally: %s", unikraft.TypeNameVersion(templateConfig))
		}

		templatePack, err := resolveTemplatePackage(ctx, templateConfig, ropts)
		if err != nil {
			return nil, err
		}

		pullOptions := append([]pack.PullOption{}, ropts.pullOptions...)
		if ropts.workdir != "" {
			pullOptions = append([]pack.PullOption{pack.WithPullWorkdir(ropts.workdir)}, pullOptions...)
		}

		if err := templatePack.Pull(ctx, pullOptions...); err != nil {
			return nil, fmt.Errorf("could not pull template %s: %w", unikraft.TypeNameVersion(templateConfig), err)
		}

		// Re-initialize after pulling so TransformFromSchema queries the updated
		// local store and populates Path().
		templateConfig, err = v07TemplateFromDocument(ctx, doc.Template)
		if err != nil {
			return nil, fmt.Errorf("could not re-initialize template config after pull: %w", err)
		}
	}

	if templateConfig.Path() == "" {
		return nil, fmt.Errorf("template path is not known: %s", unikraft.TypeNameVersion(templateConfig))
	}

	templateDoc, err := kraftfilev07.ParseDirectory(templateConfig.Path())
	if err != nil {
		return nil, fmt.Errorf("could not parse template Kraftfile at %s: %w", templateConfig.Path(), err)
	}

	// Template is the base; project fields always override.
	templateDoc.Merge(doc)

	return templateDoc, nil
}

func shouldPullTemplate(path string, forcePull bool) bool {
	if forcePull {
		return true
	}

	if path == "" {
		return true
	}

	stat, err := os.Stat(path)
	return err != nil || !stat.IsDir()
}

func resolveTemplatePackage(ctx context.Context, template *template.TemplateConfig, opts *TemplateResolutionOptions) (pack.Package, error) {
	pm := packmanager.G(ctx)
	if pm == nil {
		return nil, fmt.Errorf("package manager is not initialized")
	}

	queryOptions := []packmanager.QueryOption{
		packmanager.WithName(template.Name()),
		packmanager.WithTypes(template.Type()),
	}

	if version := template.Version(); version != "" {
		queryOptions = append(queryOptions, packmanager.WithVersion(version))
	}
	if source := template.Source(); source != "" {
		queryOptions = append(queryOptions, packmanager.WithSource(source))
	}

	queryOptions = append(queryOptions, opts.queryOptions...)

	packages, err := pm.Catalog(ctx, queryOptions...)
	if err != nil {
		return nil, err
	}

	switch len(packages) {
	case 0:
		return nil, fmt.Errorf("could not find: %s", template.String())
	case 1:
		return packages[0], nil
	default:
		if opts.selector == nil {
			return nil, fmt.Errorf("too many options for %s", template.String())
		}

		selected, err := opts.selector(packages)
		if err != nil {
			return nil, err
		}
		if selected == nil {
			return nil, fmt.Errorf("no template package selected for %s", template.String())
		}

		return selected, nil
	}
}
