use std.time;

start := time.now_ms();
total := 0;
for i in 0..1000000:
    total = total + (i % 5) + 2;
elapsed_ms := time.now_ms() - start;

println("Alca VM (for-in 1M): total=" + string(total) + ", time=" + string(elapsed_ms) + " ms");
