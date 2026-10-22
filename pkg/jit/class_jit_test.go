package jit_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/jit"
)

// TestClassProgramsRunOnTheJITTiers verifies that a program with classes runs
// on the JIT tiers: the class definition is built by the runtime helper tier,
// and the methods are compiled like any other function.
func TestClassProgramsRunOnTheJITTiers(t *testing.T) {
	const src = `
class Animal:
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
`
	const want = "Rex says woof\nRex makes a sound\n"

	for _, mode := range []jit.StdlibExecutionMode{jit.StdlibThreadedJIT, jit.StdlibAMD64JIT} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			out, err, _ := runPureStdlibMode(t, src, mode)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if out != want {
				t.Fatalf("output = %q, want %q", out, want)
			}
		})
	}
}
