// Copyright 2026 Pawl Programming Language
// Test 34: Shared state - synchronization, race conditions, and concurrent access.

// 1. Shared counter simulation
shared_counter := 0

func increment_counter() {
    shared_counter = shared_counter + 1
}
increment_counter()
increment_counter()
increment_counter()
assert shared_counter == 3

// 2. Shared data structure
shared_list: list[int] = []

func add_to_list(value: int) {
    shared_list = shared_list + [value]
}
add_to_list(1)
add_to_list(2)
add_to_list(3)
assert len(shared_list) == 3

// 3. Shared map
shared_map: map[string, int] = {}

func update_map(key: string, value: int) {
    shared_map[key] = value
}
update_map("a", 1)
update_map("b", 2)
assert shared_map["a"] == 1
assert shared_map["b"] == 2

// 4. Atomic operation simulation
func atomic_add(current: int, delta: int) -> int {
    return current + delta
}
atomic_value := 10
atomic_value = atomic_add(atomic_value, 5)
assert atomic_value == 15

// 5. Read-write lock simulation
struct Lock {
    locked: bool = false
    readers: int = 0
}
my_lock := Lock(false, 0)
assert !my_lock.locked

// 6. Shared configuration
config := {"debug": true, "port": 8080}

func get_config(key: string) -> int {
    if key in config {
        return config[key]
    }
    return 0
}
port := get_config("port")
assert port == 8080

// 7. Shared cache simulation
cache: map[string, string] = {}

func cache_get(key: string) -> string {
    if key in cache {
        return cache[key]
    }
    return ""
}
func cache_set(key: string, value: string) {
    cache[key] = value
}
cache_set("user1", "Alice")
assert cache_get("user1") == "Alice"

// 8. Shared state with struct
struct SharedState {
    counter: int = 0
    data: list[int] = []
}
state := SharedState(0, [])
state.counter = state.counter + 1
state.data = state.data + [42]
assert state.counter == 1
assert state.data[0] == 42

// 9. Global variable simulation
global_value := 100

func modify_global() {
    global_value = global_value * 2
}
modify_global()
assert global_value == 200

// 10. Shared resource access pattern
resource_available := true

func acquire_resource() -> bool {
    if resource_available {
        resource_available = false
        return true
    }
    return false
}
func release_resource() {
    resource_available = true
}
acquired := acquire_resource()
assert acquired
release_resource()
assert resource_available

// 11. Producer-consumer simulation
queue: list[int] = []

func producer() {
    queue = queue + [1]
    queue = queue + [2]
    queue = queue + [3]
}
func consumer() -> int {
    if len(queue) > 0 {
        item := queue[0]
        queue = queue[1..]
        return item
    }
    return 0
}
producer()
item1 := consumer()
item2 := consumer()
assert item1 == 1
assert item2 == 2

// 12. Shared state validation
func validate_state(state: map[string, int]) -> bool {
    if "count" in state {
        return state["count"] >= 0
    }
    return false
}
test_state := {"count": 5}
assert validate_state(test_state)

// 13. State transition
enum State {
    Idle
    Running
    Completed
}
current_state := State.Idle
assert current_state == State.Idle

current_state = State.Running
assert current_state == State.Running

// 14. Shared buffer
buffer: list[int] = []
buffer_size := 10

func buffer_write(value: int) -> bool {
    if len(buffer) < buffer_size {
        buffer = buffer + [value]
        return true
    }
    return false
}
func buffer_read() -> int {
    if len(buffer) > 0 {
        item := buffer[0]
        buffer = buffer[1..]
        return item
    }
    return 0
}
assert buffer_write(1)
assert buffer_write(2)
assert buffer_read() == 1

// 15. Shared counter with reset
counter := 0

func increment() {
    counter = counter + 1
}
func reset() {
    counter = 0
}
increment()
increment()
assert counter == 2
reset()
assert counter == 0

// 16. Shared state persistence
persistent_data := {"initialized": false}

func initialize() {
    persistent_data["initialized"] = true
}
initialize()
assert persistent_data["initialized"]

// 17. Thread-safe simulation (simplified)
func safe_increment(current: int) -> int {
    // In real implementation, would use atomic operations
    return current + 1
}
safe_counter := 0
safe_counter = safe_increment(safe_counter)
assert safe_counter == 1

// 18. Shared resource pool
pool: list[string] = ["resource1", "resource2", "resource3"]

func acquire_from_pool() -> string {
    if len(pool) > 0 {
        item := pool[0]
        pool = pool[1..]
        return item
    }
    return ""
}
func return_to_pool(item: string) {
    pool = pool + [item]
}
r1 := acquire_from_pool()
assert r1 == "resource1"
return_to_pool(r1)
assert len(pool) == 3

// 19. Shared state with mutex-like pattern
struct MutexData {
    value: int = 0
    locked: bool = false
}
mutex_data := MutexData(0, false)

func mutex_lock(data: MutexData) -> MutexData {
    data.locked = true
    return data
}
func mutex_unlock(data: MutexData) -> MutexData {
    data.locked = false
    return data
}
mutex_data = mutex_lock(mutex_data)
assert mutex_data.locked
mutex_data = mutex_unlock(mutex_data)
assert !mutex_data.locked

// 20. Shared state observer pattern
observers: list[string] = []

func add_observer(observer: string) {
    observers = observers + [observer]
}
func notify_observers(message: string) {
    for obs in observers {
        // In real implementation, would call observer callback
        assert len(obs) > 0
    }
}
add_observer("observer1")
add_observer("observer2")
assert len(observers) == 2

println("✓ test_34_shared_state passed")
