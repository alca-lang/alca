use "./m1.al"

p := m1.make()
println(p.x)
println(p.y)
println(p.sum())
opts: map[string, json] = {}
q := m1.make(7, opts)
println(opts["z"])
println(m1.opts(1))
println(m1.opts(1, "w"))
println(m1.opts(1, "w", false))
f := |v: int| v * 2
println(f(4))
g := m1.make
println(g(3).sum())
