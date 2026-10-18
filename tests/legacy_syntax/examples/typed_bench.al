use std.time

int start = time.now_ms()
int i = 0
int total = 0
while i < 1000000:
    total = total + (i % 7) * 3
    i = i + 1
int elapsed_ms = time.now_ms() - start

println("Alca Typed Loop: total=" + string(total) + ", time=" + string(elapsed_ms) + " ms")
