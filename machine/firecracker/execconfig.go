// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package firecracker

// ExecConfig represents the command-line arguments for the Firecracker binary.
type ExecConfig struct {
	// Path to unix domain socket used by the API.
	// [default: "/run/firecracker.socket"]
	ApiSock string `flag:"--api-sock"`

	// Whether or not to load boot timer device for logging elapsed time since
	// InstanceStart command.
	BootTimer bool `flag:"--boot-timer"`

	// Path to a file that contains the microVM configuration in JSON format.
	ConfigFile string `flag:"--config-file"`

	// Print the data format version of the provided snapshot state file.
	DescribeSnapshot string `flag:"--describe-snapshot"`

	// Http API request payload max size, in bytes.
	// [default: "51200"]
	HttpApiMaxPayloadSize string `flag:"--http-api-max-payload-size"`

	// MicroVM unique identifier.
	// [default: "anonymous-instance"]
	Id string `flag:"--id"`

	// Set the logger level.
	// [default: "Warning"]
	Level string `flag:"--level"`

	// Path to a fifo or a file used for configuring the logger on startup.
	LogPath string `flag:"--log-path"`

	// Path to a file that contains metadata in JSON format to add to the mmds.
	Metadata string `flag:"--metadata"`

	// Mmds data store limit, in bytes.
	MmdsSizeLimit uint64 `flag:"--mmds-size-limit"`

	// Optional parameter which allows starting and using a microVM without an
	// active API socket.
	NoApi bool `flag:"--no-api"`

	// Optional parameter which allows starting and using a microVM without
	// seccomp filtering.  Not recommended.
	NoSeccomp bool `flag:"--no-seccomp"`

	// Parent process CPU time (wall clock, microseconds).
	// This parameter is optional.
	ParentCpuTimeUs uint64 `flag:"--parent-cpu-time-us"`

	// Optional parameter which allows specifying the path to a custom seccomp
	// filter.  For advanced users.
	SeccompFilter string `flag:"--seccomp-filter"`

	// Whether or not to output the level in the logs.
	ShowLevel bool `flag:"--show-level"`

	// Whether or not to include the file path and line number of the log's
	// origin.
	ShowLogOrigin bool `flag:"--show-log-origin"`
}
