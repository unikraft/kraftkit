// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package config

// AuthConfig represents a very abstract representation of authentication used
// by some service.  Most APIs and services which can be authenticated have the
// defined four parameters found within AuthConfig.
type AuthConfig struct {
	User      string `yaml:"user" env:"KRAFTKIT_AUTH_%s_USER" long:"auth-%s-user"`
	Token     string `yaml:"token" env:"KRAFTKIT_AUTH_%s_TOKEN" long:"auth-%s-token"`
	Endpoint  string `yaml:"endpoint" env:"KRAFTKIT_AUTH_%s_ENDPOINT" long:"auth-%s-endpoint"`
	VerifySSL bool   `yaml:"verify_ssl" env:"KRAFTKIT_AUTH_%s_VERIFY_SSL" long:"auth-%s-verify-ssl"`
}

type KraftKit struct {
	NoPrompt       bool   `yaml:"no_prompt" env:"KRAFTKIT_NO_PROMPT" long:"no-prompt" usage:"Do not prompt for user interaction" default:"false"`
	NoParallel     bool   `yaml:"no_parallel" env:"KRAFTKIT_NO_PARALLEL" long:"no-parallel" usage:"Do not run internal tasks in parallel" default:"false"`
	NoEmojis       bool   `yaml:"no_emojis" env:"KRAFTKIT_NO_EMOJIS" long:"no-emojis" usage:"Do not use emojis in any console output" default:"true"`
	NoCheckUpdates bool   `yaml:"no_check_updates" env:"KRAFTKIT_NO_CHECK_UPDATES" long:"no-check-updates" usage:"Do not check for updates" default:"false"`
	Editor         string `yaml:"editor" env:"KRAFTKIT_EDITOR" long:"editor" usage:"Set the text editor to open when prompt to edit a file"`
	GitProtocol    string `yaml:"git_protocol" env:"KRAFTKIT_GIT_PROTOCOL" long:"git-protocol" usage:"Preferred Git protocol to use" default:"https"`
	Pager          string `yaml:"pager,omitempty" env:"KRAFTKIT_PAGER" long:"pager" usage:"System pager to pipe output to"`
	HTTPUnixSocket string `yaml:"http_unix_socket,omitempty" env:"KRAFTKIT_HTTP_UNIX_SOCKET" long:"http-unix-sock" usage:"When making HTTP(S) connections, pipe requests via this shared socket"`
	RuntimeDir     string `yaml:"runtime_dir" env:"KRAFTKIT_RUNTIME_DIR" long:"runtime-dir" usage:"Directory for placing runtime files (e.g. pidfiles)" default:"/var/kraftkit"`
	DefaultPlat    string `yaml:"default_plat" env:"KRAFTKIT_DEFAULT_PLAT" usage:"The default platform to use when invoking platform-specific code" noattribute:"true"`
	DefaultArch    string `yaml:"default_arch" env:"KRAFTKIT_DEFAULT_ARCH" usage:"The default architecture to use when invoking architecture-specific code" noattribute:"true"`
	ContainerdAddr string `yaml:"containerd_addr,omitempty" env:"KRAFTKIT_CONTAINERD_ADDR" long:"containerd-addr" usage:"Address of containerd daemon socket" default:""`
	EventsPidFile  string `yaml:"events_pidfile" env:"KRAFTKIT_EVENTS_PIDFILE" long:"events-pid-file" usage:"Events process ID used when running multiple unikernels" default:"/var/kraftkit/events.pid"`
	UserGroup      string `yaml:"user_group" env:"KRAFTKIT_USER_GROUP" long:"user-group" usage:"Group to use for common files" default:"kraftkit"`

	Paths struct {
		Plugins   string `yaml:"plugins,omitempty" env:"KRAFTKIT_PATHS_PLUGINS" long:"plugins-dir" usage:"Path to KraftKit plugin directory"`
		Config    string `yaml:"config,omitempty" env:"KRAFTKIT_PATHS_CONFIG" long:"config-dir" usage:"Path to KraftKit config directory"`
		Manifests string `yaml:"manifests,omitempty" env:"KRAFTKIT_PATHS_MANIFESTS" long:"manifests-dir" usage:"Path to Unikraft manifest cache"`
		Sources   string `yaml:"sources,omitempty" env:"KRAFTKIT_PATHS_SOURCES" long:"sources-dir" usage:"Path to Unikraft component cache"`
	} `yaml:"paths,omitempty"`

	Log struct {
		Level      string `yaml:"level" env:"KRAFTKIT_LOG_LEVEL" long:"log-level" usage:"Log level verbosity" default:"info"`
		Timestamps bool   `yaml:"timestamps" env:"KRAFTKIT_LOG_TIMESTAMPS" long:"log-timestamps" usage:"Enable log timestamps"`
		Type       string `yaml:"type" env:"KRAFTKIT_LOG_TYPE" long:"log-type" usage:"Log type" default:"fancy"`
	} `yaml:"log"`

	Unikraft struct {
		Mirrors   []string `yaml:"mirrors" env:"KRAFTKIT_UNIKRAFT_MIRRORS" long:"with-mirror" usage:"Paths to mirrors of Unikraft component artifacts"`
		Manifests []string `yaml:"manifests" env:"KRAFTKIT_UNIKRAFT_MANIFESTS" long:"with-manifest" usage:"Paths to package or component manifests"`
	} `yaml:"unikraft"`

	Auth map[string]AuthConfig `yaml:"auth,omitempty" noattribute:"true"`

	Aliases map[string]map[string]string `yaml:"aliases" noattribute:"true"`
}

type ConfigDetail struct {
	Key           string
	Description   string
	AllowedValues []string
}

// Descriptions of each configuration parameter as well as valid values
var configDetails = []ConfigDetail{
	{
		Key:         "no_prompt",
		Description: "toggle interactive prompting in the terminal",
	},
	{
		Key:         "editor",
		Description: "the text editor program to use for authoring text",
	},
	{
		Key:         "browser",
		Description: "the web browser to use for opening URLs",
	},
	{
		Key:         "git_protocol",
		Description: "the protocol to use for git clone and push operations",
		AllowedValues: []string{
			"https",
			"ssh",
		},
	},
	{
		Key:         "pager",
		Description: "the terminal pager program to send standard output to",
	},
	{
		Key:         "log.level",
		Description: "Set the logging verbosity",
		AllowedValues: []string{
			"fatal",
			"error",
			"warn",
			"info",
			"debug",
			"trace",
		},
	},
	{
		Key:         "log.type",
		Description: "Set the logging verbosity",
		AllowedValues: []string{
			"quiet",
			"basic",
			"fancy",
			"json",
		},
	},
	{
		Key:         "log.timestamps",
		Description: "Show timestamps with log output",
	},
}

func ConfigDetails() []ConfigDetail {
	return configDetails
}
