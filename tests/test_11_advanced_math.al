// Copyright 2026 Pawl Programming Language
// Test 11: Advanced math operations - trigonometry, logarithms, sqrt, pow, and constants.

use std.math

// 1. Trigonometric functions
angle: float = 45.0
rad := angle * math.PI / 180.0 // Convert to radians

sin_val := math.sin(rad)
cos_val := math.cos(rad)
tan_val := math.tan(rad)

// Verify sin(45°) ≈ 0.7071
assert sin_val > 0.70 and sin_val < 0.71
// Verify cos(45°) ≈ 0.7071
assert cos_val > 0.70 and cos_val < 0.71
// Verify tan(45°) = 1.0
assert tan_val > 0.99 and tan_val < 1.01

// 2. Inverse trigonometric functions
asin_val := math.asin(0.5)
acos_val := math.acos(0.5)
atan_val := math.atan(1.0)

// asin(0.5) = 30° in radians ≈ 0.5236
assert asin_val > 0.52 and asin_val < 0.53
// acos(0.5) = 60° in radians ≈ 1.0472
assert acos_val > 1.04 and acos_val < 1.05
// atan(1.0) = 45° in radians ≈ 0.7854
assert atan_val > 0.78 and atan_val < 0.79

// 3. Logarithmic functions
ln_val := math.log(10.0) // Natural log
log10_val := math.log10(100.0) // Base-10 log

// ln(10) ≈ 2.3026
assert ln_val > 2.30 and ln_val < 2.31
// log10(100) = 2.0
assert log10_val == 2.0

// 4. Power and sqrt functions
pow_val := math.pow(2.0, 10.0) // 2^10
sqrt_val := math.sqrt(16.0) // √16

// 2^10 = 1024
assert pow_val == 1024.0
// √16 = 4
assert sqrt_val == 4.0

// 5. Hypotenuse calculation
hypot_val := math.hypot(3.0, 4.0)
assert hypot_val == 5.0 // 3-4-5 triangle

// 6. Math constants
assert math.PI > 3.14 and math.PI < 3.15
assert math.E > 2.71 and math.E < 2.72

// 7. Rounding functions
floor_val := math.floor(3.7)
ceil_val := math.ceil(3.2)
round_val := math.round(3.5)

assert floor_val == 3.0
assert ceil_val == 4.0
assert round_val == 4.0

// 8. Absolute value
abs_int := math.abs(-42)
abs_float := math.abs(-3.14)

assert abs_int == 42
assert abs_float == 3.14

// 9. Min and Max
min_val := math.min(10, 5)
max_val := math.max(10, 5)

assert min_val == 5
assert max_val == 10

// 10. Clamp function
clamped := math.clamp(15, 0, 10)
assert clamped == 10

clamped_low := math.clamp(-5, 0, 10)
assert clamped_low == 0

clamped_mid := math.clamp(5, 0, 10)
assert clamped_mid == 5

// 11. Degree/Radian conversion
deg := 90.0
rad2 := math.deg2rad(deg)
assert rad2 > 1.57 and rad2 < 1.58 // π/2 ≈ 1.5708

back_deg := math.rad2deg(rad2)
assert back_deg == 90.0

println("✓ test_11_advanced_math passed")
