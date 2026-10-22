package main

import (
	"fmt"
	"strings"

	"github.com/alca-lang/alca/pkg/limits"
)

// memoryFlagHint documents the memory flags in the command line help.
const memoryFlagHint = "  --max-memory=<size>       limit the memory an Alca program may hold (e.g. 256mb)\n" +
	"  --max-allocation=<size>   limit the total memory an Alca program may allocate\n" +
	"  A size of 0 disables the limit.\n"

// memoryFlags holds the memory limits requested on the command line.
type memoryFlags struct {
	live  int64
	alloc int64
	set   bool
}

// parseMemoryFlags extracts the --max-memory and --max-allocation flags from
// args and returns the remaining arguments unchanged.
func parseMemoryFlags(args []string) (memoryFlags, []string, error) {
	var flags memoryFlags
	rest := make([]string, 0, len(args))
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--max-memory="):
			value, err := limits.ParseSize(strings.TrimPrefix(a, "--max-memory="))
			if err != nil {
				return flags, nil, fmt.Errorf("invalid --max-memory value: %w", err)
			}
			flags.live = value
			flags.set = true
		case strings.HasPrefix(a, "--max-allocation="):
			value, err := limits.ParseSize(strings.TrimPrefix(a, "--max-allocation="))
			if err != nil {
				return flags, nil, fmt.Errorf("invalid --max-allocation value: %w", err)
			}
			flags.alloc = value
			flags.set = true
		default:
			rest = append(rest, a)
		}
	}
	return flags, rest, nil
}

// applyMemoryFlags installs the requested limits on the shared meter and
// starts with a clean slate. Without the flags the meter stays unlimited, so
// the behavior of a program does not change.
func applyMemoryFlags(flags memoryFlags) {
	if !flags.set {
		return
	}
	limits.Reset()
	limits.SetLimits(flags.live, flags.alloc)
}
