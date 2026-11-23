// Copyright 2026 Pawl Programming Language
// Test 67: Sorting algorithms - bubble, quick, merge sort implementations.

// 1. Bubble sort
func bubble_sort(arr: list[int]) -> list[int] {
    result: list[int] = []
    for item in arr {
        result = result + [item]
    }
    n := len(result)
    i := 0
    while i < n {
        j := 0
        while j < n - i - 1 {
            if result[j] > result[j + 1] {
                temp := result[j]
                result[j] = result[j + 1]
                result[j + 1] = temp
            }
            j = j + 1
        }
        i = i + 1
    }
    return result
}
unsorted := [5, 2, 8, 1, 9]
sorted := bubble_sort(unsorted)
assert sorted == [1, 2, 5, 8, 9]

// 2. Selection sort
func selection_sort(arr: list[int]) -> list[int] {
    result: list[int] = []
    for item in arr {
        result = result + [item]
    }
    n := len(result)
    i := 0
    while i < n {
        min_idx := i
        j := i + 1
        while j < n {
            if result[j] < result[min_idx] {
                min_idx = j
            }
            j = j + 1
        }
        temp := result[i]
        result[i] = result[min_idx]
        result[min_idx] = temp
        i = i + 1
    }
    return result
}
sorted2 := selection_sort([3, 1, 4, 1, 5])
assert sorted2 == [1, 1, 3, 4, 5]

// 3. Insertion sort
func insertion_sort(arr: list[int]) -> list[int] {
    result: list[int] = []
    for item in arr {
        result = result + [item]
    }
    i := 1
    while i < len(result) {
        key := result[i]
        j := i - 1
        while j >= 0 and result[j] > key {
            result[j + 1] = result[j]
            j = j - 1
        }
        result[j + 1] = key
        i = i + 1
    }
    return result
}
sorted3 := insertion_sort([12, 11, 13, 5, 6])
assert sorted3 == [5, 6, 11, 12, 13]

// 4. Check if sorted
func is_sorted(arr: list[int]) -> bool {
    i := 0
    while i < len(arr) - 1 {
        if arr[i] > arr[i + 1] {
            return false
        }
        i = i + 1
    }
    return true
}
assert is_sorted([1, 2, 3, 4])
assert !is_sorted([1, 3, 2, 4])

// 5. Reverse list
func reverse_list(arr: list[int]) -> list[int] {
    result: list[int] = []
    i := len(arr) - 1
    while i >= 0 {
        result = result + [arr[i]]
        i = i - 1
    }
    return result
}
reversed := reverse_list([1, 2, 3, 4])
assert reversed == [4, 3, 2, 1]

// 6. Merge two sorted lists
func merge_sorted(a: list[int], b: list[int]) -> list[int] {
    result: list[int] = []
    i := 0
    j := 0
    while i < len(a) and j < len(b) {
        if a[i] < b[j] {
            result = result + [a[i]]
            i = i + 1
        }
        else {
            result = result + [b[j]]
            j = j + 1
        }
    }
    while i < len(a) {
        result = result + [a[i]]
        i = i + 1
    }
    while j < len(b) {
        result = result + [b[j]]
        j = j + 1
    }
    return result
}
merged := merge_sorted([1, 3, 5], [2, 4, 6])
assert merged == [1, 2, 3, 4, 5, 6]

// 7. Quick sort partition
func partition(arr: list[int], low: int, high: int) -> int {
    pivot := arr[high]
    i := low - 1
    j := low
    while j < high {
        if arr[j] < pivot {
            i = i + 1
            temp := arr[i]
            arr[i] = arr[j]
            arr[j] = temp
        }
        j = j + 1
    }
    temp := arr[i + 1]
    arr[i + 1] = arr[high]
    arr[high] = temp
    return i + 1

// 8. Quick sort
}
func quick_sort_helper(arr: list[int], low: int, high: int) -> list[int] {
    if low < high {
        pi := partition(arr, low, high)
        arr = quick_sort_helper(arr, low, pi - 1)
        arr = quick_sort_helper(arr, pi + 1, high)
    }
    return arr
}
func quick_sort(arr: list[int]) -> list[int] {
    result: list[int] = []
    for item in arr {
        result = result + [item]
    }
    if len(result) > 0 {
        result = quick_sort_helper(result, 0, len(result) - 1)
    }
    return result
}
quick_sorted := quick_sort([10, 7, 8, 9, 1, 5])
assert quick_sorted == [1, 5, 7, 8, 9, 10]

// 9. Count sort (for small range)
func count_sort(arr: list[int]) -> list[int] {
    if len(arr) == 0 {
        return []
    }
    max_val := arr[0]
    for val in arr {
        if val > max_val {
            max_val = val
        }
    }
    count: list[int] = []
    i := 0
    while i <= max_val {
        count = count + [0]
        i = i + 1
    }
    for val in arr {
        count[val] = count[val] + 1
    }
    result: list[int] = []
    i := 0
    while i <= max_val {
        j := 0
        while j < count[i] {
            result = result + [i]
            j = j + 1
        }
        i = i + 1
    }
    return result
}
count_sorted := count_sort([4, 2, 2, 8, 3, 3, 1])
assert count_sorted == [1, 2, 2, 3, 3, 4, 8]

// 10. Find min and max in one pass
func min_max(arr: list[int]) -> (int, int) {
    if len(arr) == 0 {
        return (0, 0)
    }
    min_val := arr[0]
    max_val := arr[0]
    for val in arr {
        if val < min_val {
            min_val = val
        }
        if val > max_val {
            max_val = val
        }
    }
    return (min_val, max_val)
}
(min_val, max_val) := min_max([5, 2, 8, 1, 9])
assert min_val == 1
assert max_val == 9

println("✓ test_67_sorting_algorithms passed")
