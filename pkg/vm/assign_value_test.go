// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import "testing"

// TestAssignmentAsValueOnBothEngines covers an assignment that is used as a
// value. The bytecode VM stores the value and keeps it on the stack, which is
// what the tree-walking interpreter does.
func TestAssignmentAsValueOnBothEngines(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "assignment in an expression",
			src: `x := 0;
y := (x = 5);
println(x);
println(y);
`,
			want: "55",
		},
		{
			name: "assignment of a field",
			src: `m: map[string, int] = {"a": 1};
v := (m["a"] = 7);
println(m["a"]);
println(v);
`,
			want: "77",
		},
		{
			name: "assignment of a struct field",
			src: `struct Point:
    x: int = 0;

p := Point();
v := (p.x = 3);
println(p.x);
println(v);
`,
			want: "33",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			interpOut, vmOut := runOnBothEngines(t, tc.src)
			if interpOut != tc.want {
				t.Errorf("interpreter output = %q, want %q", interpOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("vm output = %q, want %q", vmOut, tc.want)
			}
		})
	}
}
