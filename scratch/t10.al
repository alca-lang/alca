use "./m6.al"

t := m6.new_thing("A")
println(t.name)
println(t.hello(3))
r := t.setname("B")
println(r)
println(t.name)
