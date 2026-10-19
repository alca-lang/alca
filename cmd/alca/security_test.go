package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/compiler"
)

// runCLI executes cmdRun with captured stdout/stderr.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	os.Stdout, os.Stderr = outW, errW
	outDone := make(chan string)
	errDone := make(chan string)
	go func() { b, _ := io.ReadAll(outR); outDone <- string(b) }()
	go func() { b, _ := io.ReadAll(errR); errDone <- string(b) }()

	code = cmdRun(args)
	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	return <-outDone, <-errDone, code
}

func writeAlca(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.al")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestRunDeniesFileAndNetworkAccessWithoutFlags locks in the P5 capability
// policy: a program cannot read, write or reach the network unless the matching
// --allow-* flag is present. The same behaviour must hold on both engines.
func TestRunDeniesFileAndNetworkAccessWithoutFlags(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("classified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	slashSecret := filepath.ToSlash(secret)
	outPath := filepath.ToSlash(filepath.Join(dir, "out.txt"))

	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "read",
			src:  "use std.io;\nprintln(io.read_file(\"" + slashSecret + "\"));",
			want: "--allow-read",
		},
		{
			name: "write",
			src:  "use std.io;\nio.write_file(\"" + outPath + "\", \"x\");",
			want: "--allow-write",
		},
		{
			name: "network",
			src:  "use std.net;\nnet.http_get(\"http://127.0.0.1:9/\");",
			want: "--allow-net",
		},
		{
			name: "metrics server",
			src:  "use std.metrics;\nmetrics.serve(0);",
			want: "--allow-net",
		},
		{
			name: "sqlite file",
			src:  "use std.db;\ndb.sqlite(\"" + filepath.ToSlash(filepath.Join(dir, "denied.db")) + "\");",
			want: "--allow-read",
		},
		{
			name: "log file",
			src:  "use std.log;\nlog.set_file(\"" + filepath.ToSlash(filepath.Join(dir, "log.txt")) + "\");",
			want: "--allow-write",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			program := writeAlca(t, tc.src)
			for _, engineFlag := range []string{"--vm", "--interp"} {
				t.Run(engineFlag, func(t *testing.T) {
					_, stderr, code := runCLI(t, engineFlag, program)
					if code == 0 {
						t.Fatalf("run must fail without a capability flag")
					}
					if !strings.Contains(stderr, "permission denied") {
						t.Fatalf("stderr must explain the permission denial, got:\n%s", stderr)
					}
					if !strings.Contains(stderr, tc.want) {
						t.Fatalf("stderr must mention %s, got:\n%s", tc.want, stderr)
					}
				})
			}
		})
	}

	if _, err := os.Stat(filepath.Join(dir, "out.txt")); err == nil {
		t.Fatal("a denied write must not create the file")
	}
}

func TestRunAllowFlagsPermitAccessOnBothEngines(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(input, []byte("allowed"), 0o644); err != nil {
		t.Fatal(err)
	}
	program := writeAlca(t, "use std.io;\nprintln(io.read_file(\""+filepath.ToSlash(input)+"\"));")
	for _, engineFlag := range []string{"--vm", "--interp"} {
		t.Run(engineFlag, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, engineFlag, "--allow-read="+dir, program)
			if code != 0 {
				t.Fatalf("allowed read failed with %d:\n%s", code, stderr)
			}
			if stdout != "allowed\n" {
				t.Fatalf("stdout = %q, want %q", stdout, "allowed\n")
			}
		})
	}
}

func TestRunAllowAllPermitsReadAndWrite(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.txt")
	output := filepath.Join(dir, "output.txt")
	if err := os.WriteFile(input, []byte("copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	program := writeAlca(t, "use std.io;\n"+
		"data := io.read_file(\""+filepath.ToSlash(input)+"\");\n"+
		"io.write_file(\""+filepath.ToSlash(output)+"\", data);\n"+
		"println(\"done\");\n")
	for _, engineFlag := range []string{"--vm", "--interp"} {
		t.Run(engineFlag, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, engineFlag, "--allow-all", program)
			if code != 0 {
				t.Fatalf("--allow-all run failed with %d:\n%s", code, stderr)
			}
			if stdout != "done\n" {
				t.Fatalf("stdout = %q", stdout)
			}
		})
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "copy\n" {
		t.Fatalf("output file = %q", got)
	}
}

// buildStandalone builds a host application with the real runtime stub. The
// test is skipped when the generated runtimes have not been built.
func buildStandalone(t *testing.T, src string) string {
	t.Helper()
	root := repoRoot(t)
	stub := filepath.Join(root, "runtimes", compiler.RuntimeName(compiler.HostTarget()))
	if _, err := os.Stat(stub); err != nil {
		t.Skipf("cross runtimes not built; run `go run ./cmd/alca build-runtimes --out runtimes`")
	}

	dir := t.TempDir()
	input := filepath.Join(dir, "app.al")
	if err := os.WriteFile(input, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	output := compiler.ExeName(filepath.Join(dir, "app"), compiler.HostTarget())
	if code := cmdBuild([]string{input, "--runtime", stub, "-o", output}); code != 0 {
		t.Fatalf("cmdBuild exited with %d", code)
	}
	return output
}

func runArtifact(t *testing.T, output string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(output, args...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run %s: %v", output, err)
		}
	}
	return out.String(), errOut.String(), code
}

// TestStandaloneApplicationEnforcesCapabilities verifies that a built
// application, not only the alca CLI, starts with deny-all and accepts the same
// leading capability flags.
func TestStandaloneApplicationEnforcesCapabilities(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("classified"), 0o644); err != nil {
		t.Fatal(err)
	}

	reader := buildStandalone(t, "use std.io;\nprintln(io.read_file(\""+filepath.ToSlash(secret)+"\"));\n")
	if _, stderr, code := runArtifact(t, reader); code == 0 || !strings.Contains(stderr, "--allow-read") {
		t.Fatalf("standalone read without a flag: code=%d stderr=%q", code, stderr)
	}
	stdout, stderr, code := runArtifact(t, reader, "--allow-read="+dir)
	if code != 0 {
		t.Fatalf("standalone allowed read failed with %d:\n%s", code, stderr)
	}
	if stdout != "classified\n" {
		t.Fatalf("standalone read output = %q", stdout)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("network-ok"))
	}))
	defer server.Close()
	fetcher := buildStandalone(t, "use std.net;\nr := net.http_get(\""+server.URL+"\");\nprintln(r[\"body\"]);\n")
	if _, stderr, code := runArtifact(t, fetcher); code == 0 || !strings.Contains(stderr, "--allow-net") {
		t.Fatalf("standalone network without a flag: code=%d stderr=%q", code, stderr)
	}
	hostPort := strings.TrimPrefix(server.URL, "http://")
	stdout, stderr, code = runArtifact(t, fetcher, "--allow-net="+hostPort)
	if code != 0 {
		t.Fatalf("standalone allowed network request failed with %d:\n%s", code, stderr)
	}
	if stdout != "network-ok\n" {
		t.Fatalf("standalone network output = %q", stdout)
	}
}
