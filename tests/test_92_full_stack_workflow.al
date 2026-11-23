// Copyright 2026 Pawl Programming Language
// Test 92: Full stack workflow - complete scenario simulation.

// 1. User registration
struct User {
    id: int
    name: string
    email: string
}
func register_user(name: string, email: string) -> User {
    return User(1, name, email)
}
user := register_user("Alice", "alice@example.com")
assert user.name == "Alice"

// 2. User authentication
func authenticate(user: User, password: string) -> bool {
    return password == "secret123"
}
assert authenticate(user, "secret123")

// 3. Data validation
func validate_email(email: string) -> bool {
    return string.contains(email, "@") and string.contains(email, ".")
}
assert validate_email("alice@example.com")

// 4. Data processing
func process_data(data: list[int]) -> int {
    total := 0
    for val in data {
        total = total + val
    }
    return total
}
processed := process_data([1, 2, 3, 4, 5])
assert processed == 15

// 5. Response formatting
func format_response(data: any) -> string {
    use std.json
    return json.encode(data)
}
response := format_response({"status": "ok"})
assert string.contains(response, "ok")

// 6. Error handling in workflow
func workflow_step(success: bool) -> string {
    if success {
        return "Step completed"
    }
    err("Step failed", code=500)
    return "Error"
}
assert workflow_step(true) == "Step completed"

// 7. Workflow state tracking
struct WorkflowState {
    current_step: int = 0
    completed: bool = false
}
func advance_step(state: WorkflowState) -> WorkflowState {
    state.current_step = state.current_step + 1
    if state.current_step >= 3 {
        state.completed = true
    }
    return state
}
wf := WorkflowState()
wf = advance_step(wf)
wf = advance_step(wf)
wf = advance_step(wf)
assert wf.completed

// 8. Logging workflow steps
steps_log: list[string] = []

func log_step(step: string) {
    steps_log = steps_log + [step]
}
log_step("Step 1")
log_step("Step 2")
assert len(steps_log) == 2

// 9. Workflow rollback simulation
func rollback(steps: int) {
    while steps > 0 {
        steps_log = steps_log[0..len(steps_log) - 1]
        steps = steps - 1
    }
}
rollback(1)
assert len(steps_log) == 1

// 10. Workflow completion check
func is_workflow_complete(state: WorkflowState) -> bool {
    return state.completed
}
assert is_workflow_complete(wf)

println("✓ test_92_full_stack_workflow passed")
