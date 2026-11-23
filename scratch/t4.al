use "./m2.al"

h := m2.make_holder(10)
println(h.value)
println(h.ping(5))
d := json_decode("{\"a\":{\"b\":2}}")
println(d["a"]["b"])
println(d.a.b)
