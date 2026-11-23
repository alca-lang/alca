use "./m5.al"

hs := [m5.H(|x: int| x + 1), m5.H(|x: int| x * 2)]
fs := m5.run2(hs, 3)
for f in fs {
    println(f(10))
}
println(hs[0].cb(5))
