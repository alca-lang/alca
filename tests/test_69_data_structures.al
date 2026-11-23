// Copyright 2026 Pawl Programming Language
// Test 69: Data structures - stack, queue, linked list simulation.

// 1. Stack implementation
struct Stack {
    items: list[int] = []
}
func push(s: Stack, item: int) -> Stack {
    s.items = s.items + [item]
    return s
}
func pop(s: Stack) -> (Stack, int) {
    if len(s.items) == 0 {
        return (s, 0)
    }
    item := s.items[len(s.items) - 1]
    s.items = s.items[0..len(s.items) - 1]
    return (s, item)
}
func peek(s: Stack) -> int {
    if len(s.items) == 0 {
        return 0
    }
    return s.items[len(s.items) - 1]
}
stack := Stack()
stack = push(stack, 1)
stack = push(stack, 2)
stack = push(stack, 3)
assert peek(stack) == 3
(stack, popped) := pop(stack)
assert popped == 3
assert peek(stack) == 2

// 2. Queue implementation
struct Queue {
    items: list[int] = []
}
func enqueue(q: Queue, item: int) -> Queue {
    q.items = q.items + [item]
    return q
}
func dequeue(q: Queue) -> (Queue, int) {
    if len(q.items) == 0 {
        return (q, 0)
    }
    item := q.items[0]
    q.items = q.items[1..]
    return (q, item)
}
queue := Queue()
queue = enqueue(queue, 10)
queue = enqueue(queue, 20)
queue = enqueue(queue, 30)
(queue, dequeued) := dequeue(queue)
assert dequeued == 10
(queue, dequeued2) := dequeue(queue)
assert dequeued2 == 20

// 3. Linked list node simulation
struct ListNode {
    value: int
    next: ListNode = null
}
func create_linked_list(values: list[int]) -> ListNode {
    if len(values) == 0 {
        return null
    }
    head := ListNode(values[0], null)
    current := head
    i := 1
    while i < len(values) {
        current.next = ListNode(values[i], null)
        current = current.next
        i = i + 1
    }
    return head
}
func traverse_list(head: ListNode) -> list[int] {
    result: list[int] = []
    current := head
    while current != null {
        result = result + [current.value]
        current = current.next
    }
    return result
}
list_head := create_linked_list([1, 2, 3, 4])
traversed := traverse_list(list_head)
assert traversed == [1, 2, 3, 4]

// 4. Stack size
func stack_size(s: Stack) -> int {
    return len(s.items)
}
stack2 := Stack()
stack2 = push(stack2, 1)
stack2 = push(stack2, 2)
assert stack_size(stack2) == 2

// 5. Queue size
func queue_size(q: Queue) -> int {
    return len(q.items)
}
queue2 := Queue()
queue2 = enqueue(queue2, 1)
queue2 = enqueue(queue2, 2)
queue2 = enqueue(queue2, 3)
assert queue_size(queue2) == 3

// 6. Stack is empty
func is_stack_empty(s: Stack) -> bool {
    return len(s.items) == 0
}
empty_stack := Stack()
assert is_stack_empty(empty_stack)

// 7. Queue is empty
func is_queue_empty(q: Queue) -> bool {
    return len(q.items) == 0
}
empty_queue := Queue()
assert is_queue_empty(empty_queue)

// 8. Stack clear
func clear_stack(s: Stack) -> Stack {
    s.items = []
    return s
}
stack3 := Stack()
stack3 = push(stack3, 1)
stack3 = push(stack3, 2)
stack3 = clear_stack(stack3)
assert is_stack_empty(stack3)

// 9. Queue clear
func clear_queue(q: Queue) -> Queue {
    q.items = []
    return q
}
queue3 := Queue()
queue3 = enqueue(queue3, 1)
queue3 = clear_queue(queue3)
assert is_queue_empty(queue3)

// 10. Stack to list
func stack_to_list(s: Stack) -> list[int] {
    return s.items
}
stack4 := Stack()
stack4 = push(stack4, 1)
stack4 = push(stack4, 2)
assert stack_to_list(stack4) == [1, 2]

// 11. Queue to list
func queue_to_list(q: Queue) -> list[int] {
    return q.items
}
queue4 := Queue()
queue4 = enqueue(queue4, 1)
queue4 = enqueue(queue4, 2)
assert queue_to_list(queue4) == [1, 2]

// 12. Reverse stack
func reverse_stack(s: Stack) -> Stack {
    reversed: list[int] = []
    i := len(s.items) - 1
    while i >= 0 {
        reversed = reversed + [s.items[i]]
        i = i - 1
    }
    s.items = reversed
    return s
}
stack5 := Stack()
stack5 = push(stack5, 1)
stack5 = push(stack5, 2)
stack5 = push(stack5, 3)
stack5 = reverse_stack(stack5)
assert stack_to_list(stack5) == [3, 2, 1]

// 13. Find in linked list
func find_in_list(head: ListNode, target: int) -> bool {
    current := head
    while current != null {
        if current.value == target {
            return true
        }
        current = current.next
    }
    return false
}
list2 := create_linked_list([10, 20, 30])
assert find_in_list(list2, 20)
assert !find_in_list(list2, 25)

// 14. Linked list length
func list_length(head: ListNode) -> int {
    count := 0
    current := head
    while current != null {
        count = count + 1
        current = current.next
    }
    return count
}
assert list_length(list2) == 3

// 15. Stack top without popping
assert peek(stack5) == 1

println("✓ test_69_data_structures passed")
