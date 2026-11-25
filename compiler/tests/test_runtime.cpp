// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "microtest/microtest.h"
#include "runtime/runtime.h"

#include <string>
#include <vector>
#include <atomic>

using namespace ulang::runtime;

MT_TEST(runtime, mark_sweep_gc_lifecycle) {
    GarbageCollector& gc = GarbageCollector::instance();
    gc.reset();

    MT_CHECK_EQ(gc.objectCount(), 0u);
    MT_CHECK_EQ(gc.totalAllocated(), 0u);

    // 1. Allocate tracked objects
    void* obj1 = gc.allocate(64);
    void* obj2 = gc.allocate(128);
    MT_CHECK(obj1 != nullptr);
    MT_CHECK(obj2 != nullptr);
    MT_CHECK_EQ(gc.objectCount(), 2u);

    // 2. Protect obj1 as a root
    gc.addRoot(&obj1);

    // 3. Trigger collection: obj2 should be swept, obj1 retained
    gc.collect();
    MT_CHECK_EQ(gc.objectCount(), 1u);
    MT_CHECK_EQ(gc.totalAllocated(), 64u);

    // 4. Remove root and collect again
    gc.removeRoot(&obj1);
    obj1 = nullptr;
    gc.collect();
    MT_CHECK_EQ(gc.objectCount(), 0u);
    MT_CHECK_EQ(gc.totalAllocated(), 0u);
}

MT_TEST(runtime, async_green_threads_scheduler) {
    AsyncScheduler& scheduler = AsyncScheduler::instance();
    scheduler.reset();

    std::atomic<int> counter{0};

    // Spawn 10 concurrent fiber tasks
    for (int i = 0; i < 10; ++i) {
        scheduler.spawn([&counter]() {
            counter.fetch_add(1);
        });
    }

    // WaitAll blocks until all green threads complete
    scheduler.waitAll();

    MT_CHECK_EQ(counter.load(), 10);
}
