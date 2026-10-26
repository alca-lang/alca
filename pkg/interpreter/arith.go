// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package interpreter

// IntPow returns base**exp using exact integer arithmetic (exponentiation by
// squaring, wrapping on int64 overflow like the other integer operators).
// exp must be >= 0; callers handle negative exponents.
func IntPow(base, exp int64) int64 {
	result := int64(1)
	for exp > 0 {
		if exp&1 == 1 {
			result *= base
		}
		base *= base
		exp >>= 1
	}
	return result
}
