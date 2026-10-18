use std.time

start = time.now_ms()
i = 0
total = 0
while i < 1000000:
    total = total + (i % 7) * 3
    i = i + 1
elapsed_ms = time.now_ms() - start

println("Alca VM (while loop): total=" + string(total) + ", time=" + string(elapsed_ms) + " ms")
