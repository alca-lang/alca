// Copyright 2026 Pawl Programming Language
// Test 72: Dynamic programming - memoization and DP examples.

// 1. Memoized Fibonacci
memo: map[int, int] = {}

func fib_memo(n: int) -> int {
    if n in memo {
        return memo[n]
    }
    if n <= 1 {
        return n
    }
    result := fib_memo(n - 1) + fib_memo(n - 2)
    memo[n] = result
    return result
}
assert fib_memo(10) == 55
assert fib_memo(20) == 6765

// 2. Clear memo
func clear_memo() {
    i := 0
    keys: list[int] = []
    for key in memo {
        keys = keys + [key]
    }
    for key in keys {
        memo[key] = 0 // Simplified clear

// 3. Climb stairs (dynamic programming)
    }
}
func climb_stairs(n: int) -> int {
    if n <= 2 {
        return n
    }
    dp: list[int] = [1, 2]
    i := 2
    while i < n {
        dp = dp + [dp[i - 1] + dp[i - 2]]
        i = i + 1
    }
    return dp[n - 1]
}
assert climb_stairs(3) == 3
assert climb_stairs(5) == 8

// 4. Coin change (minimum coins)
func min_coins(coins: list[int], amount: int) -> int {
    if amount == 0 {
        return 0
    }
    dp: list[int] = []
    i := 0
    while i <= amount {
        dp = dp + [amount + 1]
        i = i + 1
    }
    dp[0] = 0
    i := 1
    while i <= amount {
        for coin in coins {
            if coin <= i {
                if dp[i - coin] + 1 < dp[i] {
                    dp[i] = dp[i - coin] + 1
                }
            }
        }
        i = i + 1
    }
    if dp[amount] > amount {
        return -1
    }
    return dp[amount]
}
assert min_coins([1, 2, 5], 11) == 3

// 5. Longest increasing subsequence
func lis_length(arr: list[int]) -> int {
    if len(arr) == 0 {
        return 0
    }
    dp: list[int] = []
    i := 0
    while i < len(arr) {
        dp = dp + [1]
        i = i + 1
    }
    i := 1
    while i < len(arr) {
        j := 0
        while j < i {
            if arr[j] < arr[i] and dp[j] + 1 > dp[i] {
                dp[i] = dp[j] + 1
            }
            j = j + 1
        }
        i = i + 1
    }
    max_len := dp[0]
    for val in dp {
        if val > max_len {
            max_len = val
        }
    }
    return max_len
}
assert lis_length([10, 9, 2, 5, 3, 7, 101, 18]) == 4

// 6. Knapsack (0/1)
func knapsack(weights: list[int], values: list[int], capacity: int) -> int {
    n := len(weights)
    dp: list[list[int]] = []
    i := 0
    while i <= capacity {
        row: list[int] = []
        j := 0
        while j <= n {
            row = row + [0]
            j = j + 1
        }
        dp = dp + [row]
        i = i + 1
    }
    i := 1
    while i <= n {
        w := 0
        while w <= capacity {
            if weights[i - 1] <= w {
                dp[w][i] = dp[w][i - 1] > dp[w - weights[i - 1]][i - 1] + values[i - 1] ?
                              dp[w][i - 1] : dp[w - weights[i - 1]][i - 1] + values[i - 1]
            }
            else {
                dp[w][i] = dp[w][i - 1]
            }
            w = w + 1
        }
        i = i + 1
    }
    return dp[capacity][n]
}
assert knapsack([1, 3, 4, 5], [1, 4, 5, 7], 7) == 9

// 7. Edit distance
func edit_distance(s1: string, s2: string) -> int {
    m := len(s1)
    n := len(s2)
    dp: list[list[int]] = []
    i := 0
    while i <= m {
        row: list[int] = []
        j := 0
        while j <= n {
            row = row + [0]
            j = j + 1
        }
        dp = dp + [row]
        i = i + 1
    }
    i := 0
    while i <= m {
        dp[i][0] = i
        i = i + 1
    }
    j := 0
    while j <= n {
        dp[0][j] = j
        j = j + 1
    }
    i := 1
    while i <= m {
        j := 1
        while j <= n {
            if string.substring(s1, i - 1, i) == string.substring(s2, j - 1, j) {
                dp[i][j] = dp[i - 1][j - 1]
            }
            else {
                min_val := dp[i - 1][j]
                if dp[i][j - 1] < min_val {
                    min_val = dp[i][j - 1]
                }
                if dp[i - 1][j - 1] < min_val {
                    min_val = dp[i - 1][j - 1]
                }
                dp[i][j] = min_val + 1
            }
            j = j + 1
        }
        i = i + 1
    }
    return dp[m][n]
}
assert edit_distance("kitten", "sitting") == 3

// 8. Maximum subarray sum (Kadane's algorithm)
func max_subarray(arr: list[int]) -> int {
    if len(arr) == 0 {
        return 0
    }
    max_sum := arr[0]
    current_sum := arr[0]
    i := 1
    while i < len(arr) {
        current_sum = current_sum + arr[i]
        if arr[i] > current_sum {
            current_sum = arr[i]
        }
        if current_sum > max_sum {
            max_sum = current_sum
        }
        i = i + 1
    }
    return max_sum
}
assert max_subarray([-2, 1, -3, 4, -1, 2, 1, -5, 4]) == 6

// 9. Palindrome partitioning count
func palindrome_partitions(s: string) -> int {
    n := len(s)
    dp: list[list[bool]] = []
    i := 0
    while i < n {
        row: list[bool] = []
        j := 0
        while j < n {
            row = row + [false]
            j = j + 1
        }
        dp = dp + [row]
        i = i + 1
    }
    count := 0
    i := 0
    while i < n {
        j := 0
        while j <= i {
            if string.substring(s, j, j + 1) == string.substring(s, i, i + 1) {
                dp[j][i] = true
            }
            else {
                if i - j >= 2 and dp[j + 1][i - 1] {
                    dp[j][i] = true
                }
            }
            if dp[j][i] {
                count = count + 1
            }
            j = j + 1
        }
        i = i + 1
    }
    return count
}
assert palindrome_partitions("aab") == 4

// 10. Unique paths (grid)
func unique_paths(m: int, n: int) -> int {
    dp: list[list[int]] = []
    i := 0
    while i < m {
        row: list[int] = []
        j := 0
        while j < n {
            row = row + [1]
            j = j + 1
        }
        dp = dp + [row]
        i = i + 1
    }
    i := 1
    while i < m {
        j := 1
        while j < n {
            dp[i][j] = dp[i - 1][j] + dp[i][j - 1]
            j = j + 1
        }
        i = i + 1
    }
    return dp[m - 1][n - 1]
}
assert unique_paths(3, 7) == 28

println("✓ test_72_dynamic_programming passed")
