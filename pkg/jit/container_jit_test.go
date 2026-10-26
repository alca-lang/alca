// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package jit_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/jit"
)

// TestContainerHelpersCoverRepresentations exercises every ListValue backing
// representation (Elements, Ints, and Runes), negative indexes, string and map
// indexing, list mutation, struct fields, and iteration through the JIT helper
// tier. The outputs must be identical to the interpreter and bytecode VM.
func TestContainerHelpersCoverRepresentations(t *testing.T) {
	const src = `
struct Point:
    x: int;
    y: int = 0;

func probe():
    ints := [10, 20, 30];
    println(ints[0]);
    println(ints[-1]);
    ints[1] = 99;
    println(ints[1]);

    mixed := [1, "two", true];
    println(mixed[1]);

    text := "héllo";
    println(text[1]);
    println(text[-1]);

    m := {"a": 1};
    println(m["a"]);
    m["b"] = 2;
    println(m["b"]);

    p := Point(3, 4);
    println(p.x);
    p.x = 8;
    println(p.x);

    for ch in "ab":
        println(ch);
    for i in 0..=2:
        println(i);
probe();
`
	const want = "10\n30\n99\ntwo\né\no\n1\n2\n3\n8\na\nb\n0\n1\n2\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, helperCalls := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
			if helperCalls < 10 {
				t.Fatalf("runtime helper calls = %d, want at least 10", helperCalls)
			}
		})
	}
}

// TestContainerHelperErrors checks that bad index types and out-of-range
// indexes raise the same Alca error values through the helper tier.
func TestContainerHelperErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "list index type",
			src: `
func probe():
    xs := [1, 2];
    failure: error | null = null;
    xs["a"] ?>>> failure;
    if failure != null:
        println(failure.msg);
probe();
`,
			want: "list index must be an integer\n",
		},
		{
			name: "list index bounds",
			src: `
func probe():
    xs := [1, 2];
    failure: error | null = null;
    xs[5] ?>>> failure;
    if failure != null:
        println(failure.msg);
probe();
`,
			want: "index 5 out of bounds (len 2)\n",
		},
		{
			name: "string index bounds",
			src: `
func probe():
    failure: error | null = null;
    "ab"[4] ?>>> failure;
    if failure != null:
        println(failure.msg);
probe();
`,
			want: "string index 4 out of bounds\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
			mode := mode
			t.Run(tc.name+"/"+mode.String(), func(t *testing.T) {
				out, err, _ := runPureStdlibMode(t, tc.src, mode)
				if err != nil {
					t.Fatalf("run: %v", err)
				}
				if out != tc.want {
					t.Fatalf("output = %q, want %q", out, tc.want)
				}
			})
		}
	}
}
