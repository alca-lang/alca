// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/jit"
	"github.com/alca-lang/alca/pkg/typecheck"
	"github.com/alca-lang/alca/pkg/vm"
)

var nativeMathMembers = map[string]int{
	"abs": 1, "min": 2, "max": 2, "clamp": 3,
	"sqrt": 1, "round": 1, "floor": 1, "ceil": 1,
}

func TestStdlibSupportTableCoversCanonicalTable(t *testing.T) {
	for _, mode := range []jit.StdlibExecutionMode{
		jit.StdlibInterpreter, jit.StdlibVM, jit.StdlibThreadedJIT,
		jit.StdlibAMD64JIT, jit.StdlibPureNative,
	} {
		rows := jit.StdlibSupportTable(mode)
		want := 0
		for _, module := range typecheck.StdlibModules() {
			members, _ := typecheck.StdlibMembers(module)
			want += len(members)
		}
		if len(rows) != want {
			t.Fatalf("%s: table has %d rows, want %d", mode, len(rows), want)
		}
		for _, row := range rows {
			if !row.Known {
				t.Errorf("%s: %s.%s is marked unknown", mode, row.Module, row.Member)
			}
			if row.Detail == "" {
				t.Errorf("%s: %s.%s has no detail", mode, row.Module, row.Member)
			}
			if mode == jit.StdlibPureNative && row.Tier != jit.StdlibUnavailableTier {
				t.Errorf("%s: %s.%s tier = %s, want unavailable", mode, row.Module, row.Member, row.Tier)
			}
		}
	}
}

func TestStdlibSupportNativeMathIntrinsicsOnly(t *testing.T) {
	rows := jit.StdlibSupportTable(jit.StdlibAMD64JIT)
	native := map[string]bool{}
	for _, row := range rows {
		if row.Tier != jit.StdlibNativeTier {
			continue
		}
		if row.Module != "math" {
			t.Errorf("non-math member %s.%s classified native", row.Module, row.Member)
		}
		if _, ok := nativeMathMembers[row.Member]; !ok {
			t.Errorf("unexpected native math intrinsic %q", row.Member)
		}
		native[row.Member] = true
	}
	if len(native) != len(nativeMathMembers) {
		t.Fatalf("native intrinsics = %v, want %d entries", native, len(nativeMathMembers))
	}
	for name, arity := range nativeMathMembers {
		if !native[name] {
			t.Errorf("math.%s is not classified native", name)
		}
		id, gotArity, ok := vm.MathBuiltinID(name)
		if !ok || id == 0 {
			t.Errorf("vm.MathBuiltinID(%q) is not supported", name)
		}
		if gotArity != arity {
			t.Errorf("vm.MathBuiltinID(%q) arity = %d, want %d", name, gotArity, arity)
		}
	}
}

func TestStdlibSupportPerMode(t *testing.T) {
	tests := []struct {
		module, member string
		mode           jit.StdlibExecutionMode
		want           jit.StdlibTier
	}{
		{"math", "abs", jit.StdlibInterpreter, jit.StdlibDirectTier},
		{"math", "abs", jit.StdlibVM, jit.StdlibDirectTier},
		{"math", "abs", jit.StdlibThreadedJIT, jit.StdlibJITHelperTier},
		{"math", "abs", jit.StdlibAMD64JIT, jit.StdlibNativeTier},
		{"math", "pow", jit.StdlibAMD64JIT, jit.StdlibJITHelperTier},
		{"math", "pow", jit.StdlibVM, jit.StdlibDirectTier},
		{"string", "upper", jit.StdlibAMD64JIT, jit.StdlibJITHelperTier},
		{"string", "upper", jit.StdlibThreadedJIT, jit.StdlibJITHelperTier},
		{"string", "upper", jit.StdlibVM, jit.StdlibDirectTier},
		{"string", "upper", jit.StdlibPureNative, jit.StdlibUnavailableTier},
		{"crypto", "sha256", jit.StdlibAMD64JIT, jit.StdlibJITHelperTier},
		{"crypto", "sha256", jit.StdlibPureNative, jit.StdlibUnavailableTier},
		{"io", "read_file", jit.StdlibAMD64JIT, jit.StdlibJITHelperTier},
		{"libtorch", "matmul", jit.StdlibAMD64JIT, jit.StdlibJITHelperTier},
		{"urllib2", "join", jit.StdlibAMD64JIT, jit.StdlibJITHelperTier},
		{"urllib2", "request", jit.StdlibAMD64JIT, jit.StdlibJITHelperTier},
		{"time", "format", jit.StdlibThreadedJIT, jit.StdlibJITHelperTier},
		{"time", "now", jit.StdlibThreadedJIT, jit.StdlibJITHelperTier},
	}
	for _, test := range tests {
		got := jit.ClassifyStdlibMember(test.module, test.member, test.mode)
		if !got.Known {
			t.Errorf("%s.%s (%s) unexpectedly unknown", test.module, test.member, test.mode)
			continue
		}
		if got.Tier != test.want {
			t.Errorf("%s.%s (%s) tier = %s, want %s", test.module, test.member, test.mode, got.Tier, test.want)
		}
	}
}

func TestStdlibSupportCanonicalizesAliasesAndRejectsUnknown(t *testing.T) {
	for _, module := range []string{"regex", "std.re", "re"} {
		got := jit.ClassifyStdlibMember(module, "replace", jit.StdlibAMD64JIT)
		if !got.Known || got.Module != "re" || got.Tier != jit.StdlibJITHelperTier {
			t.Errorf("%s.replace = %+v, want canonical re JIT helper", module, got)
		}
	}
	for _, module := range []string{"str", "std.string"} {
		got := jit.ClassifyStdlibMember(module, "upper", jit.StdlibVM)
		if !got.Known || got.Module != "string" || got.Tier != jit.StdlibDirectTier {
			t.Errorf("%s.upper = %+v, want canonical string engine-direct", module, got)
		}
	}
	if got := jit.ClassifyStdlibMember("zip", "zip_create", jit.StdlibVM); !got.Known || got.Module != "archive" {
		t.Errorf("zip.zip_create = %+v, want archive member", got)
	}
	if got := jit.ClassifyStdlibMember("nope", "f", jit.StdlibVM); got.Known || got.Tier != jit.StdlibUnavailableTier {
		t.Errorf("unknown module = %+v, want unavailable", got)
	}
	if got := jit.ClassifyStdlibMember("math", "not_a_member", jit.StdlibVM); got.Known || got.Tier != jit.StdlibUnavailableTier {
		t.Errorf("unknown member = %+v, want unavailable", got)
	}
}
