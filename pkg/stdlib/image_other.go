// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import (
	"fmt"
	"image"
)

func captureScreen(x, y, w, h int) (*image.RGBA, error) {
	return nil, fmt.Errorf("screenshot capture is natively supported on Windows")
}
