use std.math;
use std.crypto;
use std.path;
use std.time;

test "Basic Arithmetic and Types":
    a: int = 10;
    b: int = 25;
    sum: int = a + b;
    assert sum == 35;
    assert sum * 2 == 70;
    assert sum % 10 == 5;

test "String Operations and Concatenation":
    greeting: string = "Hello";
    target: string = "Alca";
    msg: string = greeting + ", " + target + "!";
    assert msg == "Hello, Alca!";
    assert len(msg) == 12;

test "Lists and Collections":
    items: list[int] = [1, 2, 3, 4, 5];
    assert len(items) == 5;
    assert items[0] == 1;
    assert items[4] == 5;

test "Maps and Key-Value Pairs":
    user: map[string, string] = {"name": "Alice", "role": "admin"};
    assert user["name"] == "Alice";
    assert user["role"] == "admin";

test "Math Standard Library":
    assert math.abs(-42) == 42;
    assert math.max(10, 20) == 20;
    assert math.min(10, 20) == 10;
    assert math.sqrt(16) == 4.0;

test "Crypto Standard Library":
    hash: string = crypto.sha256("test");
    assert len(hash) == 64;
    uuid: string = crypto.uuid();
    assert len(uuid) == 36;

test "Functions":
    func multiply(x: int, y: int) -> int:
        return x * y;

    result: int = multiply(6, 7);
    assert result == 42;

test "Struct Definitions and Instances":
    struct Point:
        x: int;
        y: int;

    pt: Point = Point(15, 30);
    assert pt.x == 15;
    assert pt.y == 30;

test "Path Standard Library":
    joined: string = path.join("projects", "alca", "main.al");
    assert path.base(joined) == "main.al";
    assert path.ext(joined) == ".al";
    assert path.stem(joined) == "main";

test "Time Standard Library":
    ts: int = time.now();
    assert ts > 0;
    ms: int = time.now_ms();
    assert ms > 0;

test "Loops and Branches":
    total: int = 0;
    for i in 1..5:
        total = total + i;
    assert total == 15;

    i: int = 0;
    while i < 5:
        i = i + 1;
    assert i == 5;

    label: string = "";
    if i == 5:
        label = "five";
    else:
        label = "other";
    assert label == "five";

test "Errors and Null Narrowing":
    func safe_div(a: int, b: int) -> int:
        if b == 0:
            err("division by zero", code=1);
            return 0;
        return a / b;

    assert safe_div(10, 2) == 5;

    failure: error | null = null;
    safe_div(10, 0) ?>>> failure;
    assert failure != null;
    if failure != null:
        assert failure.msg == "division by zero";
