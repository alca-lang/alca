// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm

import "testing"

func TestWhileCountingMatchesInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "exclusive upper bound",
			src:  "i := 0;\ntotal := 0;\nwhile i < 5:\n    total = total + i;\n    i = i + 1;\nprint(string(total));\n",
			want: "10",
		},
		{
			name: "inclusive upper bound",
			src:  "i := 1;\ntotal := 0;\nwhile i <= 5:\n    total = total + i;\n    i = i + 1;\nprint(string(total));\n",
			want: "15",
		},
		{
			name: "continue advances the counter",
			src:  "i := 0;\ntotal := 0;\nwhile i < 6:\n    i = i + 1;\n    if i == 3:\n        continue;\n    total = total + i;\nprint(string(total));\n",
			want: "18",
		},
		{
			name: "break stops the loop",
			src:  "i := 0;\ntotal := 0;\nwhile i < 6:\n    i = i + 1;\n    if i == 4:\n        break;\n    total = total + i;\nprint(string(total));\n",
			want: "6",
		},
		{
			name: "zero iterations",
			src:  "i := 10;\ntotal := 0;\nwhile i < 5:\n    total = total + i;\n    i = i + 1;\nprint(string(total));\n",
			want: "0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interpOut, vmOut := runOnBothEngines(t, tc.src)
			if interpOut != tc.want {
				t.Errorf("interpreter = %q, want %q", interpOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("vm = %q, want %q", vmOut, tc.want)
			}
		})
	}
}
