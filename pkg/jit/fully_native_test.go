package jit

import (
	"testing"

	"github.com/alca-lang/alca/pkg/vm"
)

// TestFullyNativeRejectsBailoutOpcodes guards the native PE build gate: a
// program with calls or builtins cannot run without the VM fallback, so it must
// use the runtime stub instead of a pure native PE.
func TestFullyNativeRejectsBailoutOpcodes(t *testing.T) {
	// print("hello") compiles to a builtin lookup followed by a call.
	withCall := append(vm.Make(vm.OpGetBuiltin, 0), vm.Make(vm.OpCall, 1)...)
	withCall = append(withCall, vm.Make(vm.OpPop)...)
	if FullyNative(withCall) {
		t.Fatal("FullyNative accepted a program containing OpGetBuiltin/OpCall")
	}

	userCall := append(vm.Make(vm.OpGetGlobal, 0), vm.Make(vm.OpCall, 0)...)
	if FullyNative(userCall) {
		t.Fatal("FullyNative accepted a program containing OpCall")
	}
}

func TestFullyNativeAcceptsArithmeticOnly(t *testing.T) {
	ins := append(vm.Make(vm.OpConstant, 1), vm.Make(vm.OpConstant, 2)...)
	ins = append(ins, vm.Make(vm.OpAdd)...)
	ins = append(ins, vm.Make(vm.OpReturnValue)...)
	if !FullyNative(ins) {
		t.Fatal("FullyNative rejected a straight-line arithmetic program")
	}
}

func TestFullyNativeRejectsStrictEqualityWithPossibleObjects(t *testing.T) {
	ins := append(vm.Make(vm.OpConstant, 0), vm.Make(vm.OpConstant, 1)...)
	ins = append(ins, vm.Make(vm.OpStrictEqual)...)
	ins = append(ins, vm.Make(vm.OpReturnValue)...)
	if FullyNative(ins) {
		t.Fatal("FullyNative accepted strict equality that may require VM deep equality")
	}
}

func TestFullyNativeRejectsMathIntrinsicWithRuntimeFallback(t *testing.T) {
	ins := append(vm.Make(vm.OpGetLocal, 0), vm.Make(vm.OpMathBuiltin, int(vm.MathBuiltinSqrt))...)
	ins = append(ins, vm.Make(vm.OpReturnValue)...)
	if FullyNative(ins) {
		t.Fatal("FullyNative accepted a math intrinsic that may require VM error handling")
	}
}

func TestFullyNativeRejectsTruncatedOrUnknownInstructions(t *testing.T) {
	if FullyNative([]byte{byte(vm.OpConstant), 0}) {
		t.Fatal("FullyNative accepted a truncated constant operand")
	}
	if FullyNative([]byte{0xFF}) {
		t.Fatal("FullyNative accepted an unknown opcode")
	}
}
