//go:build !windows

package stdlib

import "fmt"

// runNativeWindows is only implemented on Windows; other platforms fall back
// to the Tcl/Tk or Qt backends.
func (w *NativeWindow) runNativeWindows() error {
	return fmt.Errorf("the native GUI backend is only available on Windows")
}
