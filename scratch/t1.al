use std.json
use std.urllib2

struct Foo {
    name: string
    data: json
}
class Bar {
    pub token: string = "x"
    private base: string = "b"
    pub fn get() -> string {
        return self.token + self.base
    }
    pub fn paint(m: map[string, json]) -> map[string, json] {
        m["x"] = 1
        return m
    }
}
f := Foo("a", {"k": 1})
println(f.name)
println(f.data["k"])
println(type(f.data))
println(f.data)
b := Bar("tok")
println(b.get())
opts: map[string, json] = {"chat_id": 123, "text": "hi"}
opts["parse_mode"] = "HTML"
println(json.encode(opts))
res := json.parse("{\"ok\":true,\"result\":{\"message_id\":1}}")
println(res["ok"])
println(bool(res["ok"]))
if res["ok"] == true {
    println("yes")
}
d := res["result"]
println(d["message_id"])
println(type(d))
println(json.encode(d))
for k in keys(opts) {
    println(k)
}
num := 1
num = num + 2
println(string(num))
fn box(v: json) -> json {
    return v
}
println(json.encode(box([1,2,3])))
