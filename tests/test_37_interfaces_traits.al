// Copyright 2026 Pawl Programming Language
// Test 37: Interfaces and traits - polymorphic contracts and implementation.

// 1. Interface-like pattern using structs
struct Drawable {
    x: int
    y: int
}
func draw(self: Drawable) -> string {
    return "Drawing at (" + string(self.x) + ", " + string(self.y) + ")"
}
struct Circle {
    x: int
    y: int
    radius: int
}
func draw(self: Circle) -> string {
    return "Circle at (" + string(self.x) + ", " + string(self.y) + ") radius " + string(self.radius)
}
struct Rectangle {
    x: int
    y: int
    width: int
    height: int
}
func draw(self: Rectangle) -> string {
    return "Rectangle at (" + string(self.x) + ", " + string(self.y) + ") size " + string(self.width) + "x" + string(self.height)
}
circle := Circle(10, 20, 5)
rect := Rectangle(30, 40, 15, 10)

assert draw(circle) == "Circle at (10, 20) radius 5"
assert draw(rect) == "Rectangle at (30, 40) size 15x10"

// 2. Polymorphic function
func render_any(item: any) -> string {
    if typeof(item) == "Circle" {
        return draw(item)
    }
    if typeof(item) == "Rectangle" {
        return draw(item)
    }
    return "Unknown shape"
}
assert render_any(circle) == "Circle at (10, 20) radius 5"
assert render_any(rect) == "Rectangle at (30, 40) size 15x10"

// 3. Comparable interface simulation
struct ComparableValue {
    value: int
}
func compare(a: ComparableValue, b: ComparableValue) -> int {
    if a.value < b.value {
        return -1
    }
    if a.value > b.value {
        return 1
    }
    return 0
}
val1 := ComparableValue(10)
val2 := ComparableValue(20)
assert compare(val1, val2) == -1
assert compare(val2, val1) == 1
assert compare(val1, val1) == 0

// 4. Stringer interface simulation
struct Stringable {
    data: string
}
func to_string(self: Stringable) -> string {
    return self.data
}
s := Stringable("Hello")
assert to_string(s) == "Hello"

// 5. Container interface simulation
struct Container {
    items: list[int] = []
}
func add(self: Container, item: int) -> Container {
    self.items = self.items + [item]
    return self
}
func get_size(self: Container) -> int {
    return len(self.items)
}
container := Container()
container = add(container, 1)
container = add(container, 2)
assert get_size(container) == 2

// 6. Iterator interface simulation
struct Iterator {
    data: list[int] = []
    index: int = 0
}
func next(self: Iterator) -> (Iterator, int) {
    if self.index < len(self.data) {
        item := self.data[self.index]
        self.index = self.index + 1
        return (self, item)
    }
    return (self, 0)
}
iter := Iterator([1, 2, 3], 0)
(iter, item1) := next(iter)
(iter, item2) := next(iter)
assert item1 == 1
assert item2 == 2

// 7. Predicate interface simulation
func filter(items: list[int], predicate: func(int) -> bool) -> list[int] {
    result: list[int] = []
    for item in items {
        if predicate(item) {
            result = result + [item]
        }
    }
    return result
}
is_even := func(n: int) -> bool:
    return n % 2 == 0

evens := filter([1, 2, 3, 4, 5], is_even)
assert evens == [2, 4]

// 8. Mapper interface simulation
func map(items: list[int], mapper: func(int) -> int) -> list[int] {
    result: list[int] = []
    for item in items {
        result = result + [mapper(item)]
    }
    return result
}
double := func(n: int) -> int:
    return n * 2

doubled := map([1, 2, 3], double)
assert doubled == [2, 4, 6]

// 9. Reducer interface simulation
func reduce(items: list[int], reducer: func(int, int) -> int, initial: int) -> int {
    result := initial
    for item in items {
        result = reducer(result, item)
    }
    return result
}
sum := func(a: int, b: int) -> int:
    return a + b

total := reduce([1, 2, 3, 4], sum, 0)
assert total == 10

// 10. Validator interface simulation
struct Validator {
    value: any
}
func validate(self: Validator) -> bool {
    if typeof(self.value) == "int" {
        return self.value > 0
    }
    if typeof(self.value) == "string" {
        return len(self.value) > 0
    }
    return false
}
int_validator := Validator(42)
assert validate(int_validator)

str_validator := Validator("hello")
assert validate(str_validator)

// 11. Cloneable interface simulation
struct Cloneable {
    data: int
}
func clone(self: Cloneable) -> Cloneable {
    return Cloneable(self.data)
}
original := Cloneable(100)
cloned := clone(original)
assert cloned.data == 100
assert cloned !== original

// 12. Serializable interface simulation
struct Serializable {
    name: string
    value: int
}
func serialize(self: Serializable) -> string {
    return self.name + ":" + string(self.value)
}
func deserialize(data: string) -> Serializable {
    parts := string.split(data, ":")
    return Serializable(parts[0], int(parts[1]))
}
obj := Serializable("test", 42)
serialized := serialize(obj)
deserialized := deserialize(serialized)
assert deserialized.name == "test"
assert deserialized.value == 42

// 13. Observer pattern interface
struct Observer {
    name: string
}
func notify(self: Observer, message: string) {
    println(self.name + " received: " + message)
}
observer1 := Observer("Observer1")
observer2 := Observer("Observer2")

// 14. Strategy pattern interface
struct Strategy {
    name: string
}
func execute(self: Strategy, data: int) -> int {
    if self.name == "double" {
        return data * 2
    }
    if self.name == "square" {
        return data * data
    }
    return data
}
double_strategy := Strategy("double")
square_strategy := Strategy("square")

assert execute(double_strategy, 5) == 10
assert execute(square_strategy, 5) == 25

// 15. Builder pattern interface
struct Builder {
    parts: list[string] = []
}
func add_part(self: Builder, part: string) -> Builder {
    self.parts = self.parts + [part]
    return self
}
func build(self: Builder) -> string {
    return string.join(self.parts, " ")
}
builder := Builder()
builder = add_part(builder, "Hello")
builder = add_part(builder, "World")
result := build(builder)
assert result == "Hello World"

println("✓ test_37_interfaces_traits passed")
