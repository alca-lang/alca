pub struct Thing {
    name: string
    hello: func
    setname: func
}
pub fn new_thing(n: string) -> Thing {
    t := Thing(n, |x: int| 0, |x: int| 0)
    t.hello = |x: int| "hello " + t.name + " " + string(x)
    t.setname = |s: string| set_thing_name(t, s)
    return t
}
pub fn set_thing_name(t: Thing, s: string) -> bool {
    t.name = s
    return true
}
