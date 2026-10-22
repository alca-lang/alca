package vm

import "testing"

// TestClassesOnBothEngines covers the class support of the bytecode VM: fields
// with visibility, methods, single inheritance, `self`, `super`, dynamic
// dispatch, method values and nested types. Every program must print the same
// thing on the tree-walking interpreter and on the VM.
func TestClassesOnBothEngines(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "fields and methods",
			src: `class Animal:
    public string: name;

    public func speak() -> string:
        return self.name + " makes a sound";

a := Animal("Rex");
println(a.speak());
`,
			want: "Rex makes a sound",
		},
		{
			name: "private field keeps its value",
			src: `class Counter:
    private int: count = 0;

    public func bump() -> int:
        self.count = self.count + 1;
        return self.count;

c := Counter();
println(c.bump());
println(c.bump());
`,
			want: "12",
		},
		{
			name: "inheritance and super",
			src: `class Animal:
    public string: name;

    public func speak() -> string:
        return self.name + " makes a sound";

class Dog extends Animal:
    public func speak() -> string:
        return self.name + " says woof";

    public func parent_speak() -> string:
        return super.speak();

d := Dog("Rex");
println(d.speak());
println(d.parent_speak());
`,
			want: "Rex says woofRex makes a sound",
		},
		{
			name: "dynamic dispatch through the base class",
			src: `class Animal:
    public string: name;

    public func speak() -> string:
        return self.name + " makes a sound";

class Dog extends Animal:
    public func speak() -> string:
        return self.name + " says woof";

a: Animal = Dog("Rex");
println(a.speak());
`,
			want: "Rex says woof",
		},
		{
			name: "base fields come first in the constructor",
			src: `class Animal:
    public string: name;

class Dog extends Animal:
    public string: breed;

d := Dog("Rex", "lab");
println(d.name);
println(d.breed);
`,
			want: "Rexlab",
		},
		{
			name: "method value keeps its receiver",
			src: `class Animal:
    public string: name;

    public func speak() -> string:
        return self.name + " makes a sound";

a := Animal("Rex");
speak := a.speak;
println(speak());
`,
			want: "Rex makes a sound",
		},
		{
			name: "nested type",
			src: `class Box:
    public struct Item:
        public string: label;

    public item: Item;

b := Box(Item("ball"));
println(b.item.label);
`,
			want: "ball",
		},
		{
			name: "private method is reachable from the class",
			src: `class Counter:
    private int: count = 5;

    private func secret() -> int:
        return self.count;

    public func reveal() -> int:
        return self.secret();

c := Counter();
println(c.reveal());
`,
			want: "5",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			interpOut, vmOut := runOnBothEngines(t, tc.src)
			if interpOut != tc.want {
				t.Errorf("interpreter output = %q, want %q", interpOut, tc.want)
			}
			if vmOut != tc.want {
				t.Errorf("vm output = %q, want %q", vmOut, tc.want)
			}
		})
	}
}
