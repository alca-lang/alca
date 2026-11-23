// Copyright 2026 Pawl Programming Language
// Test 36: Class inheritance - extends, super, and method overriding.

// 1. Basic inheritance
struct Animal {
    name: string
    age: int
}
func make_sound(self: Animal) -> string {
    return "Generic animal sound"
}
struct Dog extends Animal {
    breed: string
}
func make_sound(self: Dog) -> string {
    return "Woof!"
}
dog := Dog("Buddy", 5, "Golden Retriever")
assert dog.name == "Buddy"
assert dog.age == 5
assert dog.breed == "Golden Retriever"
assert make_sound(dog) == "Woof!"

// 2. Accessing parent fields
cat := Animal("Whiskers", 3)
assert cat.name == "Whiskers"
assert make_sound(cat) == "Generic animal sound"

// 3. Method with inheritance
struct Vehicle {
    speed: int
}
func move(self: Vehicle) -> string {
    return "Moving at " + string(self.speed) + " km/h"
}
struct Car extends Vehicle {
    brand: string
}
func move(self: Car) -> string {
    return self.brand + " car moving at " + string(self.speed) + " km/h"
}
car := Car(120, "Toyota")
assert car.speed == 120
assert car.brand == "Toyota"
assert move(car) == "Toyota car moving at 120 km/h"

// 4. Constructor with inheritance
struct Person {
    name: string
}
func greet(self: Person) -> string {
    return "Hello, I'm " + self.name
}
struct Employee extends Person {
    employee_id: int
}
func greet(self: Employee) -> string {
    return "Hi, I'm " + self.name + " (ID: " + string(self.employee_id) + ")"
}
emp := Employee("Alice", 12345)
assert emp.name == "Alice"
assert emp.employee_id == 12345
assert greet(emp) == "Hi, I'm Alice (ID: 12345)"

// 5. Multiple levels of inheritance
struct Shape {
    color: string
}
struct Circle extends Shape {
    radius: float
}
func area(self: Circle) -> float {
    return 3.14159 * self.radius * self.radius
}
struct ColoredCircle extends Circle {
    pattern: string = "solid"
}
circle := ColoredCircle("red", 5.0)
assert circle.color == "red"
assert circle.radius == 5.0
assert area(circle) > 78.0 and area(circle) < 79.0

// 6. Overriding with different return types
struct Base {
    value: int
}
func get_value(self: Base) -> int {
    return self.value
}
struct Derived extends Base {
    extra: int
}
func get_value(self: Derived) -> int {
    return self.value + self.extra
}
derived := Derived(10, 5)
assert get_value(derived) == 15

// 7. Default values in inherited struct
struct BaseWithDefaults {
    x: int = 10
    y: int = 20
}
struct DerivedWithDefaults extends BaseWithDefaults {
    z: int = 30
}
derived_defaults := DerivedWithDefaults()
assert derived_defaults.x == 10
assert derived_defaults.y == 20
assert derived_defaults.z == 30

// 8. Method calling parent method (simulation)
func parent_greet(person: Person) -> string {
    return greet(person)
}
struct Manager extends Person {
    department: string
}
func greet(self: Manager) -> string {
    return parent_greet(self) + ", managing " + self.department
}
manager := Manager("Bob", "Engineering")
assert greet(manager) == "Hello, I'm Bob, managing Engineering"

// 9. Inheritance with nested structs
struct Address {
    street: string
    city: string
}
struct PersonWithAddress extends Person {
    address: Address
}
addr := Address("123 Main St", "City")
person_addr := PersonWithAddress("John", addr)
assert person_addr.name == "John"
assert person_addr.address.street == "123 Main St"

// 10. Type checking with inheritance
func is_animal(a: Animal) -> bool {
    return true
}
func is_dog(d: Dog) -> bool {
    return true
}
my_dog := Dog("Rex", 3, "Labrador")
assert is_animal(my_dog)
assert is_dog(my_dog)

// 11. Polymorphic behavior
func describe_animal(a: Animal) -> string {
    return "Animal: " + a.name + ", sound: " + make_sound(a)
}
animal_desc := describe_animal(dog)
assert string.contains(animal_desc, "Woof!")

// 12. Constructor chaining simulation
struct Point2D {
    x: float
    y: float
}
struct Point3D extends Point2D {
    z: float
}
p3d := Point3D(1.0, 2.0, 3.0)
assert p3d.x == 1.0
assert p3d.y == 2.0
assert p3d.z == 3.0

// 13. Protected-like access (public fields)
struct BaseClass {
    public_field: int
    private_field: int = 0
}
struct DerivedClass extends BaseClass {
    derived_field: int
}
derived_instance := DerivedClass(100, 200)
assert derived_instance.public_field == 100
assert derived_instance.derived_field == 200

// 14. Abstract-like pattern (methods that must be overridden)
struct Renderer {
    name: string
}
func render(self: Renderer) -> string {
    return "Default render"
}
struct SVGRenderer extends Renderer {
    width: int
}
func render(self: SVGRenderer) -> string {
    return "SVG " + string(self.width) + "px"
}
svg := SVGRenderer("SVG", 800)
assert render(svg) == "SVG 800px"

// 15. Interface-like behavior through inheritance
struct Flyable {
    can_fly: bool = true
}
struct Bird extends Animal {
    wingspan: float
}
bird := Bird("Eagle", 5, 2.5)
assert bird.name == "Eagle"
assert bird.wingspan == 2.5

println("✓ test_36_class_inheritance passed")
