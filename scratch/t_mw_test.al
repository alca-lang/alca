use std.json

struct Context {
    data: map[string, json] = {}
}
func mw1(ctx: Context, next: func) {
    println('mw1 before')
    next(ctx)
    println('mw1 after')
}
func mw2(ctx: Context, next: func) {
    println('mw2 before')
    next(ctx)
    println('mw2 after')
}
func handler(ctx: Context) {
    println('handler execute')
}
func make_chain(middlewares: list, final_handler: func) -> func {
    curr := final_handler
    i := len(middlewares) - 1
    while i >= 0 {
        m := middlewares[i]
        nxt := curr
        curr = |c: Context| m(c, nxt)
        i = i - 1
    }
    return curr
}
chain := make_chain([mw1, mw2], handler)
chain(Context({}))
