package stdlib

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
)

func sampleWindow() *NativeWindow {
	win := NewNativeWindow(`Test "Window"`, 400, 300)
	win.AddLabel("Hello $world [x]")
	win.AddButton("Run", &interpreter.BuiltinFunction{
		Name: "cb",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			return interpreter.NULL
		},
	})
	win.AddInput("type here")
	win.AddCheckbox("Enabled", true)
	win.AddProgressBar(42)
	return win
}

func TestTkScriptRendersWidgets(t *testing.T) {
	script := sampleWindow().tkScript()
	for _, want := range []string{
		"package require Tk",
		"wm title .",
		"label $__alca_f.w0",
		"button $__alca_f.w1",
		"ALCA_CALL:1",
		"entry $__alca_f.w2",
		"checkbutton $__alca_f.w3",
		"ttk::progressbar $__alca_f.w4",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("Tk script missing %q", want)
		}
	}
}

func requireGUIE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("ALCA_GUI_E2E") == "" {
		t.Skip("set ALCA_GUI_E2E=1 to open real GUI windows")
	}
}

func TestTkScriptRuns(t *testing.T) {
	requireGUIE2E(t)
	wish, err := findTkInterpreter()
	if err != nil {
		t.Skip(err)
	}
	script := sampleWindow().tkScript() + "after 900 {exit}\n"
	path := filepath.Join(t.TempDir(), "alca_window.tcl")
	if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(wish, path).CombinedOutput()
	if err != nil {
		t.Fatalf("wish failed: %v\n%s", err, out)
	}
}

func TestQtScriptsRenderWidgets(t *testing.T) {
	win := sampleWindow()
	python := win.qtPythonScript("PySide6")
	for _, want := range []string{
		"from PySide6.QtWidgets import",
		"QLabel(",
		"QPushButton(",
		"ALCA_CALL:",
		"QProgressBar()",
	} {
		if !strings.Contains(python, want) {
			t.Errorf("Qt Python script missing %q", want)
		}
	}

	qml := win.qtQMLScript()
	for _, want := range []string{"ApplicationWindow {", "Label {", "Button {", "ProgressBar {", "ALCA_CALL:1"} {
		if !strings.Contains(qml, want) {
			t.Errorf("Qt QML script missing %q", want)
		}
	}
}

func TestQtPythonScriptSyntax(t *testing.T) {
	python, err := exec.LookPath("python")
	if err != nil {
		python, err = exec.LookPath("python3")
	}
	if err != nil {
		t.Skip("python not available")
	}
	path := filepath.Join(t.TempDir(), "alca_qt_window.py")
	if err := os.WriteFile(path, []byte(sampleWindow().qtPythonScript("PySide6")), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(python, "-m", "py_compile", path).CombinedOutput(); err != nil {
		t.Fatalf("generated Qt Python script has a syntax error: %v\n%s", err, out)
	}
}

func TestParseCallbackLine(t *testing.T) {
	if idx, ok := parseCallbackLine("ALCA_CALL:3"); !ok || idx != 3 {
		t.Fatalf("parseCallbackLine = %d, %v; want 3, true", idx, ok)
	}
	if idx, ok := parseCallbackLine("ALCA_CALL:7 trailing"); !ok || idx != 7 {
		t.Fatalf("parseCallbackLine with suffix = %d, %v; want 7, true", idx, ok)
	}
	if _, ok := parseCallbackLine("not a callback"); ok {
		t.Fatal("expected non-callback line to be ignored")
	}
}

func TestWindowValueBackendSelection(t *testing.T) {
	winObj := windowValue("Win", 300, 200, "tk").(*interpreter.MapValue)
	backend, _ := winObj.Get("backend")
	res := backend.(*interpreter.BuiltinFunction).Fn()
	s, ok := res.(*interpreter.StringValue)
	if !ok || s.Val != "tk" {
		t.Fatalf("expected backend tk, got %v", res)
	}

	setBackend, _ := winObj.Get("set_backend")
	setBackend.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "qt"})
	res = backend.(*interpreter.BuiltinFunction).Fn()
	if s, ok := res.(*interpreter.StringValue); !ok || s.Val != "qt" {
		t.Fatalf("expected backend qt after set_backend, got %v", res)
	}
}
