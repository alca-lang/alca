use std.json

struct Filter {
    mode: string = 'any'
    value: string = ''
    state: string = ''
}
struct Handler {
    kind: string
    filter: Filter
    callback: func
    priority: int = 0
}
struct Router {
    prefix: string = ''
    handlers: list = []
    middlewares: list = []
}
struct Bot {
    token: string
    handlers: list = []
    middlewares: list = []
    on_error: func | null = null
    states: map[string, string] = {}
    state_data: map[string, json] = {}
}
func execute_chain(mws: list, idx: int, ctx: json, handler_cb: func) {
    if idx >= len(mws) {
        handler_cb(ctx)
        return
    }
    mw := mws[idx]
    next := |c: json| execute_chain(mws, idx + 1, c, handler_cb)
    mw(ctx, next)
}
func run_app() {
    b := Bot('token123')
    println('Bot initialized with token:', b.token)
}
run_app()
