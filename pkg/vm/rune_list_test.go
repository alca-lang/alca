package vm

import "testing"

// TestRuneListMatchesInterpreter covers the rune-backed list used to iterate a
// string: the list stands for one single-character string per rune, so both
// engines must produce the same characters in the same order.
func TestRuneListMatchesInterpreter(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "for over a string yields its characters",
			src: `for ch in "héllo":
    print(ch);
`,
			want: "héllo",
		},
		{
			name: "for over an empty string yields nothing",
			src: `for ch in "":
    print("no");
print("done");
`,
			want: "done",
		},
		{
			name: "string length counts runes",
			src: `print(string(len("héllo")));
`,
			want: "5",
		},
		{
			name: "indexing a string yields a character",
			src: `s := "héllo";
print(s[1]);
`,
			want: "é",
		},
		{
			name: "a string list round-trips through a list",
			src: `xs := ["a", "b"];
for x in xs:
    print(x);
`,
			want: "ab",
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
