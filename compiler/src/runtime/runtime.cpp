// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "runtime/runtime.h"
#include <cstdlib>
#include <cstring>
#include <algorithm>
#include <thread>
#include <future>

namespace ulang::runtime {

// --- Garbage Collector Implementation ---

GarbageCollector& GarbageCollector::instance() {
    static GarbageCollector gc;
    return gc;
}

GarbageCollector::~GarbageCollector() {
    reset();
}

void* GarbageCollector::allocate(size_t size) {
    if (bytesAllocated_ + size >= threshold_) {
        collect();
    }

    size_t total = sizeof(ObjectHeader) + size;
    auto* raw = static_cast<uint8_t*>(std::malloc(total));
    if (!raw) return nullptr;

    auto* header = reinterpret_cast<ObjectHeader*>(raw);
    header->size = size;
    header->marked = false;
    header->next = head_;
    head_ = header;

    bytesAllocated_ += size;
    ++objectCount_;

    void* payload = raw + sizeof(ObjectHeader);
    std::memset(payload, 0, size);
    return payload;
}

void GarbageCollector::addRoot(void** rootPtr) {
    if (rootPtr) {
        roots_.push_back(rootPtr);
    }
}

void GarbageCollector::removeRoot(void** rootPtr) {
    auto it = std::find(roots_.begin(), roots_.end(), rootPtr);
    if (it != roots_.end()) {
        roots_.erase(it);
    }
}

void GarbageCollector::collect() {
    // 1. Mark phase
    for (void** rootPtr : roots_) {
        if (!rootPtr || !(*rootPtr)) continue;
        void* ptr = *rootPtr;

        // Find corresponding header
        for (ObjectHeader* cur = head_; cur != nullptr; cur = cur->next) {
            void* payload = reinterpret_cast<uint8_t*>(cur) + sizeof(ObjectHeader);
            if (payload == ptr) {
                cur->marked = true;
                break;
            }
        }
    }

    // 2. Sweep phase
    ObjectHeader** prev = &head_;
    ObjectHeader* cur = head_;
    while (cur) {
        if (!cur->marked) {
            // Unreached object: reclaim
            ObjectHeader* unreached = cur;
            *prev = cur->next;
            cur = cur->next;

            bytesAllocated_ -= unreached->size;
            --objectCount_;
            std::free(unreached);
        } else {
            // Clear mark for next cycle
            cur->marked = false;
            prev = &(cur->next);
            cur = cur->next;
        }
    }

    // Dynamic threshold expansion
    if (bytesAllocated_ * 2 > threshold_) {
        threshold_ = bytesAllocated_ * 2;
    }
}

void GarbageCollector::reset() {
    ObjectHeader* cur = head_;
    while (cur) {
        ObjectHeader* next = cur->next;
        std::free(cur);
        cur = next;
    }
    head_ = nullptr;
    roots_.clear();
    bytesAllocated_ = 0;
    objectCount_ = 0;
}

// --- Async Fiber / Green Threads Implementation ---

AsyncScheduler& AsyncScheduler::instance() {
    static AsyncScheduler scheduler;
    return scheduler;
}

uint64_t AsyncScheduler::spawn(std::function<void()> task) {
    auto fiber = std::make_shared<FiberTask>();
    fiber->id = nextId_++;
    fiber->work = std::move(task);
    fiber->status = TaskStatus::Ready;
    tasks_.push_back(fiber);
    return fiber->id;
}

void AsyncScheduler::waitAll() {
    std::vector<std::future<void>> futures;
    for (auto& task : tasks_) {
        if (task->status == TaskStatus::Ready) {
            task->status = TaskStatus::Running;
            futures.push_back(std::async(std::launch::async, [t = task]() {
                if (t->work) {
                    t->work();
                }
                t->status = TaskStatus::Finished;
            }));
        }
    }

    for (auto& f : futures) {
        f.get();
    }
    tasks_.clear();
}

void AsyncScheduler::reset() {
    tasks_.clear();
    nextId_ = 1;
}

// C-ABI functions
extern "C" {
    void* pawl_gc_alloc(size_t size) {
        return GarbageCollector::instance().allocate(size);
    }
    void pawl_gc_collect() {
        GarbageCollector::instance().collect();
    }
    void pawl_async_spawn(void (*func)(void*), void* arg) {
        AsyncScheduler::instance().spawn([=]() {
            if (func) func(arg);
        });
    }
    void pawl_async_wait() {
        AsyncScheduler::instance().waitAll();
    }
}

} // namespace ulang::runtime
