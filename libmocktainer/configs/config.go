// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package configs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// Config defines configuration options for executing a process inside a contained environment.
type Config struct {
	// ParentDeathSignal specifies the signal that is sent to the container's process in the case
	// that the parent process dies.
	ParentDeathSignal int `json:"parent_death_signal"`

	// Path to a directory containing the container's root filesystem.
	Rootfs string `json:"rootfs"`

	// Namespaces specifies the container's namespaces that it should setup when cloning the init process
	// If a namespace is not provided that namespace is shared from the container's parent process
	Namespaces Namespaces `json:"namespaces"`

	// Networks specifies the container's network setup to be created
	Networks []*Network `json:"networks"`

	// Routes can be specified to create entries in the route table as the container is started
	Routes []*Route `json:"routes"`

	// AppArmorProfile specifies the profile to apply to the process running in the container and is
	// change at the time the process is execed
	AppArmorProfile string `json:"apparmor_profile,omitempty"`

	// ProcessLabel specifies the label to apply to the process running in the container.  It is
	// commonly used by selinux
	ProcessLabel string `json:"process_label,omitempty"`

	// Hooks are a collection of actions to perform at various container lifecycle events.
	// CommandHooks are serialized to JSON, but other hooks are not.
	Hooks Hooks

	// Version is the version of opencontainer specification that is supported.
	Version string `json:"version"`

	// Labels are user defined metadata that is stored in the config and populated on the state
	Labels []string `json:"labels"`
}

type (
	HookName string
	HookList []Hook
	Hooks    map[HookName]HookList
)

const (
	// Prestart commands are executed after the container namespaces are created,
	// but before the user supplied command is executed from init.
	// Note: This hook is now deprecated
	// Prestart commands are called in the Runtime namespace.
	Prestart HookName = "prestart"

	// CreateRuntime commands MUST be called as part of the create operation after
	// the runtime environment has been created but before the pivot_root has been executed.
	// CreateRuntime is called immediately after the deprecated Prestart hook.
	// CreateRuntime commands are called in the Runtime Namespace.
	CreateRuntime HookName = "createRuntime"

	// CreateContainer commands MUST be called as part of the create operation after
	// the runtime environment has been created but before the pivot_root has been executed.
	// CreateContainer commands are called in the Container namespace.
	CreateContainer HookName = "createContainer"

	// StartContainer commands MUST be called as part of the start operation and before
	// the container process is started.
	// StartContainer commands are called in the Container namespace.
	StartContainer HookName = "startContainer"

	// Poststart commands are executed after the container init process starts.
	// Poststart commands are called in the Runtime Namespace.
	Poststart HookName = "poststart"

	// Poststop commands are executed after the container init process exits.
	// Poststop commands are called in the Runtime Namespace.
	Poststop HookName = "poststop"
)

func (hooks HookList) RunHooks(state *specs.State) error {
	for i, h := range hooks {
		if err := h.Run(state); err != nil {
			return fmt.Errorf("error running hook #%d: %w", i, err)
		}
	}

	return nil
}

func (hooks *Hooks) UnmarshalJSON(b []byte) error {
	var state map[HookName][]CommandHook

	if err := json.Unmarshal(b, &state); err != nil {
		return err
	}

	*hooks = Hooks{}
	for n, commandHooks := range state {
		if len(commandHooks) == 0 {
			continue
		}

		(*hooks)[n] = HookList{}
		for _, h := range commandHooks {
			(*hooks)[n] = append((*hooks)[n], h)
		}
	}

	return nil
}

func (hooks *Hooks) MarshalJSON() ([]byte, error) {
	serialize := func(hooks []Hook) (serializableHooks []CommandHook) {
		for _, hook := range hooks {
			switch chook := hook.(type) {
			case CommandHook:
				serializableHooks = append(serializableHooks, chook)
			default:
				logrus.Warnf("cannot serialize hook of type %T, skipping", hook)
			}
		}

		return serializableHooks
	}

	return json.Marshal(map[string]interface{}{
		"prestart":        serialize((*hooks)[Prestart]),
		"createRuntime":   serialize((*hooks)[CreateRuntime]),
		"createContainer": serialize((*hooks)[CreateContainer]),
		"startContainer":  serialize((*hooks)[StartContainer]),
		"poststart":       serialize((*hooks)[Poststart]),
		"poststop":        serialize((*hooks)[Poststop]),
	})
}

type Hook interface {
	// Run executes the hook with the provided state.
	Run(*specs.State) error
}

type Command struct {
	Path    string         `json:"path"`
	Args    []string       `json:"args"`
	Env     []string       `json:"env"`
	Dir     string         `json:"dir"`
	Timeout *time.Duration `json:"timeout"`
}

// NewCommandHook will execute the provided command when the hook is run.
func NewCommandHook(cmd Command) CommandHook {
	return CommandHook{
		Command: cmd,
	}
}

type CommandHook struct {
	Command
}

func (c Command) Run(s *specs.State) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Cmd{
		Path:   c.Path,
		Args:   c.Args,
		Env:    c.Env,
		Stdin:  bytes.NewReader(b),
		Stdout: &stdout,
		Stderr: &stderr,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	errC := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		if err != nil {
			err = fmt.Errorf("error running hook: %w, stdout: %s, stderr: %s", err, stdout.String(), stderr.String())
		}
		errC <- err
	}()
	var timerCh <-chan time.Time
	if c.Timeout != nil {
		timer := time.NewTimer(*c.Timeout)
		defer timer.Stop()
		timerCh = timer.C
	}
	select {
	case err := <-errC:
		return err
	case <-timerCh:
		_ = cmd.Process.Kill()
		<-errC
		return fmt.Errorf("hook ran past specified timeout of %.1fs", c.Timeout.Seconds())
	}
}
