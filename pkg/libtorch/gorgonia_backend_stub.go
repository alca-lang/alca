// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !gorgonia

package libtorch

// matMulGorgonia is a stub used when the "gorgonia" build tag is not enabled.
// It always reports that the gorgonia kernel is unavailable, so the naive
// implementation is used.
func matMulGorgonia(a, b *Tensor) (*Tensor, bool) {
	return nil, false
}
