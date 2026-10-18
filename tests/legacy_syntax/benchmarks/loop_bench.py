import time

start = time.perf_counter()
i = 0
total = 0
while i < 1_000_000:
    total += (i % 7) * 3
    i += 1
elapsed_ms = (time.perf_counter() - start) * 1000

print(f"Python 3.12 (while loop): total={total}, time={elapsed_ms:.2f} ms")
