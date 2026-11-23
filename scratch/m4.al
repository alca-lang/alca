pub fn apply(f: func, x: int) -> int {
    r := f(x)
    return int(r)
}
pub fn apply2(x: int, f: func) -> int {
    return int(f(x))
}
pub fn make_adder(n: int) -> func {
    return |x: int| x + n
}
