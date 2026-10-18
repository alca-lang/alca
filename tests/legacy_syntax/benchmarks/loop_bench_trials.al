use std.time

trials = [1, 2, 3, 4, 5]
min_t = 999999
max_t = 0
total_t = 0
res_total = 0

for t in trials:
    start = time.now_ms()
    i = 0
    total = 0
    while i < 1000000:
        total = total + (i % 7) * 3
        i = i + 1
    dur = time.now_ms() - start
    total_t = total_t + dur
    res_total = total
    if dur < min_t:
        min_t = dur
    if dur > max_t:
        max_t = dur

avg_t = total_t / 5
println("Alca VM While Loop 1M (5 trials): min=" + string(min_t) + "ms, avg=" + string(avg_t) + "ms, max=" + string(max_t) + "ms, total=" + string(res_total))
