use std.json

struct Bot {
    on_error: func | null = null
}
func safe_handle(b: Bot, cb: func, ctx: json) {
    e: error | null = null
    cb(ctx) ?>>> e
    if e != null {
        println('caught exception:', e.msg)
        err_fn := b.on_error
        if err_fn != null {
            err_fn(ctx, e)
        }
    }
}
func bad_fn(ctx: json) {
    err('database down', code=500)
}
func custom_err(ctx: json, e: error) {
    println('custom handler logged:', e.msg)
}
b := Bot(custom_err)
safe_handle(b, bad_fn, {})
println('still alive!')
