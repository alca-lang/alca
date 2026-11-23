use std.time
async fn slow_calc(x: int) -> int {
    sleep(100)
    return x * 2
}
t0 := time.now()
v := slow_calc(21)
println('v =', v)
res := await slow_calc(10)
println('res =', res)
