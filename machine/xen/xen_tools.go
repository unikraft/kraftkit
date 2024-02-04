// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package xen

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"unicode"

	xs "github.com/joelnb/xenstore-go"
	"kraftkit.sh/exec"
)

const (
	XenToolsBin    = "xl"
	XenToolsCreate = "create"

	XenToolsPause   = "pause"
	XenToolsUnpause = "unpause"
	XenToolsId      = "domid"

	// `shutdown` does not work on unikraft unikerenels
	XenToolsDestroy = "destroy"
)

type XenCreateExecConfig struct {
	ConnectConsole bool `flag:"-c"`
	StartPaused    bool `flag:"-p"`
	KeepAlive      bool `flag:"-F"`
}

type XenState string

const (
	XenStateRunning  = XenState("running")
	XenStatePaused   = XenState("paused")
	XenStateBlocked  = XenState("blocked")
	XenStateShutdown = XenState("shutdown")
	XenStateCrashed  = XenState("crashed")
	XenStateDying    = XenState("dying")
)

var stateMappings = map[string]XenState{
	"r": XenStateRunning,
	"p": XenStatePaused,
	"b": XenStateBlocked,
	"s": XenStateShutdown,
	"c": XenStateCrashed,
	"d": XenStateDying,
}

var xenStateRegex = regexp.MustCompile(`\-+[rpbscd][-]*|[-]*[rpbscd]\-+`)

func (state XenState) String() string {
	return string(state)
}

func XenCreate(ctx context.Context, execConfig XenCreateExecConfig, configPath string) (*exec.Executable, error) {
	return exec.NewExecutable(XenToolsBin, execConfig, XenToolsCreate, configPath)
}

func XenDestroy(ctx context.Context, domuID int) error {
	e, err := exec.NewExecutable(XenToolsBin, XenToolsDestroy, fmt.Sprintf("%d", domuID))
	if err != nil {
		return err
	}

	_, err = xenToolsExecute(ctx, e)
	return err
}

func XenUnpause(ctx context.Context, domuID int) error {
	e, err := exec.NewExecutable(XenToolsBin, XenToolsUnpause, fmt.Sprintf("%d", domuID))
	if err != nil {
		return err
	}

	_, err = xenToolsExecute(ctx, e)
	return err
}

func XenPause(ctx context.Context, domuID int) error {
	e, err := exec.NewExecutable(XenToolsBin, XenToolsPause, fmt.Sprintf("%d", domuID))
	if err != nil {
		return err
	}

	_, err = xenToolsExecute(ctx, e)
	return err
}

func XenID(ctx context.Context, Name string) (int, error) {
	e, err := exec.NewExecutable(XenToolsBin, XenToolsId, Name)
	if err != nil {
		return 0, err
	}

	buffer, err := xenToolsExecute(ctx, e)
	if err != nil {
		return 0, err
	}

	id, err := strconv.Atoi(string(buffer))
	if err != nil {
		return 0, err
	}

	return id, nil
}

func XenGetState(ctx context.Context, domID int) (XenState, error) {
	e, err := exec.NewExecutable(XenToolsBin, "list", fmt.Sprintf("%d", domID))
	if err != nil {
		return "", err
	}

	buffer, err := xenToolsExecute(ctx, e)
	if err != nil {
		return "", err
	}

	matches := xenStateRegex.FindAll(buffer, -1)
	if matches == nil || matches > 1 {
		return "", fmt.Errorf("could not parse xen state")
	}

	stateIdx := bytes.IndexFunc(matches[0], unicode.IsLetter)
	if stateIdx == -1 {
		return "", fmt.Errorf("could not parse xen state")
	}

	xenState, ok := stateMappings[string(matches[0][stateIdx])]
	if !ok {
		return "", fmt.Errorf("could not parse xen state")
	}

	return xenState, nil
}

func XenCreateClient() (*xs.Client, error) {
	return xs.NewXenBusClient(xs.XenBusPath())
}

func xenToolsExecute(ctx context.Context, e *exec.Executable) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)

	p, err := exec.NewProcessFromExecutable(
		e,
		exec.WithStdin(nil),
		exec.WithStderr(nil),
		exec.WithStdout(buffer),
	)

	if err != nil {
		return nil, err
	}

	if err := p.StartAndWait(ctx); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
