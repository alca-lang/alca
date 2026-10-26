// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLowLevelModulesOnBothEngines verifies that the low-level modules work
// on every engine: the memory module hands out raw memory and reads it back by
// address, and the protocol module encrypts and decrypts with the cipher of the
// MTProto protocol.
func TestLowLevelModulesOnBothEngines(t *testing.T) {
	src := `use std.mem;
use std.mtproto;
use std.crypto;

addr := mem.alloc(8);
mem.write_u32(addr, 305419896);
println(mem.read_u32(addr));

key := crypto.random_bytes(32);
data := crypto.random_bytes(16);
iv := crypto.random_bytes(32);
encrypted := mtproto.ige_encrypt(key, data, iv);
println(len(mtproto.ige_decrypt(key, encrypted, iv)));

println(len(mtproto.dc_list()));
println(mtproto.tl_read_int32(mtproto.tl_int32(1234)));
`
	file := filepath.Join(t.TempDir(), "lowlevel.al")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "305419896\n16\n5\n1234\n"

	for _, engineFlag := range []string{"--vm", "--interp"} {
		t.Run(engineFlag, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, engineFlag, file)
			if code != 0 {
				t.Fatalf("run failed with %d:\n%s", code, stderr)
			}
			if stdout != want {
				t.Fatalf("output = %q, want %q", stdout, want)
			}
		})
	}
}

// TestMemoryModuleRejectsForeignAddress verifies that a program cannot use the
// memory module to read the memory of the process: only the blocks it
// allocated itself are reachable.
func TestMemoryModuleRejectsForeignAddress(t *testing.T) {
	src := `use std.mem;
value := mem.read_u32(4096);
println(value);
`
	file := filepath.Join(t.TempDir(), "memory.al")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLI(t, "--vm", file)
	if code == 0 {
		t.Fatalf("reading a foreign address must fail, output:\n%s", stdout)
	}
	if !strings.Contains(stderr, "outside of the block") {
		t.Fatalf("the diagnostic must explain the refusal, got:\n%s", stderr)
	}
}
