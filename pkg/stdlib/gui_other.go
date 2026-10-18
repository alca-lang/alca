//go:build !windows

package stdlib

import "github.com/alca-lang/alca/pkg/interpreter"

// guiMethods lists the std.gui API surface. On non-Windows platforms every
// method exists (so programs type-check and link everywhere) but returns a
// clear runtime error instead of crashing the build.
var guiMethods = []string{
	"alert", "msgbox", "confirm", "prompt", "open_file", "save_file", "notify",
	"screen_size", "width", "height", "screenshot", "webview", "window",
}

// RegisterGUIBuiltins registers std.gui; native dialogs are Windows-only.
func RegisterGUIBuiltins(env *interpreter.Environment) {
	guiModule := interpreter.NewMap()
	for _, name := range guiMethods {
		n := name
		fn := &interpreter.BuiltinFunction{Name: n, Fn: func(args ...interpreter.Value) interpreter.Value {
			return interpreter.NewError(501, "std.gui."+n+" is only supported on Windows")
		}}
		guiModule.Set(n, fn)
		env.Set("gui_"+n, fn)
	}
	env.Set("gui", guiModule)
	env.Set("std.gui", guiModule)
}
