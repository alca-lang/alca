fn fib(n: int) -> int {
    if n <= 1 {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}

sum := 0
for_start := 1
while for_start <= 10 {
    sum = sum + for_start
    for_start = for_start + 1
}

println("hello from native Pawl")
println(fib(10))
println(sum)
println(-42)
exit(0)
