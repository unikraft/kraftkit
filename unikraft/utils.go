package unikraft

import (
	"debug/elf"
	"fmt"
	"os"

	"kraftkit.sh/internal/set"
)

// IsFileUnikraftUnikernel is a utility method that determines whether the
// provided input file is a Unikraft unikernel.  The file is checked with a
// number of known facts about the kernel image built with Unikraft.
func IsFileUnikraftUnikernel(path string) (bool, error) {
	fs, err := os.Stat(path)
	if err != nil {
		return false, err
	} else if fs.IsDir() {
		return false, fmt.Errorf("first positional argument is a directory: %v", path)
	}

	// Sanity check whether the provided file is an ELF kernel with
	// Unikraft-centric properties.  This check might not always work, especially
	// if the version changes and the sections change name.
	//
	// TODO(nderjung): This check should be replaced with a more stable mechanism
	// that detects whether a bootflag is set. See[0].
	// [0]: https://github.com/unikraft/unikraft/pull/
	fe, err := elf.Open(path)
	if err != nil {
		return false, err
	}

	defer fe.Close()

	knownUnikraftSections := set.NewStringSet(
		".uk_inittab",
		".uk_ctortab",
		".uk_thread_inittab",
	)
	for _, symbol := range fe.Sections {
		if knownUnikraftSections.ContainsExactly(symbol.Name) {
			return true, nil
		}
	}

	return false, fmt.Errorf("provided file is not a Unikraft unikernel")
}
