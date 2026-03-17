// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"testing"
)

func TestMatchAddition(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantMatch     bool
		wantComposite string
		wantFlag      string
		wantCondition string
		wantMatches   []string
	}{
		{
			name:      "no += means no match",
			line:      "LIBFOO_CFLAGS = -O2",
			wantMatch: false,
		},
		{
			name:      "empty line means no match",
			line:      "",
			wantMatch: false,
		},
		{
			name:          "composite and CFLAGS flag",
			line:          "LIBFOO_CFLAGS += -O2",
			wantMatch:     true,
			wantComposite: "LIBFOO",
			wantFlag:      CFLAGS,
			wantMatches:   []string{"-O2"},
		},
		{
			name:          "composite and SRCS flag",
			line:          "LIBBAR_SRCS += file.c",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      SRCS,
			wantMatches:   []string{"file.c"},
		},
		{
			name:          "composite and ASFLAGS flag",
			line:          "LIBBAR_ASFLAGS += -masm=intel",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      ASFLAGS,
			wantMatches:   []string{"-masm=intel"},
		},
		{
			name:          "composite and ASINCLUDES flag",
			line:          "LIBBAR_ASINCLUDES += -I/usr/include",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      ASINCLUDES,
			wantMatches:   []string{"-I/usr/include"},
		},
		{
			name:          "composite and CINCLUDES flag",
			line:          "LIBBAR_CINCLUDES += -Iinclude",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      CINCLUDES,
			wantMatches:   []string{"-Iinclude"},
		},
		{
			name:          "composite and CXXFLAGS flag",
			line:          "LIBBAR_CXXFLAGS += -std=c++17",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      CXXFLAGS,
			wantMatches:   []string{"-std=c++17"},
		},
		{
			name:          "composite and CXXINCLUDES flag",
			line:          "LIBBAR_CXXINCLUDES += -Icxxinclude",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      CXXINCLUDES,
			wantMatches:   []string{"-Icxxinclude"},
		},
		{
			name:          "composite and OBJS flag",
			line:          "LIBBAR_OBJS += obj.o",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      OBJS,
			wantMatches:   []string{"obj.o"},
		},
		{
			name:          "composite and OBJCFLAGS flag",
			line:          "LIBBAR_OBJCFLAGS += -DFOO",
			wantMatch:     true,
			wantComposite: "LIBBAR",
			wantFlag:      OBJCFLAGS,
			wantMatches:   []string{"-DFOO"},
		},
		{
			name:          "plain flag type without composite - single part",
			line:          "CFLAGS += -Wall",
			wantMatch:     true,
			wantComposite: "",
			wantFlag:      CFLAGS,
			wantMatches:   []string{"-Wall"},
		},
		{
			name:          "plain SRCS without composite",
			line:          "SRCS += main.c helper.c",
			wantMatch:     true,
			wantComposite: "",
			wantFlag:      SRCS,
			wantMatches:   []string{"main.c", "helper.c"},
		},
		{
			name:          "single part not in matchAppendTypes becomes composite",
			line:          "LIBFOO += val",
			wantMatch:     true,
			wantComposite: "LIBFOO",
			wantFlag:      "",
			wantMatches:   []string{"val"},
		},
		{
			name:          "last part not in matchAppendTypes whole left is composite",
			line:          "LIBFOO_UNKNOWN += val",
			wantMatch:     true,
			wantComposite: "LIBFOO_UNKNOWN",
			wantFlag:      "",
			wantMatches:   []string{"val"},
		},
		{
			name:          "condition parsed from left side with hyphen",
			line:          "LIBFOO_CFLAGS-$(CONFIG_ENABLE) += -DENABLE",
			wantMatch:     true,
			wantComposite: "LIBFOO",
			wantFlag:      CFLAGS,
			wantCondition: "$(CONFIG_ENABLE)",
			wantMatches:   []string{"-DENABLE"},
		},
		{
			name:          "right side all whitespace gives no matches",
			line:          "LIBFOO_SRCS +=   ",
			wantMatch:     true,
			wantComposite: "LIBFOO",
			wantFlag:      SRCS,
		},
		{
			name:          "multiple matches on right side",
			line:          "LIBFOO_SRCS += a.c b.c c.c",
			wantMatch:     true,
			wantComposite: "LIBFOO",
			wantFlag:      SRCS,
			wantMatches:   []string{"a.c", "b.c", "c.c"},
		},
		{
			name:          "backslash entries are filtered out",
			line:          "LIBFOO_SRCS += a.c \\ b.c",
			wantMatch:     true,
			wantComposite: "LIBFOO",
			wantFlag:      SRCS,
			wantMatches:   []string{"a.c", "b.c"},
		},
		{
			name:          "leading and trailing whitespace trimmed",
			line:          "  LIBFOO_CFLAGS += -O2  ",
			wantMatch:     true,
			wantComposite: "LIBFOO",
			wantFlag:      CFLAGS,
			wantMatches:   []string{"-O2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, composite, flag, condition, matches := MatchAddition(tt.line)

			if match != tt.wantMatch {
				t.Errorf("MatchAddition(%q) match = %v, want %v", tt.line, match, tt.wantMatch)
			}
			if !tt.wantMatch {
				return
			}
			if composite != tt.wantComposite {
				t.Errorf("MatchAddition(%q) composite = %q, want %q", tt.line, composite, tt.wantComposite)
			}
			if flag != tt.wantFlag {
				t.Errorf("MatchAddition(%q) flag = %q, want %q", tt.line, flag, tt.wantFlag)
			}
			if condition != tt.wantCondition {
				t.Errorf("MatchAddition(%q) condition = %q, want %q", tt.line, condition, tt.wantCondition)
			}
			if len(matches) != len(tt.wantMatches) {
				t.Errorf("MatchAddition(%q) matches = %v, want %v", tt.line, matches, tt.wantMatches)
				return
			}
			for i, m := range tt.wantMatches {
				if matches[i] != m {
					t.Errorf("MatchAddition(%q) matches[%d] = %q, want %q", tt.line, i, matches[i], m)
				}
			}
		})
	}
}

func TestMatchRegistrationLine(t *testing.T) {
	cond := func(s string) *string { return &s }
	plat := func(s string) *string { return &s }

	tests := []struct {
		name          string
		line          string
		wantMatch     bool
		wantPlat      *string
		wantLibname   string
		wantCondition *string
	}{
		{
			name:        "addlib match",
			line:        "$(eval $(call addlib,libfoo))",
			wantMatch:   true,
			wantLibname: "libfoo",
		},
		{
			name:        "addlib match with leading whitespace",
			line:        "  $(eval $(call addlib,libbar))",
			wantMatch:   true,
			wantLibname: "libbar",
		},
		{
			name:          "addlib_s match valid",
			line:          "$(eval $(call addlib_s,libfoo,$(CONFIG_LIBFOO)))",
			wantMatch:     true,
			wantLibname:   "libfoo",
			wantCondition: cond("CONFIG_LIBFOO"),
		},
		{
			name:      "addlib_s match invalid split count",
			line:      "$(eval $(call addlib_s,libfoo))",
			wantMatch: false,
		},
		{
			name:        "addplatlib match valid",
			line:        "$(eval $(call addplatlib,kvm,libplatfoo))",
			wantMatch:   true,
			wantPlat:    plat("kvm"),
			wantLibname: "libplatfoo",
		},
		{
			name:      "addplatlib match invalid split count",
			line:      "$(eval $(call addplatlib,kvm))",
			wantMatch: false,
		},
		{
			name:          "addplatlib_s match valid",
			line:          "$(eval $(call addplatlib_s,kvm,libplatbar,$(CONFIG_PLAT)))",
			wantMatch:     true,
			wantPlat:      plat("kvm"),
			wantLibname:   "libplatbar",
			wantCondition: cond("CONFIG_PLAT"),
		},
		{
			name:      "addplatlib_s match invalid split count",
			line:      "$(eval $(call addplatlib_s,kvm,libplatbar))",
			wantMatch: false,
		},
		{
			name:      "non-matching line",
			line:      "LIBFOO_SRCS += file.c",
			wantMatch: false,
		},
		{
			name:      "empty line no match",
			line:      "",
			wantMatch: false,
		},
		{
			name:      "comment line no match",
			line:      "# This is a comment",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, plat, libname, condition := MatchRegistrationLine(tt.line)

			if match != tt.wantMatch {
				t.Errorf("MatchRegistrationLine(%q) match = %v, want %v", tt.line, match, tt.wantMatch)
			}
			if !tt.wantMatch {
				return
			}
			if libname != tt.wantLibname {
				t.Errorf("MatchRegistrationLine(%q) libname = %q, want %q", tt.line, libname, tt.wantLibname)
			}
			if tt.wantPlat == nil && plat != nil {
				t.Errorf("MatchRegistrationLine(%q) plat = %q, want nil", tt.line, *plat)
			}
			if tt.wantPlat != nil {
				if plat == nil {
					t.Errorf("MatchRegistrationLine(%q) plat = nil, want %q", tt.line, *tt.wantPlat)
				} else if *plat != *tt.wantPlat {
					t.Errorf("MatchRegistrationLine(%q) plat = %q, want %q", tt.line, *plat, *tt.wantPlat)
				}
			}
			if tt.wantCondition == nil && condition != nil {
				t.Errorf("MatchRegistrationLine(%q) condition = %q, want nil", tt.line, *condition)
			}
			if tt.wantCondition != nil {
				if condition == nil {
					t.Errorf("MatchRegistrationLine(%q) condition = nil, want %q", tt.line, *tt.wantCondition)
				} else if *condition != *tt.wantCondition {
					t.Errorf("MatchRegistrationLine(%q) condition = %q, want %q", tt.line, *condition, *tt.wantCondition)
				}
			}
		})
	}
}
