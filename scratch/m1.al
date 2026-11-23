pub class Point {
    pub x: int = 1
    pub y: int = 2
    pub fn sum() -> int {
        return self.x + self.y
    }
}
pub fn make(a: int = 5, b: map[string, json] = {}) -> Point {
    p := Point(a)
    b["z"] = p.sum()
    return p
}
pub fn opts(a: int, b: string = "q", c: bool = true) -> string {
    return string(a) + b + string(c)
}
