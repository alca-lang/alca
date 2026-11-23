// Copyright 2026 Pawl Programming Language
// Test 14: Complex collections - nested maps/lists, deep operations, and data structures.

// 1. Nested list operations
matrix: list[list[int]] = [
    [1, 2, 3],
    [4, 5, 6],
    [7, 8, 9]
]

// Access nested elements
assert matrix[0][0] == 1
assert matrix[1][1] == 5
assert matrix[2][2] == 9

// 2. Nested map operations
users: map[string, map[string, string]] = {
    "user1": {"name": "Alice", "role": "admin"},
    "user2": {"name": "Bob", "role": "editor"}
}

assert users["user1"]["name"] == "Alice"
assert users["user2"]["role"] == "editor"

// 3. List of maps
employees: list[map[string, int]] = [
    {"id": 1, "salary": 50000},
    {"id": 2, "salary": 60000},
    {"id": 3, "salary": 70000}
]

assert employees[0]["id"] == 1
assert employees[2]["salary"] == 70000

// 4. Map of lists
departments: map[string, list[string]] = {
    "engineering": ["Alice", "Bob", "Charlie"],
    "marketing": ["David", "Eve"]
}

assert len(departments["engineering"]) == 3
assert departments["marketing"][0] == "David"

// 5. Deep copy simulation for lists
func deep_copy_list(src: list[int]) -> list[int] {
    dst: list[int] = []
    for item in src {
        dst = dst + [item]
    }
    return dst
}
original := [1, 2, 3, 4, 5]
copied := deep_copy_list(original)
assert copied == original
copied[0] = 99
assert original[0] == 1 // Original unchanged

// 6. Deep copy simulation for maps
func deep_copy_map(src: map[string, int]) -> map[string, int] {
    dst: map[string, int] = {}
    for key in src {
        dst[key] = src[key]
    }
    return dst
}
original_map := {"a": 1, "b": 2, "c": 3}
copied_map := deep_copy_map(original_map)
assert copied_map["a"] == 1
copied_map["a"] = 99
assert original_map["a"] == 1 // Original unchanged

// 7. Matrix operations - sum all elements
func matrix_sum(m: list[list[int]]) -> int {
    total: int = 0
    for row in m {
        for val in row {
            total = total + val
        }
    }
    return total
}
matrix_sum_val := matrix_sum(matrix)
assert matrix_sum_val == 45 // 1+2+3+4+5+6+7+8+9

// 8. Matrix transpose
func transpose(m: list[list[int]]) -> list[list[int]] {
    if len(m) == 0 {
        return []
    }
    rows := len(m)
    cols := len(m[0])
    result: list[list[int]] = []
    i := 0
    while i < cols {
        new_row: list[int] = []
        j := 0
        while j < rows {
            new_row = new_row + [m[j][i]]
            j = j + 1
        }
        result = result + [new_row]
        i = i + 1
    }
    return result
}
transposed := transpose(matrix)
assert transposed[0][0] == 1
assert transposed[0][1] == 4
assert transposed[2][2] == 9

// 9. Flatten nested list
func flatten(nested: list[list[int]]) -> list[int] {
    result: list[int] = []
    for inner in nested {
        for val in inner {
            result = result + [val]
        }
    }
    return result
}
nested_list := [[1, 2], [3, 4, 5], [6]]
flattened := flatten(nested_list)
assert len(flattened) == 6
assert flattened[0] == 1
assert flattened[5] == 6

// 10. Filter nested structures
func filter_by_salary(employees: list[map[string, int]], min_salary: int) -> list[map[string, int]] {
    result: list[map[string, int]] = []
    for emp in employees {
        if emp["salary"] >= min_salary {
            result = result + [emp]
        }
    }
    return result
}
high_earners := filter_by_salary(employees, 60000)
assert len(high_earners) == 2
assert high_earners[0]["id"] == 2
assert high_earners[1]["id"] == 3

// 11. Group by operation
func group_by_role(users: list[map[string, string]]) -> map[string, list[string]] {
    result: map[string, list[string]] = {}
    for user in users {
        role := user["role"]
        name := user["name"]
        if role in result {
            result[role] = result[role] + [name]
        }
        else {
            result[role] = [name]
        }
    }
    return result
}
user_list := [
    {"name": "Alice", "role": "admin"},
    {"name": "Bob", "role": "editor"},
    {"name": "Charlie", "role": "admin"}
]

grouped := group_by_role(user_list)
assert len(grouped["admin"]) == 2
assert len(grouped["editor"]) == 1

// 12. Merge nested maps
func merge_maps(a: map[string, int], b: map[string, int]) -> map[string, int] {
    result: map[string, int] = {}
    for key in a {
        result[key] = a[key]
    }
    for key in b {
        result[key] = b[key]
    }
    return result
}
map1 := {"a": 1, "b": 2}
map2 := {"c": 3, "d": 4}
merged := merge_maps(map1, map2)
assert len(merged) == 4
assert merged["a"] == 1
assert merged["d"] == 4

// 13. List comprehension simulation (filter + transform)
func transform_filter(nums: list[int]) -> list[int] {
    result: list[int] = []
    for n in nums {
        if n % 2 == 0 {
            result = result + [n * n]
        }
    }
    return result
}
numbers := [1, 2, 3, 4, 5, 6]
squared_evens := transform_filter(numbers)
assert squared_evens == [4, 16, 36]

// 14. Deep search in nested structure
func find_in_nested(data: list[list[int]], target: int) -> bool {
    for inner in data {
        for val in inner {
            if val == target {
                return true
            }
        }
    }
    return false
}
assert find_in_nested(matrix, 5)
assert !find_in_nested(matrix, 100)

// 15. Tree-like structure (list of nodes with children)
struct Node {
    value: int
    children: list[Node] = []
}
leaf1 := Node(1, [])
leaf2 := Node(2, [])
parent := Node(0, [leaf1, leaf2])

assert len(parent.children) == 2
assert parent.children[0].value == 1

println("✓ test_14_complex_collections passed")
