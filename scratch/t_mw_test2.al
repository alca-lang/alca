use std.json

struct Context {
    data: map[string, json] = {}
}
func run_pipeline(mws: list, idx: int, ctx: Context, handler: func) {
    if idx >= len(mws) {
        handler(ctx)
        return
    }
    mw := mws[idx]
    next := |c: Context| run_pipeline(mws, idx + 1, c, handler)
    mw(ctx, next)
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
run_pipeline([mw1, mw2], 0, Context({}), handler)
