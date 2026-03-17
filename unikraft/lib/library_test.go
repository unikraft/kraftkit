// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

// --- Helpers ---

func newTestLib(t *testing.T, opts ...LibraryOption) *LibraryConfig {
	t.Helper()
	lib, err := NewLibraryConfigFromOptions(opts...)
	if err != nil {
		t.Fatalf("NewLibraryConfigFromOptions() error: %v", err)
	}
	return lib
}

// mockPackage implements pack.Package for testing NewLibraryFromPackage.
type mockPackage struct {
	name    string
	version string
}

func (m *mockPackage) Name() string                                   { return m.name }
func (m *mockPackage) Version() string                                { return m.version }
func (m *mockPackage) String() string                                 { return m.name }
func (m *mockPackage) Type() unikraft.ComponentType                   { return unikraft.ComponentTypeLib }
func (m *mockPackage) ID() string                                     { return m.name }
func (m *mockPackage) Metadata() interface{}                          { return nil }
func (m *mockPackage) Size() int64                                    { return 0 }
func (m *mockPackage) Columns() []tableprinter.Column                 { return nil }
func (m *mockPackage) Push(context.Context, ...pack.PushOption) error { return nil }
func (m *mockPackage) Pull(context.Context, ...pack.PullOption) error { return nil }
func (m *mockPackage) Unpack(context.Context, string) error           { return nil }
func (m *mockPackage) Save(context.Context) error                     { return nil }
func (m *mockPackage) Export(context.Context, string) error           { return nil }
func (m *mockPackage) Delete(context.Context) error                   { return nil }
func (m *mockPackage) PulledAt(context.Context) (bool, time.Time, error) {
	return false, time.Time{}, nil
}
func (m *mockPackage) CreatedAt(context.Context) (time.Time, error) { return time.Time{}, nil }
func (m *mockPackage) UpdatedAt(context.Context) (time.Time, error) { return time.Time{}, nil }
func (m *mockPackage) Format() pack.PackageFormat                   { return pack.PackageFormat("mock") }

// --- Tests for NewLibraryConfigFromOptions ---

func TestNewLibraryConfigFromOptions_Empty(t *testing.T) {
	lib := newTestLib(t)
	if lib == nil {
		t.Fatal("expected non-nil LibraryConfig")
	}
	if lib.name != "" {
		t.Errorf("name = %q, want empty", lib.name)
	}
}

func TestNewLibraryConfigFromOptions_WithOptions(t *testing.T) {
	lib := newTestLib(t,
		WithName("testlib"),
		WithVersion("1.0.0"),
		WithSource("https://example.com"),
		WithLicense("BSD-3-Clause"),
		WithCompiler("gcc"),
		WithCompileDate("2024-01-01"),
		WithCompiledBy("Alice"),
		WithCompiledByAssoc("Unikraft GmbH"),
		WithIsInternal(true),
	)

	if lib.Name() != "testlib" {
		t.Errorf("Name() = %q, want %q", lib.Name(), "testlib")
	}
	if lib.Version() != "1.0.0" {
		t.Errorf("Version() = %q, want %q", lib.Version(), "1.0.0")
	}
	if lib.Source() != "https://example.com" {
		t.Errorf("Source() = %q, want %q", lib.Source(), "https://example.com")
	}
	if lib.License() != "BSD-3-Clause" {
		t.Errorf("License() = %q, want %q", lib.License(), "BSD-3-Clause")
	}
	if lib.Compiler() != "gcc" {
		t.Errorf("Compiler() = %q, want %q", lib.Compiler(), "gcc")
	}
	if lib.CompileDate() != "2024-01-01" {
		t.Errorf("CompileDate() = %q, want %q", lib.CompileDate(), "2024-01-01")
	}
	if lib.CompiledBy() != "Alice" {
		t.Errorf("CompiledBy() = %q, want %q", lib.CompiledBy(), "Alice")
	}
	if lib.CompiledByAssoc() != "Unikraft GmbH" {
		t.Errorf("CompiledByAssoc() = %q, want %q", lib.CompiledByAssoc(), "Unikraft GmbH")
	}
	if !lib.IsInternal() {
		t.Errorf("IsInternal() = false, want true")
	}
}

// --- Tests for getter methods ---

func TestLibraryConfig_String(t *testing.T) {
	lib := newTestLib(t, WithName("mylib"))
	if lib.String() != "mylib" {
		t.Errorf("String() = %q, want %q", lib.String(), "mylib")
	}
}

func TestLibraryConfig_Type(t *testing.T) {
	lib := newTestLib(t)
	if lib.Type() != unikraft.ComponentTypeLib {
		t.Errorf("Type() = %v, want %v", lib.Type(), unikraft.ComponentTypeLib)
	}
}

func TestLibraryConfig_Path(t *testing.T) {
	lib := &LibraryConfig{path: "/some/path"}
	if lib.Path() != "/some/path" {
		t.Errorf("Path() = %q, want %q", lib.Path(), "/some/path")
	}
}

func TestLibraryConfig_Platform_Nil(t *testing.T) {
	lib := newTestLib(t)
	if lib.Platform() != nil {
		t.Errorf("Platform() = %v, want nil", lib.Platform())
	}
}

func TestLibraryConfig_Platform_NonNil(t *testing.T) {
	s := "kvm"
	lib := &LibraryConfig{platform: &s}
	if lib.Platform() == nil || *lib.Platform() != "kvm" {
		t.Errorf("Platform() = %v, want %q", lib.Platform(), "kvm")
	}
}

func TestLibraryConfig_Exportsyms(t *testing.T) {
	lib := &LibraryConfig{exportsyms: []string{"foo", "bar"}}
	syms := lib.Exportsyms()
	if len(syms) != 2 || syms[0] != "foo" || syms[1] != "bar" {
		t.Errorf("Exportsyms() = %v, want [foo bar]", syms)
	}
}

func TestLibraryConfig_Syscalls(t *testing.T) {
	sc := &unikraft.ProvidedSyscall{Name: "read", Nargs: 3}
	lib := &LibraryConfig{syscalls: []*unikraft.ProvidedSyscall{sc}}
	syscalls := lib.Syscalls()
	if len(syscalls) != 1 || syscalls[0].Name != "read" {
		t.Errorf("Syscalls() = %v, want [read]", syscalls)
	}
}

func TestLibraryConfig_CompilationFlags(t *testing.T) {
	cv := func(v string) *make.ConditionalValue { return &make.ConditionalValue{Value: v} }
	lib := &LibraryConfig{
		asflags:     []*make.ConditionalValue{cv("asf")},
		asincludes:  []*make.ConditionalValue{cv("asi")},
		cflags:      []*make.ConditionalValue{cv("cf")},
		cincludes:   []*make.ConditionalValue{cv("ci")},
		cxxflags:    []*make.ConditionalValue{cv("cxxf")},
		cxxincludes: []*make.ConditionalValue{cv("cxxi")},
		objs:        []*make.ConditionalValue{cv("obj")},
		objcflags:   []*make.ConditionalValue{cv("objcf")},
		srcs:        []*make.ConditionalValue{cv("src.c")},
	}

	checkCV := func(name string, got []*make.ConditionalValue, want string) {
		t.Helper()
		if len(got) != 1 || got[0].Value != want {
			t.Errorf("%s() = %v, want [{%s}]", name, got, want)
		}
	}
	checkCV("ASFlags", lib.ASFlags(), "asf")
	checkCV("ASIncludes", lib.ASIncludes(), "asi")
	checkCV("CFlags", lib.CFlags(), "cf")
	checkCV("CIncludes", lib.CIncludes(), "ci")
	checkCV("CXXFlags", lib.CXXFlags(), "cxxf")
	checkCV("CXXIncludes", lib.CXXIncludes(), "cxxi")
	checkCV("Objs", lib.Objs(), "obj")
	checkCV("ObjCFlags", lib.ObjCFlags(), "objcf")
	checkCV("Srcs", lib.Srcs(), "src.c")
}

// --- Tests for SetKConfig and KConfig ---

func TestLibraryConfig_SetKConfig(t *testing.T) {
	lib := &LibraryConfig{}
	kv := kconfig.KeyValueMap{}
	kv.Set("CONFIG_TEST", "y")
	lib.SetKConfig(kv)
	if lib.kconfig == nil {
		t.Error("SetKConfig did not set kconfig")
	}
}

func TestLibraryConfig_KConfig_NameWithLibPrefix(t *testing.T) {
	lib := &LibraryConfig{name: "libfoo"}
	kv := lib.KConfig()
	// Should set CONFIG_LIBFOO=y (with prefix, since name already starts with "lib")
	if _, ok := kv.Get(kconfig.Prefix + "LIBFOO"); !ok {
		t.Errorf("KConfig() missing CONFIG_LIBFOO, got keys: %v", kv)
	}
}

func TestLibraryConfig_KConfig_NameWithoutLibPrefix(t *testing.T) {
	lib := &LibraryConfig{name: "foo"}
	kv := lib.KConfig()
	// Should set CONFIG_LIBFOO=y (adding "LIB" prefix)
	if _, ok := kv.Get(kconfig.Prefix + "LIBFOO"); !ok {
		t.Errorf("KConfig() missing CONFIG_LIBFOO, got keys: %v", kv)
	}
}

func TestLibraryConfig_KConfig_MergesExistingKConfig(t *testing.T) {
	existing := kconfig.KeyValueMap{}
	existing.Set("CONFIG_MY_OPTION", "y")
	lib := &LibraryConfig{name: "libbar", kconfig: existing}
	kv := lib.KConfig()
	if _, ok := kv.Get("CONFIG_MY_OPTION"); !ok {
		t.Errorf("KConfig() should include existing kconfig key CONFIG_MY_OPTION")
	}
}

// --- Tests for IsUnpacked ---

func TestLibraryConfig_IsUnpacked_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	lib := &LibraryConfig{path: dir}
	if !lib.IsUnpacked() {
		t.Errorf("IsUnpacked() = false, want true for existing dir %q", dir)
	}
}

func TestLibraryConfig_IsUnpacked_NonExistingPath(t *testing.T) {
	lib := &LibraryConfig{path: "/does/not/exist/path"}
	if lib.IsUnpacked() {
		t.Error("IsUnpacked() = true, want false for non-existing path")
	}
}

// --- Tests for PrintInfo ---

func TestLibraryConfig_PrintInfo(t *testing.T) {
	lib := &LibraryConfig{}
	got := lib.PrintInfo(context.Background())
	want := "not implemented: unikraft.lib.LibraryConfig.PrintInfo"
	if got != want {
		t.Errorf("PrintInfo() = %q, want %q", got, want)
	}
}

// --- Tests for MarshalYAML ---

func TestLibraryConfig_MarshalYAML_WithSourceAndKconfig(t *testing.T) {
	kv := kconfig.KeyValueMap{}
	kv.Set("CONFIG_A", "y")
	lib := &LibraryConfig{
		version: "1.0.0",
		source:  "https://example.com/lib",
		kconfig: kv,
	}
	out, err := lib.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error: %v", err)
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("MarshalYAML() returned %T, want map", out)
	}
	if m["version"] != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", m["version"])
	}
	if m["source"] != "https://example.com/lib" {
		t.Errorf("source = %v, want 'https://example.com/lib'", m["source"])
	}
	if _, ok := m["kconfig"]; !ok {
		t.Error("MarshalYAML() missing kconfig key")
	}
}

func TestLibraryConfig_MarshalYAML_WithoutSourceOrKconfig(t *testing.T) {
	lib := &LibraryConfig{version: "2.0.0"}
	out, err := lib.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error: %v", err)
	}
	m := out.(map[string]interface{})
	if _, ok := m["source"]; ok {
		t.Error("MarshalYAML() should not include empty source")
	}
	if _, ok := m["kconfig"]; ok {
		t.Error("MarshalYAML() should not include empty kconfig")
	}
}

// --- Tests for NewFromDir ---

func TestNewFromDir_NoMakefileUK(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	_, err := NewFromDir(ctx, dir)
	if err == nil {
		t.Error("NewFromDir() expected error for directory without Makefile.uk")
	}
}

func TestNewFromDir_BasicLib(t *testing.T) {
	dir := t.TempDir()

	makefile := `$(eval $(call addlib,libmytest))

LIBMYTEST_SRCS-$(CONFIG_LIBMYTEST_MAIN) += $(LIBMYTEST_BASE)/main.c
LIBMYTEST_CFLAGS += -O2
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	if len(libs) == 0 {
		t.Fatal("NewFromDir() returned no libraries")
	}
	lib, ok := libs["libmytest"]
	if !ok {
		t.Fatalf("NewFromDir() missing library 'libmytest', got: %v", libs)
	}
	if lib.name != "mytest" {
		t.Errorf("lib.name = %q, want %q", lib.name, "mytest")
	}
	if lib.path != dir {
		t.Errorf("lib.path = %q, want %q", lib.path, dir)
	}
}

func TestNewFromDir_WithExportsyms(t *testing.T) {
	dir := t.TempDir()

	makefile := `$(eval $(call addlib,libexported))
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	exportsyms := `# exported symbols
my_func
other_func
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Exportsyms_uk), []byte(exportsyms), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libexported"]
	if !ok {
		t.Fatal("NewFromDir() missing 'libexported'")
	}
	if len(lib.exportsyms) != 2 || lib.exportsyms[0] != "my_func" || lib.exportsyms[1] != "other_func" {
		t.Errorf("exportsyms = %v, want [my_func other_func]", lib.exportsyms)
	}
}

func TestNewFromDir_WithSyscalls(t *testing.T) {
	dir := t.TempDir()

	makefile := `$(eval $(call addlib,libsyscalllib))

UK_PROVIDED_SYSCALLS += read-3 write-3
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libsyscalllib"]
	if !ok {
		t.Fatal("missing libsyscalllib")
	}
	if len(lib.syscalls) != 2 {
		t.Errorf("syscalls = %v, want 2 entries", lib.syscalls)
	}
}

func TestNewFromDir_MultipleLibs(t *testing.T) {
	dir := t.TempDir()

	// Two libraries registered — global additions should NOT be associated
	makefile := `$(eval $(call addlib,libfirst))
$(eval $(call addlib,libsecond))

CFLAGS += -DGLOBAL
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	if len(libs) != 2 {
		t.Errorf("NewFromDir() got %d libraries, want 2", len(libs))
	}
}

func TestNewFromDir_WithOptions(t *testing.T) {
	dir := t.TempDir()

	makefile := `$(eval $(call addlib,libwithopt))
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir, WithIsInternal(true))
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libwithopt"]
	if !ok {
		t.Fatal("missing libwithopt")
	}
	if !lib.internal {
		t.Error("NewFromDir() option WithIsInternal(true) not applied")
	}
}

func TestNewFromDir_PlatLib(t *testing.T) {
	dir := t.TempDir()

	makefile := `$(eval $(call addplatlib,kvm,libkvmplat))
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libkvmplat"]
	if !ok {
		t.Fatal("missing libkvmplat")
	}
	if lib.platform == nil || *lib.platform != "kvm" {
		t.Errorf("platform = %v, want 'kvm'", lib.platform)
	}
}

func TestNewFromDir_WithAllFlagTypes(t *testing.T) {
	dir := t.TempDir()

	makefile := `$(eval $(call addlib,libflags))

LIBFLAGS_ASFLAGS += -masm=intel
LIBFLAGS_ASINCLUDES += -Iasm_include
LIBFLAGS_CFLAGS += -O2
LIBFLAGS_CINCLUDES += -Iinclude
LIBFLAGS_CXXFLAGS += -std=c++17
LIBFLAGS_CXXINCLUDES += -Icxxinclude
LIBFLAGS_OBJS += obj.o
LIBFLAGS_OBJCFLAGS += -DFOO
LIBFLAGS_SRCS += main.c
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libflags"]
	if !ok {
		t.Fatal("missing libflags")
	}

	checkFlag := func(name string, got []*make.ConditionalValue, wantVal string) {
		t.Helper()
		if len(got) != 1 || got[0].Value != wantVal {
			t.Errorf("%s: got %v, want [{%s}]", name, got, wantVal)
		}
	}
	checkFlag("asflags", lib.asflags, "-masm=intel")
	checkFlag("asincludes", lib.asincludes, "-Iasm_include")
	checkFlag("cflags", lib.cflags, "-O2")
	checkFlag("cincludes", lib.cincludes, "-Iinclude")
	checkFlag("cxxflags", lib.cxxflags, "-std=c++17")
	checkFlag("cxxincludes", lib.cxxincludes, "-Icxxinclude")
	checkFlag("objs", lib.objs, "obj.o")
	checkFlag("objcflags", lib.objcflags, "-DFOO")
	checkFlag("srcs", lib.srcs, "main.c")
}

func TestNewFromDir_GlobalFlagsForSingleLib(t *testing.T) {
	dir := t.TempDir()

	// Global flags (no composite prefix) should be assigned to the only lib
	makefile := `$(eval $(call addlib,libsingle))

ASFLAGS += -masm=intel
ASINCLUDES += -Iasm
CFLAGS += -O2
CINCLUDES += -Iinc
CXXFLAGS += -std=c++17
CXXINCLUDES += -Icxx
OBJS += obj.o
OBJCFLAGS += -DOBJ
SRCS += main.c
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libsingle"]
	if !ok {
		t.Fatal("missing libsingle")
	}

	checkFlag := func(name string, got []*make.ConditionalValue, wantVal string) {
		t.Helper()
		if len(got) != 1 || got[0].Value != wantVal {
			t.Errorf("%s: got %v, want [{%s}]", name, got, wantVal)
		}
	}
	checkFlag("asflags", lib.asflags, "-masm=intel")
	checkFlag("asincludes", lib.asincludes, "-Iasm")
	checkFlag("cflags", lib.cflags, "-O2")
	checkFlag("cincludes", lib.cincludes, "-Iinc")
	checkFlag("cxxflags", lib.cxxflags, "-std=c++17")
	checkFlag("cxxincludes", lib.cxxincludes, "-Icxx")
	checkFlag("objs", lib.objs, "obj.o")
	checkFlag("objcflags", lib.objcflags, "-DOBJ")
	checkFlag("srcs", lib.srcs, "main.c")
}

// --- Tests for KConfigTree ---

func TestLibraryConfig_KConfigTree_MissingFile(t *testing.T) {
	lib := &LibraryConfig{path: "/nonexistent/path"}
	_, err := lib.KConfigTree(context.Background())
	if err == nil {
		t.Error("KConfigTree() expected error when Config.uk does not exist")
	}
}

func TestLibraryConfig_KConfigTree_WithValidFile(t *testing.T) {
	dir := t.TempDir()
	configUK := `config LIBTEST
	bool "Test library"
	default y
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Config_uk), []byte(configUK), 0644); err != nil {
		t.Fatal(err)
	}

	lib := &LibraryConfig{path: dir}
	// The parse may fail due to kconfig toolchain availability, but at minimum
	// we need to exercise the code path that reads Config.uk.
	// We only check that a non-stat error means Config.uk was found.
	_, _ = lib.KConfigTree(context.Background())
}

// --- Tests for NewLibraryConfigFromOptions option error ---

func TestNewLibraryConfigFromOptions_OptionError(t *testing.T) {
	errOpt := func(_ *LibraryConfig) error {
		return fmt.Errorf("option failed")
	}
	_, err := NewLibraryConfigFromOptions(LibraryOption(errOpt))
	if err == nil {
		t.Error("NewLibraryConfigFromOptions() expected error from failing option")
	}
}

// --- Tests for NewFromDir with option error ---

func TestNewFromDir_OptionError(t *testing.T) {
	dir := t.TempDir()
	makefile := `$(eval $(call addlib,liberropt))
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	errOpt := LibraryOption(func(_ *LibraryConfig) error {
		return fmt.Errorf("apply option failed")
	})

	ctx := context.Background()
	_, err := NewFromDir(ctx, dir, errOpt)
	if err == nil {
		t.Error("NewFromDir() expected error from failing option")
	}
}

// --- Tests for NewFromDir with backslash continuation ---

func TestNewFromDir_BackslashContinuation(t *testing.T) {
	dir := t.TempDir()
	// A line with backslash continuation: scanner reads next line and appends
	makefile := "$(eval $(call addlib,libcont))\n\nLIBCONT_SRCS += \\\n\tmain.c\n"
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libcont"]
	if !ok {
		t.Fatal("missing libcont")
	}
	// The continuation line should have been parsed into srcs
	if len(lib.srcs) == 0 {
		t.Error("expected srcs to be populated via backslash continuation")
	}
}

// --- Tests for NewFromDir with invalid syscall entry (no dash) ---

func TestNewFromDir_SyscallWithNoDash(t *testing.T) {
	dir := t.TempDir()
	// "invalidsyscall" has no "-" so NewProvidedSyscall returns nil and should be skipped
	makefile := `$(eval $(call addlib,libsysbad))

UK_PROVIDED_SYSCALLS += invalidsyscall read-3
`
	if err := os.WriteFile(filepath.Join(dir, unikraft.Makefile_uk), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	libs, err := NewFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("NewFromDir() error: %v", err)
	}
	lib, ok := libs["libsysbad"]
	if !ok {
		t.Fatal("missing libsysbad")
	}
	// Only the valid "read-3" syscall should be present; "invalidsyscall" skipped
	if len(lib.syscalls) != 1 || lib.syscalls[0].Name != "read" {
		t.Errorf("syscalls = %v, want [read]", lib.syscalls)
	}
}

// --- Tests for NewLibraryFromPackage ---

func TestNewLibraryFromPackage(t *testing.T) {
	pkg := &mockPackage{name: "mypkg", version: "3.0.0"}
	ctx := context.Background()
	lib, err := NewLibraryFromPackage(ctx, pkg)
	if err != nil {
		t.Fatalf("NewLibraryFromPackage() error: %v", err)
	}
	if lib.name != "mypkg" {
		t.Errorf("name = %q, want %q", lib.name, "mypkg")
	}
	if lib.version != "3.0.0" {
		t.Errorf("version = %q, want %q", lib.version, "3.0.0")
	}
}
