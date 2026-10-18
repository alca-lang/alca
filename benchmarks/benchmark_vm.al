// ==============================================================================
// Alca Lang: Benchmark (Tree-walking Interpreter vs Optimized ByteCode VM)
// Run with `alca run --vm benchmarks/benchmark_vm.al` and
// `alca run --interp benchmarks/benchmark_vm.al` to compare engines.
// ==============================================================================

use std.time;

println("Начинаем бенчмарк: 200,000 итераций цикла с вычислениями...");

start := time.now_ms();

i := 0;
sum := 0;
while i < 200000:
    sum = sum + (i % 7) * 3;
    i = i + 1;

elapsed := time.now_ms() - start;

println("Результат: sum = " + string(sum));
println("Затраченное время: " + string(elapsed) + " мс");
