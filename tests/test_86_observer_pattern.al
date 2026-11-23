// Copyright 2026 Pawl Programming Language
// Test 86: Observer pattern - event subscription and notification.

// 1. Observer registration
struct Observer {
    id: int
    callback: func(string) -> void = null
}
observers: list[Observer] = []

func register_observer(obs: Observer) {
    observers = observers + [obs]
}
func unregister_observer(id: int) {
    result: list[Observer] = []
    for obs in observers {
        if obs.id != id {
            result = result + [obs]
        }
    }
    observers = result
}
obs1 := Observer(1, null)
obs2 := Observer(2, null)
register_observer(obs1)
register_observer(obs2)
assert len(observers) == 2

// 2. Notify observers
func notify_observers(message: string) {
    for obs in observers {
        println("Observer " + string(obs.id) + ": " + message)
    }
}
notify_observers("Test message")

// 3. Observer count
func observer_count() -> int {
    return len(observers)
}
assert observer_count() == 2

// 4. Clear observers
func clear_observers() {
    observers = []
}
clear_observers()
assert observer_count() == 0

// 5. Event types
enum EventType {
    Click
    Change
    Submit

// 6. Event emitter
}
struct EventEmitter {
    listeners: map[EventType, list[Observer]] = {}
}
func add_listener(emitter: EventEmitter, event: EventType, obs: Observer) -> EventEmitter {
    if event in emitter.listeners {
        emitter.listeners[event] = emitter.listeners[event] + [obs]
    }
    else {
        emitter.listeners[event] = [obs]
    }
    return emitter
}
emitter := EventEmitter()
emitter = add_listener(emitter, EventType.Click, obs1)
assert len(emitter.listeners[EventType.Click]) == 1

// 7. Emit event
func emit_event(emitter: EventEmitter, event: EventType, data: string) {
    if event in emitter.listeners {
        for obs in emitter.listeners[event] {
            println("Event " + string(event) + ": " + data)
        }
    }
}
emit_event(emitter, EventType.Click, "Button clicked")

// 8. Subject-Observers pattern
struct Subject {
    state: int = 0
    observers: list[Observer] = []
}
func attach(subject: Subject, obs: Observer) -> Subject {
    subject.observers = subject.observers + [obs]
    return subject
}
func detach(subject: Subject, id: int) -> Subject {
    result: list[Observer] = []
    for obs in subject.observers {
        if obs.id != id {
            result = result + [obs]
        }
    }
    subject.observers = result
    return subject
}
subj := Subject(0, [])
subj = attach(subj, obs1)
assert len(subj.observers) == 1

// 9. State change notification
func set_state(subject: Subject, new_state: int) -> Subject {
    subject.state = new_state
    for obs in subject.observers {
        println("State changed to " + string(new_state))
    }
    return subject
}
subj = set_state(subj, 42)
assert subj.state == 42

// 10. Observer with data
struct DataObserver {
    id: int
    data: string = ""
}
func update_data(obs: DataObserver, new_data: string) -> DataObserver {
    obs.data = new_data
    return obs
}
data_obs := DataObserver(1, "")
data_obs = update_data(data_obs, "new data")
assert data_obs.data == "new data"

println("✓ test_86_observer_pattern passed")
