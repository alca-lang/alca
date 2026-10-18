use std.time;

func compute(x: int, y: int) -> int:
    return (x * 3) - (y + 1);

start := time.now_ms();
i := 0;
total := 0;
while i < 1000000:
    total = total + compute(i % 100, 5);
    i = i + 1;
elapsed_ms := time.now_ms() - start;

println("Alca VM (1M function calls): total=" + string(total) + ", time=" + string(elapsed_ms) + " ms");
