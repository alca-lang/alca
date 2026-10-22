//go:build windows

package stdlib

import (
	"testing"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/vm"
)

// TestNativeWidgetWindow builds a real Win32 window with every supported
// widget and verifies that it is created, laid out and closed cleanly.
func TestNativeWidgetWindow(t *testing.T) {
	requireGUIE2E(t)

	win := NewNativeWindow("Alca Native Widget Test", 480, 380)
	win.SetBackend("native")
	win.AddLabel("Status: ready")
	win.AddInput("Type something...")
	win.AddCheckbox("Enable feature", true)
	win.AddProgressBar(75)
	win.AddButton("Run", nil)

	done := make(chan error, 1)
	go func() {
		done <- win.Run()
	}()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state := nativeActiveWindow
		if state != nil && state.hwnd != 0 && len(state.handles) == 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	state := nativeActiveWindow
	if state == nil || state.hwnd == 0 {
		t.Fatal("expected a native window handle")
	}
	if len(state.handles) != 5 {
		t.Fatalf("expected 5 native controls, got %d", len(state.handles))
	}

	procPostMessageW.Call(state.hwnd, wmClose, 0, 0)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("native window failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native window did not close")
	}
}

// TestNativeButtonInvokesVMClosure checks that a button callback compiled by
// the VM runs when the Win32 control sends BN_CLICKED.
func TestNativeButtonInvokesVMClosure(t *testing.T) {
	requireGUIE2E(t)

	src := `
clicks := 0;
func on_click():
    clicks = clicks + 1;
`
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile: %v", err)
	}
	machine := vm.NewWithEnv(c.Bytecode(), interpreter.New().Env())
	if err := machine.Run(); err != nil {
		t.Fatalf("run: %v", err)
	}

	globalIndex := func(name string) int {
		for i, n := range c.GlobalNames() {
			if n == name {
				return i
			}
		}
		return -1
	}
	clickIdx := globalIndex("clicks")
	fnIdx := globalIndex("on_click")
	if clickIdx < 0 || fnIdx < 0 {
		t.Fatalf("globals not found: clicks=%d on_click=%d", clickIdx, fnIdx)
	}
	fn, ok := machine.GlobalAt(fnIdx).(*vm.CompiledFunction)
	if !ok {
		t.Fatalf("expected *vm.CompiledFunction, got %T", machine.GlobalAt(fnIdx))
	}

	win := NewNativeWindow("Alca Callback Test", 360, 220)
	win.SetBackend("native")
	win.AddButton("Click", fn)

	done := make(chan error, 1)
	go func() { done <- win.Run() }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		state := nativeActiveWindow
		if state != nil && state.hwnd != 0 && len(state.handles) == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	state := nativeActiveWindow
	if state == nil || len(state.handles) != 1 {
		t.Fatal("native button was not created")
	}

	// wParam = MAKEWPARAM(controlID, BN_CLICKED), with the button first.
	procSendMessageW.Call(state.hwnd, wmCommand, uintptr(nativeControlIDBase), state.handles[0].hwnd)

	procPostMessageW.Call(state.hwnd, wmClose, 0, 0)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("native window failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native window did not close")
	}

	clicks := machine.GlobalAt(clickIdx)
	iv, ok := clicks.(*interpreter.IntegerValue)
	if !ok || iv.Val != 1 {
		t.Fatalf("callback did not run: clicks = %v", clicks)
	}
}
