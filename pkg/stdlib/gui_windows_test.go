//go:build windows

package stdlib_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestRegisterGUIBuiltins(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterGUIBuiltins(env)

	// Check gui module registered
	val, ok := env.Get("gui")
	if !ok {
		t.Fatalf("expected 'gui' module to be registered in environment")
	}

	guiMap, ok := val.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("expected 'gui' to be a MapValue, got %T", val)
	}

	expectedMethods := []string{
		"alert", "msgbox", "confirm", "prompt",
		"open_file", "save_file", "notify",
		"screen_size", "width", "height", "backends", "set_backend",
		"webview", "window", "tk_window", "qt_window",
		"task_manager", "native_task_manager",
	}

	for _, m := range expectedMethods {
		fnVal, exists := guiMap.Get(m)
		if !exists {
			t.Errorf("expected gui module to contain method %q", m)
		}
		if _, isFn := fnVal.(*interpreter.BuiltinFunction); !isFn {
			t.Errorf("expected method %q to be *BuiltinFunction, got %T", m, fnVal)
		}
	}
}

func TestNativeWindowSpecification(t *testing.T) {
	win := stdlib.NewNativeWindow("Test Window", 600, 400)
	if win.Title != "Test Window" {
		t.Errorf("expected title 'Test Window', got %q", win.Title)
	}

	lbl := win.AddLabel("Status: Ready")
	if lbl.Type != "label" || lbl.Text != "Status: Ready" {
		t.Errorf("unexpected label widget: %+v", lbl)
	}

	btn := win.AddButton("Click Me", nil)
	if btn.Type != "button" || btn.Text != "Click Me" {
		t.Errorf("unexpected button widget: %+v", btn)
	}

	chk := win.AddCheckbox("Enable", true)
	if chk.Type != "checkbox" || chk.Value != "true" {
		t.Errorf("unexpected checkbox widget: %+v", chk)
	}

	prg := win.AddProgressBar(80)
	if prg.Type != "progress" || prg.Value != "80" {
		t.Errorf("unexpected progress widget: %+v", prg)
	}

	if len(win.Widgets) != 4 {
		t.Errorf("expected 4 widgets, got %d", len(win.Widgets))
	}
}
