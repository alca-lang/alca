package jit

import (
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/typecheck"
	"github.com/alca-lang/alca/pkg/vm"
)

// StdlibExecutionMode identifies the engine whose standard-library behavior is
// being described. The distinction matters because a call that is "callable"
// everywhere can still have very different execution paths.
type StdlibExecutionMode int

const (
	// StdlibInterpreter is the tree-walking interpreter.
	StdlibInterpreter StdlibExecutionMode = iota
	// StdlibVM is the bytecode VM without the JIT.
	StdlibVM
	// StdlibThreadedJIT is the portable threaded JIT. Pure stdlib calls fused
	// into OpStdlibCall execute in the JIT helper tier; host-facing calls and
	// module imports also execute there. Dynamic non-builtin callables bail out
	// to the VM.
	StdlibThreadedJIT
	// StdlibAMD64JIT is the native AMD64 backend. It translates the eight
	// numeric math intrinsics inline and executes fused pure and host stdlib
	// calls through the JIT runtime-helper tier.
	StdlibAMD64JIT
	// StdlibPureNative is a pure native PE/AOT executable with no VM. Standard
	// library modules are not available in this mode.
	StdlibPureNative
)

func (m StdlibExecutionMode) String() string {
	switch m {
	case StdlibInterpreter:
		return "interpreter"
	case StdlibVM:
		return "vm"
	case StdlibThreadedJIT:
		return "threaded-jit"
	case StdlibAMD64JIT:
		return "amd64-jit"
	case StdlibPureNative:
		return "pure-native"
	default:
		return "unknown"
	}
}

// StdlibTier describes how a standard-library member executes.
type StdlibTier int

const (
	// StdlibNativeTier means the member call is lowered to inline machine
	// instructions. Only the AMD64 backend has such intrinsics.
	StdlibNativeTier StdlibTier = iota
	// StdlibJITHelperTier means a JIT executes the registered member directly
	// through its runtime-helper tier and then resumes JIT code, without VM
	// bytecode dispatch for the call itself.
	StdlibJITHelperTier
	// StdlibDirectTier means the engine executes the member itself, without a
	// JIT translation step.
	StdlibDirectTier
	// StdlibDeoptTier means the JIT stops at the call site and hands execution
	// to the bytecode VM, which runs the registered stdlib function.
	StdlibDeoptTier
	// StdlibUnavailableTier means the member cannot run in this mode at all.
	StdlibUnavailableTier
)

func (t StdlibTier) String() string {
	switch t {
	case StdlibNativeTier:
		return "native-intrinsic"
	case StdlibJITHelperTier:
		return "jit-runtime-helper"
	case StdlibDirectTier:
		return "engine-direct"
	case StdlibDeoptTier:
		return "vm-deopt"
	case StdlibUnavailableTier:
		return "unavailable"
	default:
		return "unknown"
	}
}

// StdlibSupport describes one stdlib member in one execution mode.
type StdlibSupport struct {
	Module string
	Member string
	Known  bool
	Tier   StdlibTier
	Detail string
}

// nativeMathIntrinsics is the exact set of std.math members the bytecode
// compiler can lower to OpMathBuiltin and the AMD64 backend can execute inline.
// Keep this in sync with vm.MathBuiltinID.
var nativeMathIntrinsics = map[string]bool{
	"abs":   true,
	"min":   true,
	"max":   true,
	"clamp": true,
	"sqrt":  true,
	"round": true,
	"floor": true,
	"ceil":  true,
}

// canonicalStdlibModule maps the documented module aliases and the "std."
// prefix onto the canonical names used by the type-checker table.
func canonicalStdlibModule(module string) string {
	module = strings.TrimPrefix(module, "std.")
	switch module {
	case "regex":
		return "re"
	case "str":
		return "string"
	case "zip":
		return "archive"
	default:
		return module
	}
}

// ClassifyStdlibMember reports how a stdlib member is executed in the given
// mode. Unknown modules and members are returned with Known=false so callers
// can distinguish "not supported here" from "not a stdlib member".
func ClassifyStdlibMember(module, member string, mode StdlibExecutionMode) StdlibSupport {
	canonical := canonicalStdlibModule(module)
	out := StdlibSupport{Module: canonical, Member: member}

	members, ok := typecheck.StdlibMembers(canonical)
	if !ok {
		out.Tier = StdlibUnavailableTier
		out.Detail = "not a standard-library module"
		return out
	}
	known := false
	for _, name := range members {
		if name == member {
			known = true
			break
		}
	}
	if !known {
		out.Tier = StdlibUnavailableTier
		out.Detail = "not a member of std." + canonical
		return out
	}
	out.Known = true

	if mode == StdlibPureNative {
		out.Tier = StdlibUnavailableTier
		out.Detail = "a pure native PE has no VM or runtime modules; build with the runtime stub to use the standard library"
		return out
	}

	if mode == StdlibAMD64JIT && canonical == "math" && nativeMathIntrinsics[member] {
		if _, _, ok := vm.MathBuiltinID(member); ok {
			out.Tier = StdlibNativeTier
			out.Detail = "exact-arity calls through an unshadowed, unescaped std.math alias are emitted inline; mutation, aliasing, or invalid runtime types fall back to the VM for normal behavior"
			return out
		}
	}

	if mode == StdlibThreadedJIT || mode == StdlibAMD64JIT {
		out.Tier = StdlibJITHelperTier
		if vm.IsPureStdlibMember(canonical, member) {
			out.Detail = "direct unshadowed, unescaped module calls are fused and executed by the JIT engine itself; execution resumes in JIT code after the call. Mutation, aliasing, or a non-builtin binding falls back to the VM for normal behavior"
		} else {
			out.Detail = "module lookup, member access, and the registered builtin call execute in the JIT runtime-helper tier without VM bytecode dispatch; the host operation still runs its normal Go implementation. Non-builtin or dynamic bindings fall back to the VM"
		}
		return out
	}

	out.Tier = StdlibDirectTier
	out.Detail = "the engine executes the registered stdlib function directly"
	return out
}

// StdlibSupportTable returns the complete canonical stdlib member table with
// its execution classification for the given mode, sorted by module and member.
func StdlibSupportTable(mode StdlibExecutionMode) []StdlibSupport {
	var table []StdlibSupport
	for _, module := range typecheck.StdlibModules() {
		members, _ := typecheck.StdlibMembers(module)
		for _, member := range members {
			table = append(table, ClassifyStdlibMember(module, member, mode))
		}
	}
	sort.Slice(table, func(i, j int) bool {
		if table[i].Module == table[j].Module {
			return table[i].Member < table[j].Member
		}
		return table[i].Module < table[j].Module
	})
	return table
}
