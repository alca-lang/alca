func bad_handler(x: int) {
    err('something blew up', code=500)
}
func safe_dispatch(h: func, x: int) {
    e: error | null = null
    h(x) ?>>> e
    if e != null {
        println('recovered from error:', e.msg)
    }
    else {
        println('all ok')
    }
}
safe_dispatch(bad_handler, 1)
println('loop continues running')
