use std.math
use std.crypto
use std.path
use std.time

test "Basic Arithmetic and Types":
    int a = 10
    int b = 25
    int sum = a + b
    assert sum == 35
    assert sum * 2 == 70
    assert sum % 10 == 5

test "String Operations and Concatenation":
    string greeting = "Hello"
    string target = "Alca"
    string msg = greeting + ", " + target + "!"
    assert msg == "Hello, Alca!"
    assert len(msg) == 12

test "Lists and Collections":
    items = [1, 2, 3, 4, 5]
    assert len(items) == 5
    assert items[0] == 1
    assert items[4] == 5

test "Maps and Key-Value Pairs":
    user = {"name": "Alice", "role": "admin"}
    assert user["name"] == "Alice"
    assert user["role"] == "admin"

test "Math Standard Library":
    assert math.abs(-42) == 42
    assert math.max(10, 20) == 20
    assert math.min(10, 20) == 10
    assert math.sqrt(16) == 4

test "Crypto Standard Library":
    string hash = crypto.sha256("test")
    assert len(hash) == 64
    string uuid = crypto.uuid()
    assert len(uuid) == 36

test "Functions and Closures":
    func multiply(x: int, y: int) -> int:
        return x * y
    
    int result = multiply(6, 7)
    assert result == 42

test "Struct Definitions and Instances":
    struct Point:
        x: int
        y: int

    pt = Point(15, 30)
    assert pt.x == 15
    assert pt.y == 30
