// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// Native Runtime:
// 1. Mark-Sweep Garbage Collector (Precise Heap Allocation & Object Tracking)
// 2. Async Fiber / Green Threads Engine (M:N Task Scheduler & Spawn / Join)

#pragma once

#include <cstdint>
#include <cstddef>
#include <vector>
#include <functional>
#include <memory>

namespace ulang::runtime {

// --- 1. Mark-Sweep Garbage Collector ---

struct ObjectHeader {
    size_t size;           // Size of user payload in bytes
    bool marked;           // Mark flag for GC
    ObjectHeader* next;    // Linked list of all allocated objects
};

class GarbageCollector {
public:
    static GarbageCollector& instance();

    // Allocate an object tracked by GC
    void* allocate(size_t size);

    // Register a pointer root (e.g. stack variable or global)
    void addRoot(void** rootPtr);
    void removeRoot(void** rootPtr);

    // Perform Mark-Sweep Garbage Collection cycle
    void collect();

    // Statistics
    size_t totalAllocated() const { return bytesAllocated_; }
    size_t objectCount() const { return objectCount_; }

    void reset();

private:
    GarbageCollector() = default;
    ~GarbageCollector();

    ObjectHeader* head_{nullptr};
    std::vector<void**> roots_;
    size_t bytesAllocated_{0};
    size_t objectCount_{0};
    size_t threshold_{1024 * 1024}; // 1 MB default collection trigger
};

// --- 2. Async / Fiber Engine (Green Threads) ---

enum class TaskStatus {
    Ready,
    Running,
    Finished
};

struct FiberTask {
    uint64_t id;
    std::function<void()> work;
    TaskStatus status{TaskStatus::Ready};
};

class AsyncScheduler {
public:
    static AsyncScheduler& instance();

    // Spawns a task as a green thread / fiber
    uint64_t spawn(std::function<void()> task);

    // Wait for all pending asynchronous tasks (equivalent to sync.WaitGroup / WaitAll)
    void waitAll();

    // Reset scheduler state
    void reset();

private:
    AsyncScheduler() = default;
    ~AsyncScheduler() = default;

    std::vector<std::shared_ptr<FiberTask>> tasks_;
    uint64_t nextId_{1};
};

// C-ABI wrappers callable by native compiled binaries or runtime glue
extern "C" {
    void* pawl_gc_alloc(size_t size);
    void  pawl_gc_collect();
    void  pawl_async_spawn(void (*func)(void*), void* arg);
    void  pawl_async_wait();
}

} // namespace ulang::runtime
