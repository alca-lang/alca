// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"testing"
)

func TestCaptureScreen(t *testing.T) {
	img, err := captureScreen(0, 0, 100, 100)
	if err != nil {
		t.Skipf("skipping screenshot test in background/headless session: %v", err)
	}
	if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 100 {
		t.Fatalf("unexpected size: %v", img.Bounds())
	}
}
