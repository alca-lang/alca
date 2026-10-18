import time

start = time.perf_counter()
total = 0
for i in range(1_000_000):
    total += (i % 5) + 2
elapsed_ms = (time.perf_counter() - start) * 1000

print(f"Python 3.12 (for-in range 1M): total={total}, time={elapsed_ms:.2f} ms")
