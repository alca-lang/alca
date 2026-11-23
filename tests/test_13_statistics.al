// Copyright 2026 Pawl Programming Language
// Test 13: Statistical operations - mean, median, variance, standard deviation.

use std.math

// 1. Mean (average) calculation
func mean(values: list[float]) -> float {
    if len(values) == 0 {
        return 0.0
    }
    total: float = 0.0
    for v in values {
        total = total + v
    }
    return total / float(len(values))
}
data1 := [1.0, 2.0, 3.0, 4.0, 5.0]
mean_val := mean(data1)
assert mean_val == 3.0

// 2. Median calculation
func median(values: list[float]) -> float {
    if len(values) == 0 {
        return 0.0
    // Simple sort simulation (bubble sort for clarity)
    }
    sorted: list[float] = []
    for v in values {
        sorted = sorted + [v]
    }
    i := 0
    while i < len(sorted) {
        j := i + 1
        while j < len(sorted) {
            if sorted[j] < sorted[i] {
                temp := sorted[i]
                sorted[i] = sorted[j]
                sorted[j] = temp
            }
            j = j + 1
        }
        i = i + 1
    }
    mid := len(sorted) / 2
    if len(sorted) % 2 == 0 {
        return (sorted[mid - 1] + sorted[mid]) / 2.0
    }
    else {
        return sorted[mid]
    }
}
data2 := [3.0, 1.0, 4.0, 2.0, 5.0]
median_val := median(data2)
assert median_val == 3.0

// Even number of elements
data3 := [1.0, 2.0, 3.0, 4.0]
median_even := median(data3)
assert median_even == 2.5

// 3. Variance calculation
func variance(values: list[float]) -> float {
    if len(values) == 0 {
        return 0.0
    }
    m := mean(values)
    sum_sq_diff: float = 0.0
    for v in values {
        diff := v - m
        sum_sq_diff = sum_sq_diff + (diff * diff)
    }
    return sum_sq_diff / float(len(values))
}
data4 := [2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0]
var_val := variance(data4)
assert var_val > 3.9 and var_val < 4.1 // ≈ 4.0

// 4. Standard deviation
func std_dev(values: list[float]) -> float {
    return math.sqrt(variance(values))
}
std_val := std_dev(data4)
assert std_val > 1.9 and std_val < 2.1 // ≈ 2.0

// 5. Range (min and max)
func range_vals(values: list[float]) -> list[float] {
    if len(values) == 0 {
        return [0.0, 0.0]
    }
    min_val := values[0]
    max_val := values[0]
    for v in values {
        if v < min_val {
            min_val = v
        }
        if v > max_val {
            max_val = v
        }
    }
    return [min_val, max_val]
}
data5 := [10.0, 50.0, 30.0, 70.0, 20.0]
r := range_vals(data5)
assert r[0] == 10.0
assert r[1] == 70.0

// 6. Sum of squares
func sum_of_squares(values: list[float]) -> float {
    total: float = 0.0
    for v in values {
        total = total + (v * v)
    }
    return total
}
data6 := [1.0, 2.0, 3.0]
sos := sum_of_squares(data6)
assert sos == 14.0 // 1+4+9

// 7. Product of values
func product(values: list[float]) -> float {
    if len(values) == 0 {
        return 0.0
    }
    result: float = 1.0
    for v in values {
        result = result * v
    }
    return result
}
data7 := [2.0, 3.0, 4.0]
prod := product(data7)
assert prod == 24.0

// 8. Mode (most frequent value)
func mode(values: list[int]) -> int {
    if len(values) == 0 {
        return 0
    }
    freq: map[int, int] = {}
    for v in values {
        if v in freq {
            freq[v] = freq[v] + 1
        }
        else {
            freq[v] = 1
        }
    }
    max_count := 0
    mode_val := values[0]
    for v in values {
        if freq[v] > max_count {
            max_count = freq[v]
            mode_val = v
        }
    }
    return mode_val
}
data8 := [1, 2, 2, 3, 3, 3, 4]
mode_val := mode(data8)
assert mode_val == 3

// 9. Percentile approximation (simplified)
func percentile(values: list[float], p: float) -> float {
    if len(values) == 0 {
        return 0.0
    }
    sorted: list[float] = []
    for v in values {
        sorted = sorted + [v]
    // Simple sort
    }
    i := 0
    while i < len(sorted) {
        j := i + 1
        while j < len(sorted) {
            if sorted[j] < sorted[i] {
                temp := sorted[i]
                sorted[i] = sorted[j]
                sorted[j] = temp
            }
            j = j + 1
        }
        i = i + 1
    }
    idx := int(float(len(sorted) - 1) * p / 100.0)
    return sorted[idx]
}
data9 := [1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0]
p50 := percentile(data9, 50.0) // Median
assert p50 == 5.0

p25 := percentile(data9, 25.0) // First quartile
assert p25 > 2.0 and p25 < 4.0

// 10. Correlation coefficient (simplified)
func correlation(x: list[float], y: list[float]) -> float {
    if len(x) != len(y) or len(x) == 0 {
        return 0.0
    }
    n := float(len(x))
    sum_x: float = 0.0
    sum_y: float = 0.0
    sum_xy: float = 0.0
    sum_x2: float = 0.0
    sum_y2: float = 0.0
    i := 0
    while i < len(x) {
        sum_x = sum_x + x[i]
        sum_y = sum_y + y[i]
        sum_xy = sum_xy + (x[i] * y[i])
        sum_x2 = sum_x2 + (x[i] * x[i])
        sum_y2 = sum_y2 + (y[i] * y[i])
        i = i + 1
    }
    numerator := (n * sum_xy) - (sum_x * sum_y)
    denominator := math.sqrt((n * sum_x2 - sum_x * sum_x) * (n * sum_y2 - sum_y * sum_y))
    if denominator == 0.0 {
        return 0.0
    }
    return numerator / denominator
}
x_vals := [1.0, 2.0, 3.0, 4.0, 5.0]
y_vals := [2.0, 4.0, 6.0, 8.0, 10.0] // Perfect positive correlation
corr := correlation(x_vals, y_vals)
assert corr > 0.99 and corr <= 1.0

println("✓ test_13_statistics passed")
