use std.json

state_store: map[string, string] = {}
data_store: map[string, map[string, json]] = {}

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
    if not (user_id in data_store) {
        data_store[user_id] = {}
    }
    data_store[user_id][key] = val
}
func get_data(user_id: string, key: string) -> json {
    if user_id in data_store {
        if key in data_store[user_id] {
            return data_store[user_id][key]
        }
    }
    return null
}
set_state('123', 'WAIT_NAME')
set_data('123', 'age', 25)
println('state:', get_state('123'))
println('age:', get_data('123', 'age'))
