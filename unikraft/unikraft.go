// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package unikraft

const (
	// Environmental variables recognized by Unikraft's build system.
	UK_NAME          = "CONFIG_UK_NAME"
	UK_DEFNAME       = "CONFIG_UK_DEFNAME"
	UK_CONFIG        = "CONFIG_UK_CONFIG"
	UK_FULLVERSION   = "CONFIG_UK_FULLVERSION"
	UK_CODENAME      = "CONFIG_UK_CODENAME"
	UK_ARCH          = "CONFIG_UK_ARCH"
	UK_BASE          = "CONFIG_UK_BASE"
	UK_APP           = "CONFIG_UK_APP"
	UK_DEFCONFIG     = "UK_DEFCONFIG"
	KCONFIG_APP_DIR  = "KCONFIG_APP_DIR"
	KCONFIG_LIB_DIR  = "KCONFIG_LIB_DIR"
	KCONFIG_LIB_IN   = "KCONFIG_LIB_IN"
	KCONFIG_PLAT_DIR = "KCONFIG_PLAT_DIR"
	KCONFIG_PLAT_IN  = "KCONFIG_PLAT_IN"

	// Filenames which represent ecosystem files
	Config_uk     = "Config.uk"
	Exportsyms_uk = "exportsyms.uk"
	Linker_uk     = "Linker.uk"
	Localsyms_uk  = "localsyms.uk"
	Makefile_uk   = "Makefile.uk"

	// Standard Makefile.uk variables
	UK_PROVIDED_SYSCALLS = "UK_PROVIDED_SYSCALLS"

	// Built-in paths
	VendorDir = ".unikraft"
	BuildDir  = ".unikraft/build"
	LibsDir   = ".unikraft/libs"
)
