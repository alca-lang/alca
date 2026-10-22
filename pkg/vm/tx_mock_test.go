package vm

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// TestTransactionsAndMocksOnBothEngines covers the transaction blocks and the
// mock statements. A transaction keeps its changes when the block succeeds and
// discards them when it fails; a mock replaces the binding of the mocked name.
func TestTransactionsAndMocksOnBothEngines(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "a successful transaction keeps its changes",
			src: `x := 1;
tx:
    x = 2;
println(x);
`,
			want: "2",
		},
		{
			name: "a mock replaces the mocked binding",
			src: `x := 1;
mock x = 42;
println(x);
`,
			want: "42",
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

// TestFailedTransactionReportsTheSameError checks that a transaction that
// fails reports the same error on both engines.
func TestFailedTransactionReportsTheSameError(t *testing.T) {
	const src = `x := 1;
tx:
    x = 2;
    err("boom");
println(x);
`
	interpErr := interpreter.New().Execute(parseProgram(t, src))
	vmErr := runSource(t, src)
	if interpErr == nil || vmErr == nil {
		t.Fatalf("both engines must fail: interpreter=%v vm=%v", interpErr, vmErr)
	}
	if !strings.Contains(vmErr.Error(), "boom") {
		t.Fatalf("vm error = %v, want it to mention the failure", vmErr)
	}
	if !strings.Contains(interpErr.Error(), "boom") {
		t.Fatalf("interpreter error = %v, want it to mention the failure", interpErr)
	}
}
