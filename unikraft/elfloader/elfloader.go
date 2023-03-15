// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package elfloader

import (
	"context"
	"debug/elf"
	//"fmt"
	//"reflect"

	//"fmt"
	//"fmt"
	//"io"
	"os"
	//"path/filepath"
	//"sort"
	//"strings"

	//"github.com/sirupsen/logrus"
	//"github.com/xlab/treeprint"

	"kraftkit.sh/kconfig"
	//"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"

	//"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/unikraft/template"
)

const ELFLoaderAppName = "elfloader"

type ELFLoader struct {
	aopts       []app.ApplicationOption
	application app.Application
	binFile     string
}

func (elfloader ELFLoader) GetBinary() string {
	return elfloader.binFile
}

func (elfloader ELFLoader) Name() string {
	return elfloader.application.Name()
}

func (elfloader ELFLoader) Source() string {
	return elfloader.application.Source()
}

func (elfloader ELFLoader) Version() string {
	return elfloader.application.Version()
}

func (elfloader ELFLoader) WorkingDir() string {
	return elfloader.application.WorkingDir()
}

func (elfloader ELFLoader) OutDir() string {
	return elfloader.application.OutDir()
}

func (elfloader ELFLoader) Template() template.Template {
	return elfloader.application.Template()
}

func (elfloader ELFLoader) Unikraft() core.Unikraft {
	return elfloader.application.Unikraft()
}

func (elfloader ELFLoader) Libraries(ctx context.Context) (lib.Libraries, error) {
	return elfloader.application.Libraries(ctx)
}

func (elfloader ELFLoader) Targets() target.Targets {
	return elfloader.application.Targets()
}

func (elfloader ELFLoader) Extensions() component.Extensions {
	return elfloader.application.Extensions()
}

func (elfloader ELFLoader) Kraftfiles() []string {
	return elfloader.application.Kraftfiles()
}

func (elfloader ELFLoader) MergeTemplate(ctx context.Context, merge app.Application) (app.Application, error) {
	return elfloader.application.MergeTemplate(ctx, merge)
}

func (elfloader ELFLoader) KConfigTree(env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return elfloader.application.KConfigTree(env...)
}

func (elfloader ELFLoader) KConfig() kconfig.KeyValueMap {
	return elfloader.application.KConfig()
}

func (elfloader ELFLoader) KConfigFile(tc target.Target) string {
	return elfloader.application.KConfigFile(tc)
}

func (elfloader ELFLoader) IsConfigured(tc target.Target) bool {
	return elfloader.application.IsConfigured(tc)
}

func (elfloader ELFLoader) MakeArgs(tc target.Target) (*core.MakeArgs, error) {
	return elfloader.application.MakeArgs(tc)
}

func (elfloader ELFLoader) Make(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Make(ctx, tc, mopts...)
}

func (elfloader ELFLoader) SyncConfig(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.SyncConfig(ctx, tc, mopts...)
}

func (elfloader ELFLoader) DefConfig(ctx context.Context, tc target.Target, extra kconfig.KeyValueMap, mopts ...make.MakeOption) error {
	return elfloader.application.DefConfig(ctx, tc, extra, mopts...)
}

func (elfloader ELFLoader) Configure(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Configure(ctx, tc, mopts...)
}

func (elfloader ELFLoader) Prepare(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Prepare(ctx, tc, mopts...)
}

func (elfloader ELFLoader) Clean(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Configure(ctx, tc, mopts...)
}

func (elfloader ELFLoader) Properclean(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Properclean(ctx, tc, mopts...)
}

func (elfloader ELFLoader) Fetch(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Fetch(ctx, tc, mopts...)
}

func (elfloader ELFLoader) Set(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Set(ctx, tc, mopts...)
}

func (elfloader ELFLoader) Unset(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return elfloader.application.Unset(ctx, tc, mopts...)
}

func (elfloader ELFLoader) Build(ctx context.Context, tc target.Target, opts ...app.BuildOption) error {
	return elfloader.application.Build(ctx, tc, opts...)
}

func (elfloader ELFLoader) LibraryNames() []string {
	return elfloader.application.LibraryNames()
}

func (elfloader ELFLoader) TargetNames() []string {
	return elfloader.application.TargetNames()
}

func (elfloader ELFLoader) TargetByName(name string) (target.Target, error) {
	return elfloader.application.TargetByName(name)
}

func (elfloader ELFLoader) Components() ([]component.Component, error) {
	return elfloader.application.Components()
}

func (elfloader ELFLoader) Type() unikraft.ComponentType {
	return elfloader.application.Type()
}

func (elfloader ELFLoader) Path() string {
	return elfloader.application.Path()
}

func (elfloader ELFLoader) PrintInfo(ctx context.Context) string {
	return elfloader.application.PrintInfo(ctx)
}

func (elfloader ELFLoader) WithTarget(targ target.Target) (app.Application, error) {
	return elfloader.application.WithTarget(targ)
}

func New(ctx context.Context, bin string, eopts ...ELFLoaderOption) (app.Application, error) {
	fi, err := os.Open(bin)
	if err != nil {
		return nil, err
	}

	ef, err := elf.NewFile(fi)
	if err != nil {
		return nil, err
	}

	if ef.Machine.String() == "EM_386" {
		// TODO properly check if `ef` is a unikraft application.
		//
		// For now assume that you will only run 64 bit applications
		// using the elfloader. A Unikraft unikernel will show up as a
		// 32 bit elf, so just check that for now.
		return nil, nil
	}

	elfloader := ELFLoader{}

	for _, o := range eopts {
		if err := o(&elfloader); err != nil {
			return nil, err
		}
	}

	app, err := app.NewApplicationFromOptions(elfloader.aopts...)
	if err != nil {
		return nil, err
	}
	elfloader.application = app

	elfloader.binFile = bin

	return elfloader, err
}
