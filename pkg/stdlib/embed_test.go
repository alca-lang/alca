package stdlib

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
)

func TestEmbeddedResourceReadAndPathValidation(t *testing.T) {
	SetEmbeddedResources(map[string][]byte{"assets/data.bin": {0, 1, 255}})
	defer SetEmbeddedResources(nil)

	got, err := ReadEmbeddedResource(`assets\data.bin`)
	if err != nil || len(got) != 3 || got[2] != 255 {
		t.Fatalf("ReadEmbeddedResource returned %v, %v", got, err)
	}
	for _, name := range []string{"../secret", "/absolute", `..\secret`} {
		if _, err := ReadEmbeddedResource(name); err == nil {
			t.Errorf("expected invalid path %q to fail", name)
		}
	}
}

func TestRegisterEmbedBuiltins(t *testing.T) {
	SetEmbeddedResources(map[string][]byte{"assets/app.ico": []byte("ico"), "data/config.json": []byte("{}")})
	defer SetEmbeddedResources(nil)
	env := interpreter.NewEnvironment()
	RegisterEmbedBuiltins(env)

	moduleValue, ok := env.Get("std.embed")
	if !ok {
		t.Fatal("std.embed module was not registered")
	}
	module := moduleValue.(*interpreter.MapValue)
	readValue, _ := module.Get("read")
	read := readValue.(*interpreter.BuiltinFunction)
	got := read.Fn(&interpreter.StringValue{Val: "assets/app.ico"})
	if value, ok := got.(*interpreter.StringValue); !ok || value.Val != "ico" {
		t.Fatalf("embed.read returned %T (%v), want embedded string", got, got)
	}

	listValue, _ := module.Get("list")
	list := listValue.(*interpreter.BuiltinFunction)
	listResult := list.Fn(&interpreter.StringValue{Val: "assets"}).(*interpreter.ListValue)
	if len(listResult.Elements) != 1 || listResult.Elements[0].(*interpreter.StringValue).Val != "assets/app.ico" {
		t.Fatalf("embed.list returned %v", listResult.Inspect())
	}
}
