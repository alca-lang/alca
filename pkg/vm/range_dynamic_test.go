// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import "testing"

// TestDynamicRangeLoopMatchesInterpreter exercises the direction-aware counting
// loop emitted for `for v in a..b` with non-literal bounds. Ranges are
// inclusive, so the body must run at least once for every pair of bounds
// (ascending, descending, and equal), and break/continue/final-value semantics
// must match the interpreter's fixed-item-list iteration exactly.
func TestDynamicRangeLoopMatchesInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "half open ascending bounds",
			src: `a := 2;
b := 5;
for i in a..b:
    print(string(i) + "-");
`,
			want: "2-3-4-",
		},
		{
			name: "ascending bounds inclusive",
			src: `a := 2;
b := 5;
for i in a..=b:
    print(string(i) + "-");
`,
			want: "2-3-4-5-",
		},
		{
			name: "descending bounds inclusive",
			src: `a := 5;
b := 2;
for i in a..=b:
    print(string(i) + "-");
`,
			want: "5-4-3-2-",
		},
		{
			name: "equal bounds iterate once inclusive",
			src: `a := 7;
b := 7;
for i in a..=b:
    print(string(i) + "-");
`,
			want: "7-",
		},
		{
			name: "break stops early",
			src: `a := 1;
b := 10;
for i in a..=b:
    if i == 4:
        break;
    print(string(i) + "-");
`,
			want: "1-2-3-",
		},
		{
			name: "continue skips even values ascending",
			src: `a := 1;
b := 6;
for i in a..=b:
    if i % 2 == 0:
        continue;
    print(string(i) + "-");
`,
			want: "1-3-5-",
		},
		{
			name: "continue skips even values descending",
			src: `a := 6;
b := 1;
for i in a..=b:
    if i % 2 == 0:
        continue;
    print(string(i) + "-");
`,
			want: "5-3-1-",
		},
		{
			name: "final value ascending is last item",
			src: `a := 3;
b := 6;
for i in a..=b:
    x := i;
println(string(i));
`,
			want: "6",
		},
		{
			name: "final value descending is last item",
			src: `a := 6;
b := 3;
for i in a..=b:
    x := i;
println(string(i));
`,
			want: "3",
		},
		{
			name: "expression bounds are evaluated once",
			src: `n := 2;
for i in 1..=n*2:
    print(string(i) + "-");
`,
			want: "1-2-3-4-",
		},
		{
			name: "nested dynamic loops",
			src: `n := 3;
for i in 1..=n:
    for j in 1..=n:
        print(string(i) + string(j) + " ");
`,
			want: "11 12 13 21 22 23 31 32 33 ",
		},
		{
			name: "writing the loop variable does not change iteration count",
			src: `n := 0;
b := 4;
for i in 0..=b:
    n = n + 1;
    i = i + 1;
println(string(n));
`,
			want: "5",
		},
		{
			name: "bound referring to the loop name uses enclosing binding",
			src: `i := 3;
for i in 0..=i:
    print(string(i) + "-");
`,
			want: "0-1-2-3-",
		},
		{
			name: "non-int bounds iterate zero times like EvalRange",
			src: `a: json = 1.5;
b: json = 3;
n := 0;
for i in a..b:
    n = n + 1;
println(string(n));
`,
			want: "0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpOut, vmOut := runOnBothEngines(t, tc.src)
			if interpOut != tc.want {
				t.Errorf("interpreter output = %q, want %q", interpOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("vm output = %q, want %q", vmOut, tc.want)
			}
			if interpOut != vmOut {
				t.Errorf("engines disagree: interpreter %q, vm %q", interpOut, vmOut)
			}
		})
	}
}
