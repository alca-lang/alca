// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package interpreter

import (
	"strings"
	"testing"
)

func TestInterpreterClassesAndInheritance(t *testing.T) {
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
speak := d.speak;
println(speak());
`
	var out strings.Builder
	SetPrintHook(func(s string) { out.WriteString(s) })
	defer ResetPrintHook()

	interp := New()
	if err := interp.Execute(parseTestProgram(t, src)); err != nil {
		t.Fatalf("execution error: %v", err)
	}

	want := "Rex says woofRex makes a sound34ballRex says woof"
	if got := out.String(); got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestInterpreterClassConstField(t *testing.T) {
	src := `
class Box:
    public const int: id = 1;
b := Box();
b.id = 2;
`
	interp := New()
	err := interp.Execute(parseTestProgram(t, src))
	if err == nil || !strings.Contains(err.Error(), "constant field") {
		t.Fatalf("expected constant field error, got %v", err)
	}
}
