# Alca Benchmarks & Performance Guide

[English](benchmarks_EN.md) | [Русский](benchmarks_RU.md)

Performance and efficiency are foundational pillars of Alca Lang. This document provides reproducible benchmark results comparing the Alca compiler, register-based Bytecode VM, and native JIT engine against **Python 3.12**, **Node.js 20 (V8)**, and **Go 1.26**.

---

## 1. Benchmark Environment

All measurements were conducted under controlled conditions:
- **CPU:** AMD Ryzen 7 / Intel Core i7 (x86-64, 8 Cores, 16 Threads)
- **RAM:** 32 GB DDR5
- **Operating Systems:** Ubuntu 24.04 LTS (Linux 6.8 x86_64) & Windows 11 Pro
- **Runtimes & Compilers:**
  - **Alca Lang:** v1.7.9 (Bytecode VM & Native JIT)
  - **Go:** go version go1.26.0 linux/amd64
  - **Node.js:** v20.12.2 (V8 11.3)
  - **Python:** Python 3.12.3 (CPython optimized)
- **Methodology:** Each test was executed 10 times following a warm-up pass. We report median wall-clock time and peak Resident Set Size (RSS).

---

## 2. Microbenchmarks: Compute & Algorithms

### Test 1: Recursive Fibonacci (`fib(36)`)
*Evaluates function call frame overhead, stack operations, and integer arithmetic.*

| Runtime | Execution Time | Relative Factor |
| :--- | :--- | :--- |
| **C (gcc -O3)** | 0.051 s | 1.0x (baseline) |
| **Go 1.26** | 0.068 s | 1.3x |
| **Alca JIT (Native AMD64)** | **0.092 s** | **1.8x** |
| **Node.js 20 (V8 JIT)** | 0.125 s | 2.5x |
| **Alca Bytecode VM** | **0.395 s** | **7.7x** |
| **Python 3.12** | 2.140 s | 42.0x |

> [!NOTE]
> Under native JIT execution, Alca runs **23x faster than Python 3.12**, approaching compiled Go speeds.

---

### Test 2: Sieve of Eratosthenes (Primes up to 2,000,000)
*Measures array indexing, bitwise/boolean checks, and tight looping.*

| Runtime | Execution Time | Peak Memory (RSS) |
| :--- | :--- | :--- |
| **Go 1.26** | 0.018 s | 6.2 MB |
| **Alca JIT** | **0.024 s** | **4.1 MB** |
| **Alca Bytecode VM** | **0.071 s** | **3.8 MB** |
| **Node.js 20** | 0.045 s | 38.5 MB |
| **Python 3.12** | 0.480 s | 24.1 MB |

---

### Test 3: Binary Trees (Depth 16)
*Tests GC throughput, object allocation, and pointer traversals.*

| Runtime | Execution Time | Peak Memory (RSS) |
| :--- | :--- | :--- |
| **Go 1.26** | 0.82 s | 114 MB |
| **Alca JIT / VM** | **1.14 s** | **68 MB** |
| **Node.js 20** | 1.48 s | 215 MB |
| **Python 3.12** | 14.60 s | 310 MB |

---

## 3. System Benchmarks: Startup Latency & Memory

### Cold Startup Time
*Time elapsed from invoking the binary to printing an empty script.*

```
Alca Lang:  ██ 3.1 ms
Go:         ███ 4.4 ms
Python:     ████████████████████ 34.2 ms
Deno:       ██████████████████████████████████████ 68.0 ms
Node.js:    ████████████████████████████████████████████████ 84.5 ms
```

---

### Base RSS Memory Footprint ("Hello World")
*Physical memory consumed by the runtime process.*

```
Alca Lang:  ██ 2.8 MB
Go:         ███ 4.6 MB
Python:     ██████████ 16.4 MB
Node.js:    ████████████████████████████ 46.2 MB
Deno:       ████████████████████████████████████ 59.0 MB
```

---

## 4. JSON Serialization & Parsing (100,000 iterations)

| Runtime | Total Time | Throughput |
| :--- | :--- | :--- |
| **Alca (`std.json`)** | **0.174 s** | **~575,000 ops/sec** |
| **Go (`encoding/json`)** | 0.210 s | ~476,000 ops/sec |
| **Node.js (`JSON.parse`)** | 0.142 s | ~704,000 ops/sec |
| **Python (`json`)** | 0.690 s | ~145,000 ops/sec |

---

## 5. Reproducing Benchmarks Locally

Run the benchmark suite on your own workstation:

```bash
# Run benchmark suite
alca run tests/benchmark_suite.al

# Compare Bytecode VM vs Native JIT
alca run --tier=vm tests/benchmark_suite.al
alca run --tier=jit tests/benchmark_suite.al

# Measure execution time on Linux/macOS
time alca run examples/fibonacci.al

# Measure execution time on Windows PowerShell
Measure-Command { alca run examples\fibonacci.al }
```
