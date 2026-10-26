// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import "testing"

// TestListIterationMatchesInterpreter exercises the for-in loop, whose back edge
// is now a single fused opcode (index += 1, loop while index <= last). It covers
// the empty case, break/continue, nested loops, and the non-list iterables that
// the hidden @iter helper turns into lists.
func TestListIterationMatchesInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "empty list runs zero times",
			src:  "xs := list();\nfor x in xs:\n    print(\"body\");\nprint(\"done\");\n",
			want: "done",
		},
		{
			name: "values in order",
			src:  "xs := [10, 20, 30];\nfor x in xs:\n    print(string(x) + \",\");\n",
			want: "10,20,30,",
		},
		{
			name: "continue skips the rest of the body",
			src:  "xs := [1, 2, 3, 4];\nfor x in xs:\n    if x == 2:\n        continue;\n    print(string(x) + \",\");\n",
			want: "1,3,4,",
		},
		{
			name: "break stops the loop",
			src:  "xs := [1, 2, 3];\nfor x in xs:\n    if x == 2:\n        break;\n    print(string(x) + \",\");\n",
			want: "1,",
		},
		{
			name: "loop variable keeps the last item",
			src:  "xs := [1, 2, 3];\nt := 0;\nfor x in xs:\n    t = t + x;\nprint(string(t) + \":\" + string(x));\n",
			want: "6:3",
		},
		{
			name: "nested loops",
			src:  "for a in [1, 2]:\n    for b in [10, 20]:\n        print(string(a * b) + \",\");\n",
			want: "10,20,20,40,",
		},
		{
			name: "string iterates characters",
			src:  "for ch in \"héllo\":\n    print(ch);\n",
			want: "héllo",
		},
		{
			name: "integer iterates zero up to itself",
			src:  "for i in 3:\n    print(string(i) + \",\");\n",
			want: "0,1,2,",
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
		})
	}
}
