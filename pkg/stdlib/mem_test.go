package stdlib_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func memModule(t *testing.T) *interpreter.MapValue {
	t.Helper()
	env := interpreter.NewEnvironment()
	stdlib.RegisterMemBuiltins(env)
	value, ok := env.Get("std.mem")
	if !ok {
		t.Fatal("expected std.mem to be registered")
	}
	return value.(*interpreter.MapValue)
}

func memCall(t *testing.T, module *interpreter.MapValue, name string, args ...interpreter.Value) interpreter.Value {
	t.Helper()
	fn, ok := module.Get(name)
	if !ok {
		t.Fatalf("std.mem has no member %q", name)
	}
	return fn.(*interpreter.BuiltinFunction).Fn(args...)
}

func TestMemModuleReadWrite(t *testing.T) {
	module := memModule(t)

	address := memCall(t, module, "alloc", &interpreter.IntegerValue{Val: 16})
	if address.Type() == interpreter.ERROR_VAL {
		t.Fatalf("alloc failed: %s", address.(*interpreter.ErrorValue).Message)
	}
	addr := address.(*interpreter.IntegerValue).Val

	if written := memCall(t, module, "write_u32", &interpreter.IntegerValue{Val: addr}, &interpreter.IntegerValue{Val: 0xdeadbeef}); written.Type() == interpreter.ERROR_VAL {
		t.Fatalf("write_u32 failed: %s", written.(*interpreter.ErrorValue).Message)
	}
	read := memCall(t, module, "read_u32", &interpreter.IntegerValue{Val: addr})
	if read.Type() == interpreter.ERROR_VAL {
		t.Fatalf("read_u32 failed: %s", read.(*interpreter.ErrorValue).Message)
	}
	if value := read.(*interpreter.IntegerValue).Val; int32(value) != -559038737 {
		t.Fatalf("read_u32 = %d, want %d", value, int32(-559038737))
	}

	// A float round trip.
	if written := memCall(t, module, "write_f64", &interpreter.IntegerValue{Val: addr}, &interpreter.FloatValue{Val: 2.5}); written.Type() == interpreter.ERROR_VAL {
		t.Fatalf("write_f64 failed: %s", written.(*interpreter.ErrorValue).Message)
	}
	readFloat := memCall(t, module, "read_f64", &interpreter.IntegerValue{Val: addr})
	if readFloat.Type() == interpreter.ERROR_VAL {
		t.Fatalf("read_f64 failed: %s", readFloat.(*interpreter.ErrorValue).Message)
	}
	if value := readFloat.(*interpreter.FloatValue).Val; value != 2.5 {
		t.Fatalf("read_f64 = %v, want 2.5", value)
	}

	// The size of the block is reported.
	size := memCall(t, module, "size", &interpreter.IntegerValue{Val: addr})
	if size.Type() == interpreter.ERROR_VAL || size.(*interpreter.IntegerValue).Val != 16 {
		t.Fatalf("size = %v, want 16", size)
	}
}

func TestMemModuleRejectsOutOfBoundsAccess(t *testing.T) {
	module := memModule(t)
	address := memCall(t, module, "alloc", &interpreter.IntegerValue{Val: 8})
	addr := address.(*interpreter.IntegerValue).Val

	// Reading eight bytes at the end of the block is outside of it.
	read := memCall(t, module, "read_u64", &interpreter.IntegerValue{Val: addr}, &interpreter.IntegerValue{Val: 1})
	if read.Type() != interpreter.ERROR_VAL {
		t.Fatalf("a read outside of the block must fail, got %v", read)
	}
	write := memCall(t, module, "write_u64", &interpreter.IntegerValue{Val: addr}, &interpreter.IntegerValue{Val: 1}, &interpreter.IntegerValue{Val: 1})
	if write.Type() != interpreter.ERROR_VAL {
		t.Fatalf("a write outside of the block must fail, got %v", write)
	}
	// An address the program did not allocate is unknown.
	unknown := memCall(t, module, "read_u32", &interpreter.IntegerValue{Val: 0x1234})
	if unknown.Type() != interpreter.ERROR_VAL {
		t.Fatalf("an unknown address must fail, got %v", unknown)
	}
}

func TestMemModuleFreeAndReuse(t *testing.T) {
	module := memModule(t)
	address := memCall(t, module, "alloc", &interpreter.IntegerValue{Val: 4})
	addr := address.(*interpreter.IntegerValue).Val
	if freed := memCall(t, module, "free", &interpreter.IntegerValue{Val: addr}); freed.Type() == interpreter.ERROR_VAL {
		t.Fatalf("free failed: %s", freed.(*interpreter.ErrorValue).Message)
	}
	// The block is gone, so it cannot be read any more.
	read := memCall(t, module, "read_u32", &interpreter.IntegerValue{Val: addr})
	if read.Type() != interpreter.ERROR_VAL {
		t.Fatalf("a freed block must not be readable, got %v", read)
	}
	// Freeing it twice is an error, not a crash.
	if again := memCall(t, module, "free", &interpreter.IntegerValue{Val: addr}); again.Type() != interpreter.ERROR_VAL {
		t.Fatalf("freeing a block twice must fail, got %v", again)
	}
}
