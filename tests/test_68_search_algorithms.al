// Copyright 2026 Pawl Programming Language
// Test 68: Search algorithms - binary search, linear search.

// 1. Linear search
func linear_search(arr: list[int], target: int) -> int {
    i := 0
    while i < len(arr) {
        if arr[i] == target {
            return i
        }
        i = i + 1
    }
    return -1
}
arr := [5, 3, 8, 1, 9]
assert linear_search(arr, 8) == 2
assert linear_search(arr, 10) == -1

// 2. Binary search (requires sorted array)
func binary_search(arr: list[int], target: int) -> int {
    low := 0
    high := len(arr) - 1
    while low <= high {
        mid := (low + high) / 2
        if arr[mid] == target {
            return mid
        }
        if arr[mid] < target {
            low = mid + 1
        }
        else {
            high = mid - 1
        }
    }
    return -1
}
sorted_arr := [1, 3, 5, 7, 9, 11]
assert binary_search(sorted_arr, 7) == 3
assert binary_search(sorted_arr, 4) == -1

// 3. Find all occurrences
func find_all(arr: list[int], target: int) -> list[int] {
    result: list[int] = []
    i := 0
    while i < len(arr) {
        if arr[i] == target {
            result = result + [i]
        }
        i = i + 1
    }
    return result
}
multi := [1, 2, 3, 2, 4, 2]
indices := find_all(multi, 2)
assert indices == [1, 3, 5]

// 4. Find minimum
func find_min(arr: list[int]) -> int {
    if len(arr) == 0 {
        return 0
    }
    min_val := arr[0]
    for val in arr {
        if val < min_val {
            min_val = val
        }
    }
    return min_val
}
assert find_min([5, 2, 8, 1, 9]) == 1

// 5. Find maximum
func find_max(arr: list[int]) -> int {
    if len(arr) == 0 {
        return 0
    }
    max_val := arr[0]
    for val in arr {
        if val > max_val {
            max_val = val
        }
    }
    return max_val
}
assert find_max([5, 2, 8, 1, 9]) == 9

// 6. Find first match from end
func find_last(arr: list[int], target: int) -> int {
    i := len(arr) - 1
    while i >= 0 {
        if arr[i] == target {
            return i
        }
        i = i - 1
    }
    return -1
}
assert find_last([1, 2, 3, 2, 4], 2) == 3

// 7. Contains check
func contains(arr: list[int], target: int) -> bool {
    return linear_search(arr, target) != -1
}
assert contains([1, 2, 3], 2)
assert !contains([1, 2, 3], 5)

// 8. Find range of values
func find_range(arr: list[int], min_val: int, max_val: int) -> list[int] {
    result: list[int] = []
    for val in arr {
        if val >= min_val and val <= max_val {
            result = result + [val]
        }
    }
    return result
}
range_vals := find_range([1, 5, 10, 15, 20], 5, 15)
assert range_vals == [5, 10, 15]

// 9. Find closest value
func find_closest(arr: list[int], target: int) -> int {
    if len(arr) == 0 {
        return 0
    }
    closest := arr[0]
    min_diff := arr[0] - target
    if min_diff < 0 {
        min_diff = -min_diff
    }
    for val in arr {
        diff := val - target
        if diff < 0 {
            diff = -diff
        }
        if diff < min_diff {
            min_diff = diff
            closest = val
        }
    }
    return closest
}
assert find_closest([1, 4, 6, 8], 5) == 4 or find_closest([1, 4, 6, 8], 5) == 6

// 10. Count occurrences
func count_occurrences(arr: list[int], target: int) -> int {
    count := 0
    for val in arr {
        if val == target {
            count = count + 1
        }
    }
    return count
}
assert count_occurrences([1, 2, 2, 3, 2], 2) == 3

println("✓ test_68_search_algorithms passed")
