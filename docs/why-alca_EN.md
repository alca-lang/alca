# Why Alca? Comparing to Python, Go, and Deno

[English](why-alca_EN.md) | [Русский](why-alca_RU.md)

When evaluating a new programming language, engineers naturally ask: **"Why adopt another language when we already have mature ecosystems like Python, Go, and Deno/Node.js?"**

The answer lies in the engineering trade-offs existing languages force upon us:
- Choosing **Python** grants developer joy, readable syntax, and lightning-fast prototyping, but incurs severe interpreter runtime overhead, multi-threading bottlenecks caused by the Global Interpreter Lock (GIL), and dependency friction (`venv`, wheels, broken C extensions).
- Choosing **Go** provides outstanding compilation speed, single-binary distribution, and scalable concurrency, but burdens developers with repetitive boilerplate (hundreds of `if err != nil` checks), lack of expressive functional pipelines (`|>`), no list comprehensions, and rigid OOP semantics.
- Choosing **Deno** or **Node.js** brings the vast JavaScript/TypeScript ecosystem, but drags along the 50MB Chromium V8 engine, slow cold starts (~80–120ms), and massive baseline memory footprints (~50–70MB) even for trivial CLI scripts.

**Alca Lang** was engineered to synthesize the greatest strengths of these environments while systematically eliminating their friction points.

---

## 1. Feature & Architecture Comparison

| Metric | Alca Lang | Python 3.12 | Go 1.26 | Deno / Node.js 20 |
| :--- | :--- | :--- | :--- | :--- |
| **Syntax Style** | Elegant, indentation-based (Pythonic clarity) | Clean, indentation-based | Verbose, classic C-like | C-like braces and semicolons |
| **Type System** | Strict static + powerful type inference | Dynamic (type hints unchecked at runtime) | Strict static | Optional (TypeScript requires compile step) |
| **Concurrency** | Native OS threads with zero GIL | GIL (single thread executing bytecode) | Goroutines (CSP scheduler) | Single-threaded Event Loop |
| **Cold Startup** | **< 3.5 ms** | ~35–50 ms | < 5 ms | ~60–100 ms |
| **Baseline RSS Memory** | **~2.8 MB** | ~16–22 MB | ~4–6 MB | ~50–70 MB (V8 Engine) |
| **Distribution** | 1 standalone binary (zero dependencies) | Requires Python runtime, venv, wheels | 1 standalone binary | Requires Deno runtime or node_modules |
| **Data Pipelines** | Native pipeline operator `\|>` | None (nested function calls) | None (manual loops) | Method chaining (`.map().filter()`) |
| **Pattern Matching** | Full pattern matching with destructuring | Limited `match/case` | None (simple `switch` only) | None (`switch` only) |
| **Sandbox Security** | Built into VM opcode interpreter | None (unrestricted host access) | None (unrestricted host access) | CLI permission flags |
| **Package Manager** | Integrated APM (`alca.lock`, SHA-256) | pip (no native lockfile without poetry) | go modules (`go.mod`, `go.sum`) | deno vendor / npm |

---

## 2. Alca vs Python: Speed and Type Safety Without Sacrificing Beauty

If you love Python for its clean syntax, readable code, and absence of curly-bracket noise, Alca will feel immediately familiar:

```alca
# List comprehensions, slices, and f-strings
users = ["alice", "bob", "charlie", "david"];
active = [u.upper() for u in users if len(u) > 3];

println(f"Active users: {active}");
```

### Where Alca Outperforms Python:
1. **10x to 35x Faster Execution**: Alca uses a high-performance register-based bytecode VM and an integrated JIT compiler that generates native x86-64 machine code on the fly.
2. **Zero GIL**: Multi-threaded Alca programs scale linearly across all CPU cores without global interpreter locking.
3. **Compile-Time Safety**: Say goodbye to runtime `AttributeError` crashes in production. Alca's static type checker verifies your logic before execution, while type inference keeps declarations concise.
4. **Single-Binary Output**: Run `alca build app.al -o app.exe` to produce a completely self-contained executable that runs on any target machine with zero installation requirements.

---

## 3. Alca vs Go: Expressiveness Without Boilerplate

Go is widely admired for its operational simplicity and deployment model. Alca shares Go's commitment to reliability, but eliminates tedious boilerplate.

### Data Processing in Go:
```go
var result []string
for _, item := range data {
    if isValid(item) {
        val, err := transform(item)
        if err != nil {
            return nil, err
        }
        result = append(result, val)
    }
}
```

### The Same Logic in Alca via Pipeline Operators:
```alca
result := data 
    |> filter(isValid) 
    |> map(transform);
```

### Where Alca Outperforms Go:
1. **Modern Functional Primitives**: Native pipeline operators `|>`, pattern matching `match`, comprehensions, and tuple unpacking make code up to 3x more compact while enhancing readability.
2. **Ergonomic Error Handling**: Instead of repetitive `if err != nil` checks on every line, Alca offers concise error streams (`try`, `catch`, `or`, and safe navigation `?.`).
3. **Instant Iteration**: Run scripts directly via JIT during development (`alca run app.al`), then compile to a production-ready binary when done.

---

## 4. Alca vs Deno / Node.js: True Sandbox Without V8 Bloat

Deno popularized capability-based security permissions. However, dragging the Chromium V8 engine along makes Node and Deno impractical for lightweight system tools and low-memory microservices.

### Where Alca Outperforms Deno/Node:
1. **Instant Response (<3.5ms vs 80–120ms)**: An Alca utility completes execution and exits before V8 even finishes initializing its runtime.
2. **2.8MB vs 60MB Memory Footprint**: Alca services consume up to 25x less RAM than equivalent Node.js services, allowing hundreds of microservices to co-exist on a single small server.
3. **Core-Level Sandboxing**: Alca's capability model is enforced directly within the VM instruction dispatcher, guaranteeing total containment.

---

## 5. Ideal Use Cases for Alca

1. **High-Performance Microservices**: Ultra-low memory footprint, fast HTTP processing, and sub-millisecond startup.
2. **CLI Utilities & DevOps Tooling**: Single-binary deployment with instant cold start.
3. **Data Pipelines & Automation**: Pythonic brevity combined with native compiled execution speed.
4. **Secure Plugin Systems**: VM-level sandbox capabilities protect the host system from untrusted code.
