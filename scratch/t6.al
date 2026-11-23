use "./m3.al"

b := m3.new_bot("t")
h := m3.Handler("a", |x: int| x + 1)
println(h.fnc(1))
h2 := m3.Handler("b", m3.double)
println(h2.fnc(3))
m3.add(b, h)
println(m3.run(b, 5))
