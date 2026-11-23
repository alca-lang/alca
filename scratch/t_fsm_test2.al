use std.json

state_store: map[string, string] = {}
data_store: map[string, json] = {}

func set_state(user_id: string, state: string) {
    state_store[user_id] = state
}
func get_state(user_id: string) -> string {
    if user_id in state_store {
        return state_store[user_id]
    }
    return ''
}
func set_data(user_id: string, key: string, val: json) {
    cur: map[string, json] = {}
    if user_id in data_store {
        cur = map(data_store[user_id])
    }
    cur[key] = val
    data_store[user_id] = cur
}
func get_data(user_id: string, key: string) -> json {
    if user_id in data_store {
        cur: map[string, json] = map(data_store[user_id])
        if key in cur {
            return cur[key]
        }
    }
    return null
}
set_state('123', 'WAIT_NAME')
set_data('123', 'age', 25)
println('state:', get_state('123'))
println('age:', get_data('123', 'age'))
