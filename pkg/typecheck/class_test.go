// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package typecheck_test

import (
	"strings"
	"testing"
)

func TestClassesAndInheritance(t *testing.T) {
	src := `
class Animal:
    public string: name;
    private int: age = 0;

    public func speak() -> string:
        return self.name + " makes a sound";

    public func birthday() -> int:
        self.age = self.age + 1;
        return self.age;

    private func secret() -> int:
        return self.age;

    public func reveal() -> int:
        return self.secret();

class Dog extends Animal:
    public string: breed = "mutt";

    public func speak() -> string:
        return self.name + " says woof";

    public func parent_speak() -> string:
        return super.speak();

    public struct Toy:
        public string: label;

    public toy: Toy;

d := Dog("Rex", 3, "lab", Toy("ball"));
a: Animal = d;
println(a.speak());
println(d.parent_speak());
println(d.reveal());
println(d.birthday());
println(d.toy.label);
`
	if errs := check(t, src); len(errs) > 0 {
		t.Fatalf("valid class program produced errors:\n%s", strings.Join(errs, "\n"))
	}
}

func TestClassTypeErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "private field outside class",
			src: `
class Box:
    private int: value = 1;
b := Box();
println(b.value);`,
			want: "is private",
		},
		{
			name: "private method outside class",
			src: `
class Box:
    private func value() -> int:
        return 1;
b := Box();
println(b.value());`,
			want: "is private",
		},
		{
			name: "const field assignment",
			src: `
class Box:
    public const int: id = 1;
b := Box();
b.id = 2;`,
			want: "constant field",
		},
		{
			name: "override signature",
			src: `
class A:
    public func f(x: int) -> int:
        return x;
class B extends A:
    public func f(x: string) -> int:
        return 1;`,
			want: "different signature",
		},
		{
			name: "unknown base class",
			src: `
class B extends Missing:
    public int: x = 1;`,
			want: "unknown base class",
		},
		{
			name: "field hides inherited field",
			src: `
class A:
    public int: x = 1;
class B extends A:
    public int: x = 2;`,
			want: "already declared in a base class",
		},
		{
			name: "private base field in subclass method",
			src: `
class A:
    private int: x = 1;
class B extends A:
    public func get() -> int:
        return self.x;`,
			want: "is private",
		},
		{
			name: "redeclare private base method",
			src: `
class A:
    private func helper() -> int:
        return 1;
class B extends A:
    public func helper() -> int:
        return 2;`,
			want: "cannot redeclare private method",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := check(t, tc.src)
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q, got none", tc.want)
			}
			if !strings.Contains(strings.Join(errs, "\n"), tc.want) {
				t.Fatalf("expected an error containing %q, got:\n%s", tc.want, strings.Join(errs, "\n"))
			}
		})
	}
}
