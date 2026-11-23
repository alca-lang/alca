pub struct Holder {
    value: int
    ping: func
}
pub fn make_holder(n: int) -> Holder {
    h := Holder(n, |x: int| x + 1)
    return h
}
