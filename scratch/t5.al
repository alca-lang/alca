use "./m3.al"

b := m3.new_bot("t")
h := m3.Handler("a", |x: int| x + 1)
m3.add(b, h)
m3.add(b, m3.Handler("b", |x: int| x * 2))
println(b.count)
println(len(b.handlers))
println(m3.run(b, 5))
for hh in b.handlers {
    println(hh.name)
}
missing: map[string, json] = {"a": 1}
println(missing["x"])
println("a" in missing)
println("x" in missing)
println(type(missing["x"]))
