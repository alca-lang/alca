// Copyright 2026 Pawl Programming Language
// Test 35: Channel simulation - queues, producer-consumer, and message passing.

// 1. Simple queue simulation
queue: list[int] = []

func enqueue(value: int) {
    queue = queue + [value]
}
func dequeue() -> int {
    if len(queue) > 0 {
        item := queue[0]
        queue = queue[1..]
        return item
    }
    return 0
}
enqueue(10)
enqueue(20)
enqueue(30)
assert dequeue() == 10
assert dequeue() == 20
assert len(queue) == 1

// 2. Bounded queue
struct BoundedQueue {
    data: list[int] = []
    capacity: int = 10
}
func bounded_enqueue(q: BoundedQueue, value: int) -> BoundedQueue {
    if len(q.data) < q.capacity {
        q.data = q.data + [value]
    }
    return q
}
func bounded_dequeue(q: BoundedQueue) -> (BoundedQueue, int) {
    if len(q.data) > 0 {
        item := q.data[0]
        q.data = q.data[1..]
        return (q, item)
    }
    return (q, 0)
}
bq := BoundedQueue([], 5)
bq = bounded_enqueue(bq, 1)
bq = bounded_enqueue(bq, 2)
(bq, item) := bounded_dequeue(bq)
assert item == 1

// 3. Producer-consumer pattern
producer_queue: list[int] = []

func producer() {
    i := 0
    while i < 5 {
        producer_queue = producer_queue + [i]
        i = i + 1
    }
}
func consumer() -> int {
    if len(producer_queue) > 0 {
        item := producer_queue[0]
        producer_queue = producer_queue[1..]
        return item
    }
    return -1
}
producer()
assert consumer() == 0
assert consumer() == 1

// 4. Priority queue simulation
struct PriorityItem {
    value: int
    priority: int
}
priority_queue: list[PriorityItem] = []

func priority_enqueue(pq: list[PriorityItem], item: PriorityItem) -> list[PriorityItem] {
    pq = pq + [item]
    // Simple sort by priority
    i := 0
    while i < len(pq) {
        j := i + 1
        while j < len(pq) {
            if pq[j].priority > pq[i].priority {
                temp := pq[i]
                pq[i] = pq[j]
                pq[j] = temp
            }
            j = j + 1
        }
        i = i + 1
    }
    return pq
}
func priority_dequeue(pq: list[PriorityItem]) -> (list[PriorityItem], PriorityItem) {
    if len(pq) > 0 {
        item := pq[0]
        pq = pq[1..]
        return (pq, item)
    }
    return (pq, PriorityItem(0, 0))
}
priority_queue = priority_enqueue(priority_queue, PriorityItem(10, 1))
priority_queue = priority_enqueue(priority_queue, PriorityItem(20, 3))
priority_queue = priority_enqueue(priority_queue, PriorityItem(30, 2))

(priority_queue, high_prio) := priority_dequeue(priority_queue)
assert high_prio.priority == 3

// 5. Message passing simulation
message_bus: list[string] = []

func send_message(msg: string) {
    message_bus = message_bus + [msg]
}
func receive_message() -> string {
    if len(message_bus) > 0 {
        msg := message_bus[0]
        message_bus = message_bus[1..]
        return msg
    }
    return ""
}
send_message("Hello")
send_message("World")
assert receive_message() == "Hello"
assert receive_message() == "World"

// 6. Broadcast channel
subscribers: list[list[string]] = []

func subscribe() -> int {
    subscribers = subscribers + [[]]
    return len(subscribers) - 1
}
func broadcast(msg: string) {
    i := 0
    while i < len(subscribers) {
        subscribers[i] = subscribers[i] + [msg]
        i = i + 1
    }
}
sub1 := subscribe()
sub2 := subscribe()
broadcast("test message")
assert len(subscribers[sub1]) == 1
assert len(subscribers[sub2]) == 1

// 7. Ring buffer
struct RingBuffer {
    data: list[int] = []
    size: int = 5
    head: int = 0
    tail: int = 0
}
func ring_write(rb: RingBuffer, value: int) -> RingBuffer {
    if len(rb.data) < rb.size {
        rb.data = rb.data + [value]
    }
    else {
        rb.data[rb.head] = value
        rb.head = (rb.head + 1) % rb.size
    }
    rb.tail = (rb.tail + 1) % rb.size
    return rb
}
func ring_read(rb: RingBuffer) -> (RingBuffer, int) {
    if len(rb.data) > 0 {
        item := rb.data[rb.head]
        rb.head = (rb.head + 1) % rb.size
        return (rb, item)
    }
    return (rb, 0)
}
ring := RingBuffer([], 5)
ring = ring_write(ring, 1)
ring = ring_write(ring, 2)
(ring, val) := ring_read(ring)
assert val == 1

// 8. Fan-out pattern
func fan_out(source: list[int], n: int) -> list[list[int]] {
    result: list[list[int]] = []
    i := 0
    while i < n {
        result = result + [[]]
        i = i + 1
    }
    idx := 0
    for item in source {
        result[idx] = result[idx] + [item]
        idx = (idx + 1) % n
    }
    return result
}
source_data := [1, 2, 3, 4, 5, 6]
fanned := fan_out(source_data, 3)
assert len(fanned) == 3

// 9. Fan-in pattern
func fan_in(channels: list[list[int]]) -> list[int] {
    result: list[int] = []
    for ch in channels {
        for item in ch {
            result = result + [item]
        }
    }
    return result
}
ch1 := [1, 2]
ch2 := [3, 4]
fanned_in := fan_in([ch1, ch2])
assert len(fanned_in) == 4

// 10. Channel timeout simulation
timeout_channel: list[int] = []
func wait_with_timeout(timeout_ms: int) -> int {
    if len(timeout_channel) > 0 {
        return timeout_channel[0]
    }
    return -1 // Timeout
}
timeout_channel = timeout_channel + [42]
result := wait_with_timeout(1000)
assert result == 42

// 11. Select pattern simulation
func select_case(ch1: list[int], ch2: list[int]) -> (int, string) {
    if len(ch1) > 0 {
        return (ch1[0], "ch1")
    }
    if len(ch2) > 0 {
        return (ch2[0], "ch2")
    }
    return (0, "none")
}
chan1 := [10]
chan2 := []
(val, source) := select_case(chan1, chan2)
assert val == 10
assert source == "ch1"

// 12. Buffered channel
buffered_channel: list[int] = []
buffer_size := 5

func buffered_send(value: int) -> bool {
    if len(buffered_channel) < buffer_size {
        buffered_channel = buffered_channel + [value]
        return true
    }
    return false
}
func buffered_recv() -> int {
    if len(buffered_channel) > 0 {
        item := buffered_channel[0]
        buffered_channel = buffered_channel[1..]
        return item
    }
    return 0
}
assert buffered_send(1)
assert buffered_send(2)
assert buffered_recv() == 1

// 13. Channel closing simulation
struct Channel {
    data: list[int] = []
    closed: bool = false
}
chan := Channel([], false)

func channel_send(ch: Channel, value: int) -> Channel {
    if !ch.closed {
        ch.data = ch.data + [value]
    }
    return ch
}
func channel_recv(ch: Channel) -> (Channel, int) {
    if len(ch.data) > 0 {
        item := ch.data[0]
        ch.data = ch.data[1..]
        return (ch, item)
    }
    return (ch, 0)
}
chan = channel_send(chan, 1)
chan = channel_send(chan, 2)
(chan, item) := channel_recv(chan)
assert item == 1

// 14. Channel direction (send-only, recv-only simulation)
send_only_chan: list[int] = []
recv_only_chan: list[int] = []

func send_only(value: int) {
    send_only_chan = send_only_chan + [value]
}
func recv_only() -> int {
    if len(recv_only_chan) > 0 {
        item := recv_only_chan[0]
        recv_only_chan = recv_only_chan[1..]
        return item
    }
    return 0
}
send_only(5)
recv_only_chan = send_only_chan
assert recv_only() == 5

// 15. Channel pipeline
func stage1(input: int) -> int {
    return input * 2
}
func stage2(input: int) -> int {
    return input + 10
}
func pipeline(value: int) -> int {
    return stage2(stage1(value))
}
pipeline_result := pipeline(5)
assert pipeline_result == 20

// 16. Worker pool with channels
tasks: list[int] = []
results: list[int] = []

func worker() {
    if len(tasks) > 0 {
        task := tasks[0]
        tasks = tasks[1..]
        results = results + [task * 2]
    }
}
tasks = tasks + [1]
tasks = tasks + [2]
tasks = tasks + [3]
worker()
worker()
worker()
assert results == [2, 4, 6]

// 17. Channel synchronization
sync_chan: list[bool] = []

func sync_send() {
    sync_chan = sync_chan + [true]
}
func sync_wait() {
    if len(sync_chan) > 0 {
        sync_chan = sync_chan[1..]
    }
}
sync_send()
assert len(sync_chan) == 1
sync_wait()
assert len(sync_chan) == 0

// 18. Channel multiplexing
func multiplex(channels: list[list[int]]) -> list[int] {
    result: list[int] = []
    for ch in channels {
        for item in ch {
            result = result + [item]
        }
    }
    return result
}
multi1 := [1, 2]
multi2 := [3, 4]
multi3 := [5, 6]
multiplexed := multiplex([multi1, multi2, multi3])
assert len(multiplexed) == 6

// 19. Channel capacity check
func channel_capacity(ch: list[int], max: int) -> bool {
    return len(ch) <= max
}
test_chan := [1, 2, 3]
assert channel_capacity(test_chan, 5)
assert !channel_capacity(test_chan, 2)

// 20. Channel drain
func drain_channel(ch: list[int]) -> list[int] {
    result: list[int] = []
    while len(ch) > 0 {
        result = result + [ch[0]]
        ch = ch[1..]
    }
    return result
}
drain_chan := [1, 2, 3, 4, 5]
drained := drain_channel(drain_chan)
assert len(drained) == 5
assert len(drain_chan) == 0

println("✓ test_35_channel_simulation passed")
