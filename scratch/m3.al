use "./m2.al"

pub struct Handler {
    name: string
    fnc: func
}
pub struct Bot {
    token: string
    handlers: list[Handler]
    count: int = 0
}
pub fn new_bot(t: string) -> Bot {
    return Bot(t, [])
}
pub fn add(b: Bot, h: Handler) -> bool {
    push(b.handlers, h)
    b.count = b.count + 1
    return true
}
pub fn run(b: Bot, x: int) -> int {
    total := 0
    for h in b.handlers {
        r := h.fnc(x)
        total = total + int(r)
    }
    return total
}
pub fn double(x: int) -> int {
    return x * 2
}
