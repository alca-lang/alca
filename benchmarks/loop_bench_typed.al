use std.time;

start: int = time.now_ms();
i: int = 0;
total: int = 0;
while i < 1000000:
    total = total + (i % 7) * 3;
    i = i + 1;
elapsed_ms: int = time.now_ms() - start;

println("Alca VM (typed while loop): total=" + string(total) + ", time=" + string(elapsed_ms) + " ms");
