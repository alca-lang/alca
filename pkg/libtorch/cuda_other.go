// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package libtorch

import (
	"fmt"
)

func initCUDA() {
	cudaInitOnce.Do(func() {
		// Non-Windows platforms fallback cleanly unless CUDA runtime is linked
		cudaAvailable = false
		cudaDevCount = 0
	})
}

func cudaAlloc(bytes int) (uintptr, error) {
	return 0, fmt.Errorf("CUDA is not supported on this platform")
}

func cudaFree(ptr uintptr, bytes int) error {
	return nil
}

func cudaMemcpyHtoD(dst uintptr, src []float64) error {
	return fmt.Errorf("CUDA is not supported on this platform")
}

func cudaMemcpyDtoH(dst []float64, src uintptr) error {
	return fmt.Errorf("CUDA is not supported on this platform")
}

func cudaSync() error {
	return nil
}
