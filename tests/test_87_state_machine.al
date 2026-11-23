// Copyright 2026 Pawl Programming Language
// Test 87: State machine - state transitions and management.

// 1. Simple state machine
enum State {
    Idle
    Running
    Paused
    Stopped
}
current_state := State.Idle

// 2. State transition
func transition(current: State, new_state: State) -> State {
    return new_state
}
current_state = transition(current_state, State.Running)
assert current_state == State.Running

// 3. Valid transition check
func is_valid_transition(from: State, to: State) -> bool {
    if from == State.Stopped {
        return false
    }
    return true
}
assert is_valid_transition(State.Idle, State.Running)
assert !is_valid_transition(State.Stopped, State.Running)

// 4. State machine with conditions
struct StateMachine {
    state: State = State.Idle
}
func change_state(sm: StateMachine, new_state: State) -> StateMachine {
    if is_valid_transition(sm.state, new_state) {
        sm.state = new_state
    }
    return sm
}
sm := StateMachine()
sm = change_state(sm, State.Running)
assert sm.state == State.Running

// 5. State history
state_history: list[State] = []

func record_state(s: State) {
    state_history = state_history + [s]
}
record_state(State.Idle)
record_state(State.Running)
assert len(state_history) == 2

// 6. State machine with actions
func on_enter(state: State) -> string {
    if state == State.Running {
        return "Starting"
    }
    if state == State.Stopped {
        return "Stopping"
    }
    return "Unknown"
}
action := on_enter(State.Running)
assert action == "Starting"

// 7. State machine with guards
func can_transition(sm: StateMachine, to: State) -> bool {
    if sm.state == State.Paused and to == State.Idle {
        return false
    }
    return true
}
sm2 := StateMachine(State.Paused)
assert !can_transition(sm2, State.Idle)

// 8. State machine with events
enum Event {
    Start
    Stop
    Pause
    Resume
}
func handle_event(sm: StateMachine, event: Event) -> StateMachine {
    if event == Event.Start and sm.state == State.Idle {
        sm.state = State.Running
    }
    if event == Event.Stop and sm.state == State.Running {
        sm.state = State.Stopped
    }
    if event == Event.Pause and sm.state == State.Running {
        sm.state = State.Paused
    }
    if event == Event.Resume and sm.state == State.Paused {
        sm.state = State.Running
    }
    return sm
}
sm3 := StateMachine()
sm3 = handle_event(sm3, Event.Start)
assert sm3.state == State.Running

// 9. State machine final state
func is_final_state(s: State) -> bool {
    return s == State.Stopped
}
assert !is_final_state(State.Running)
assert is_final_state(State.Stopped)

// 10. State machine reset
func reset_machine(sm: StateMachine) -> StateMachine {
    sm.state = State.Idle
    return sm
}
sm4 := StateMachine(State.Stopped)
sm4 = reset_machine(sm4)
assert sm4.state == State.Idle

println("✓ test_87_state_machine passed")
