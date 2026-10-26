// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Alca interpreter and VM workload: run with `alca run benchmarks/main.al`.
// For elapsed time, measure the process externally, e.g. PowerShell
// `Measure-Command { alca run benchmarks/main.al }`.

func fib(n: int) -> int:
    if n < 2:
        return n;
    return fib(n - 1) + fib(n - 2);

println("recursive fibonacci(30): " + string(fib(30)));

iterations := 100000;
i := 0;
total := 0;
while i < iterations:
    total = total + i;
    i = i + 1;
println("integer loop (" + string(iterations) + "): " + string(total));

words := ["alca", "bytecode", "interpreter", "benchmark"];
joined := "";
i = 0;
while i < 1000:
    joined = joined + words[i % len(words)];
    i = i + 1;
println("string concatenation characters: " + string(len(joined)));

scores := {"vm": 1, "jit": 2, "interpreter": 3};
i = 0;
scoreTotal := 0;
while i < 100000:
    scoreTotal = scoreTotal + scores["vm"] + scores["jit"] + scores["interpreter"];
    i = i + 1;
println("map lookup total: " + string(scoreTotal));

values: list[int] = [];
i = 0;
while i < 10000:
    values = push(values, i);
    i = i + 1;
println("list size: " + string(len(values)));
    