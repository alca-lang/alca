import time

def compute(x, y):
    return (x * 3) - (y + 1)

start = time.perf_counter()
i = 0
total = 0
while i < 1_000_000:
    total += compute(i % 100, 5)
    i += 1
elapsed_ms = (time.perf_counter() - start) * 1000

print(f"Python 3.12 (1M function calls): total={total}, time={elapsed_ms:.2f} ms")
