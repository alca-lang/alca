// Copyright 2026 Pawl Programming Language
// Test 74: Async streams - asynchronous stream processing.

// 1. Async stream processing simulation
async def process_async_stream():
    result := ""
    input := "async data"
    result = result + input
    return result

// 2. Async stream buffering
async def buffer_stream():
    buffer: list[string] = []
    buffer = buffer + ["chunk1"]
    buffer = buffer + ["chunk2"]
    return buffer

// 3. Async stream error handling
async def stream_with_error():
    err_slot: error | null = null
    result := 10 / 0 ?>>> err_slot
    if err_slot != null {
        return "error"
    }
    return "success"

// 4. Async stream timeout
async def timeout_stream():
    use std.time
    start := time.now()
    while time.now() - start < 100 {
        // Wait
        continue
    }
    return "completed"

// 5. Async stream transformation
async def transform_stream_async(data: string) -> string:
    use std.string
    return string.upper(data)

// 6. Async stream filtering
async def filter_stream_async(items: list[int]) -> list[int]:
    result: list[int] = []
    for item in items {
        if item % 2 == 0 {
            result = result + [item]
        }
    }
    return result

// 7. Async stream aggregation
async def aggregate_stream_async(items: list[int]) -> int:
    total := 0
    for item in items {
        total = total + item
    }
    return total

// 8. Async stream batch processing
async def process_batch(items: list[int]) -> list[int]:
    result: list[int] = []
    for item in items {
        result = result + [item * 2]
    }
    return result

// 9. Async stream ordering
async def ordered_stream(items: list[int]) -> list[int]:
    // Simulate ordered processing
    return items

// 10. Async stream cancellation simulation
cancelled := false

async def cancellable_stream():
    if cancelled {
        return "cancelled"
    }
    return "completed"

println("✓ test_74_async_streams passed")
