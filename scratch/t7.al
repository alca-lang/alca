use "./m4.al"

f := |x: int| x * 5
println("direct", f(2))
println("apply", m4.apply(f, 2))
println("apply2", m4.apply2(2, f))
g := m4.make_adder(10)
println("adder", g(2))
