use "./m4.al"

struct H {
    cb: func
}
pub fn run2(hs: list[H], x: int) -> list[func] {
    out: list[func] = []
    for h in hs {
        push(out, h.cb)
    }
    return out
}
