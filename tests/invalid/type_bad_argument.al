// Negative fixture: passing a wrong argument type.
func add(a: int, b: int) -> int:
    return a + b;

println(add(10, "wrong argument"));
