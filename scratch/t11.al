use std.json

fn f() -> json {
    return json.parse("[{\"a\":1},{\"a\":2}]")
}
fn g() -> list[json] {
    x := f()
    return list(x)
}
fn h() -> int {
    x := f()
    total := 0
    for e in x {
        total = total + int(e["a"])
    }
    return total
}
y := g()
println(len(y))
for e in y {
    println(e["a"])
}
println(h())
z: json = [1,2,3]
for n in z {
    println(n)
}
