// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import (
	"testing"
)

func functionOpcodes(t *testing.T, compiler *Compiler) [][]byte {
	t.Helper()
	var out [][]byte
	for _, value := range compiler.Bytecode().Constants {
		if fn, ok := value.(*CompiledFunction); ok {
			out = append(out, fn.Instructions)
		}
	}
	return out
}

func countOpcode(streams [][]byte, want Opcode) int {
	count := 0
	for _, ins := range streams {
		for ip := 0; ip < len(ins); {
			op := Opcode(ins[ip])
			if op == want {
				count++
			}
			ip++
			if def, err := Lookup(byte(op)); err == nil {
				for _, width := range def.OperandWidths {
					ip += width
				}
			}
		}
	}
	return count
}

func TestPureStdlibMemberClassification(t *testing.T) {
	cases := []struct {
		module, member string
		want           bool
	}{
		{"std.math", "sin", true},
		{"std.path", "join", true},
		{"std.path", "which", false},
		{"std.crypto", "sha256", true},
		{"std.crypto", "aes_encrypt", false},
		{"std.crypto", "random_bytes", false},
		{"std.urllib2", "join", true},
		{"std.urllib2", "request", false},
		{"std.time", "format", true},
		{"std.time", "now", false},
		{"std.json", "encode", true},
		{"std.csv", "parse", true},
		{"std.csv", "read", false},
		{"std.ini", "stringify", true},
		{"std.ini", "write", false},
		{"std.formats", "json.encode", true},
		{"std.formats", "csv.stringify", true},
		{"std.formats", "csv.read", false},
		{"std.formats", "ini.write", false},
	}
	for _, tc := range cases {
		if got := IsPureStdlibMember(tc.module, tc.member); got != tc.want {
			t.Errorf("IsPureStdlibMember(%q, %q) = %v, want %v", tc.module, tc.member, got, tc.want)
		}
	}
}

func TestPureStdlibCallLowering(t *testing.T) {
	program := parseProgram(t, `
use std.str as s;
use std.path;
use std.crypto;
use std.urllib2;
use std.io;
func probe():
    a := s.upper("x");
    b := path.join("a", "b");
    c := crypto.sha256("x");
    d := urllib2.join("https://example.test/a", "../b");
    e := path.which("go");
    f := crypto.random_bytes(4);
    g := urllib2.request("https://example.test");
    h := io.read_file("x");
probe();
`)
	compiler := NewCompiler()
	if err := compiler.Compile(program); err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := countOpcode(functionOpcodes(t, compiler), OpStdlibCall)
	if got != 4 {
		t.Fatalf("OpStdlibCall count = %d, want 4 pure members only", got)
	}
}

func TestPureStdlibLoweringRespectsMutationAndEscape(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "direct mutation",
			src: `use std.str as s;
s.upper = 7;
result := s.upper("x");
`,
		},
		{
			name: "alias escapes",
			src: `use std.str as s;
other := s;
result := s.upper("x");
`,
		},
		{
			name: "function shadowing",
			src: `use std.str as s;
func s():
    return "shadow";
result := s.upper("x");
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program := parseProgram(t, tc.src)
			compiler := NewCompiler()
			if err := compiler.Compile(program); err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got := countOpcode(functionOpcodes(t, compiler), OpStdlibCall); got != 0 {
				t.Fatalf("OpStdlibCall count = %d after alias escape/mutation, want 0", got)
			}
		})
	}
}
