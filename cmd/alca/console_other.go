//go:build !windows

package main

// enableUTF8Console is a no-op outside Windows: terminals there are expected
// to handle UTF-8 natively.
func enableUTF8Console() {}
