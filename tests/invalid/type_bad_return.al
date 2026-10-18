// Negative fixture: a function returning the wrong type.
func add(a: int, b: int) -> int:
    return "invalid return type";

println(add(1, 2));
