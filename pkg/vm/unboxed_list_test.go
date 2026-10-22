package vm

import "testing"

// TestUnboxedListMutationMatchesInterpreter covers the unboxed int-list
// representation: a range produces a list whose elements live in the unboxed
// Ints slice, and every operation that mutates or reorders the element slice
// must materialise it first so both engines agree.
func TestUnboxedListMutationMatchesInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "pop on a range list",
			src: `xs := 1..4;
xs.pop();
print(string(xs[0]) + "," + string(xs[1]) + "," + string(xs[2]));
`,
			want: "1,2,3",
		},
		{
			name: "index assignment on a range list",
			src: `xs := 1..3;
xs[0] = 99;
print(string(xs[0]) + "," + string(xs[1]));
`,
			want: "99,2",
		},
		{
			name: "push on a range list",
			src: `xs := 1..2;
xs.push(3);
print(string(len(xs)) + ":" + string(xs[2]));
`,
			want: "3:3",
		},
		{
			name: "reverse on a range list",
			src: `xs := 1..3;
xs.reverse();
print(string(xs[0]) + "," + string(xs[1]) + "," + string(xs[2]));
`,
			want: "3,2,1",
		},
		{
			name: "sort on a range list",
			src: `xs := 3..1;
xs.sort();
print(string(xs[0]) + "," + string(xs[1]) + "," + string(xs[2]));
`,
			want: "1,2,3",
		},
		{
			name: "range list length and indexing",
			src: `xs := 1..5;
print(string(len(xs)) + ":" + string(xs[4]));
`,
			want: "5:5",
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
