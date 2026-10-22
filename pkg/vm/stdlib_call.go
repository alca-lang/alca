package vm

import (
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// BuildListSlots constructs a list with the same int-optimized representation
// the VM's OpList uses. The input slots are not modified.
func BuildListSlots(slots []StackSlot) interpreter.Value {
	allInts := true
	for i := range slots {
		if slots[i].tag != SlotInt {
			allInts = false
			break
		}
	}
	if allInts {
		ints := make([]int64, len(slots))
		for i := range slots {
			ints[i] = slots[i].i64
		}
		return interpreter.NewIntList(ints)
	}
	elements := make([]interpreter.Value, len(slots))
	for i := range slots {
		elements[i] = slots[i].ToValue()
	}
	return interpreter.NewList(elements...)
}

// BuildMapSlots constructs a map from alternating key/value slots, matching
// the VM's OpMap coercion rules.
func BuildMapSlots(slots []StackSlot) interpreter.Value {
	m := interpreter.NewMap()
	for i := 0; i+1 < len(slots); i += 2 {
		m.Set(interpreter.CoerceToString(slots[i].ToValue()), slots[i+1].ToValue())
	}
	return m
}

// LookupStdlibMember resolves a standard-library member through the module
// binding installed by a use statement. member may be a nested path such as
// "json.encode" for formats.json.encode.
func LookupStdlibMember(env *interpreter.Environment, alias, member string) (interpreter.Value, bool) {
	if env == nil {
		return nil, false
	}
	value, ok := env.Get(alias)
	if !ok {
		return nil, false
	}
	for _, field := range strings.Split(member, ".") {
		module, ok := value.(*interpreter.MapValue)
		if !ok {
			return nil, false
		}
		value, ok = module.Get(field)
		if !ok {
			return nil, false
		}
	}
	return value, true
}

// ResolveBuiltinValue implements the OpGetBuiltin lookup order: environment
// binding first, then the always-available builtin function map, then null.
func ResolveBuiltinValue(env *interpreter.Environment, name string) interpreter.Value {
	if env != nil {
		if value, ok := env.Get(name); ok {
			return value
		}
	}
	if fn, ok := interpreter.BuiltinFunctionMap[name]; ok {
		return fn
	}
	return interpreter.NULL
}

// executeStdlibCall invokes a fused pure stdlib call. The stack holds only the
// arguments; the callable is resolved from the module binding. Reusing
// executeCall keeps arity checks, method wrappers, and error propagation
// identical to a normal module-member call.
func (vm *VM) executeStdlibCall(alias, member string, numArgs int) error {
	callee, found := LookupStdlibMember(vm.env, alias, member)
	if !found {
		// Preserve the generic member-access behavior for misspelled members
		// or a module value changed at runtime. In particular, a missing field
		// produces the same method-wrapper error as OpGetBuiltin + OpGetField
		// + OpCall instead of a different "calling non-function" diagnostic.
		var err error
		callee, err = vm.resolveStdlibMember(alias, member)
		if err != nil {
			return err
		}
	}
	start := vm.sp - numArgs
	copy(vm.stack[start+1:vm.sp+1], vm.stack[start:vm.sp])
	vm.stack[start] = FromValue(callee)
	vm.sp++
	return vm.executeCall(callee, numArgs)
}

// resolveStdlibMember mirrors the generic OpGetBuiltin + OpGetField sequence
// for a nested member path.
func (vm *VM) resolveStdlibMember(alias, member string) (interpreter.Value, error) {
	var value interpreter.Value
	if vm.env != nil {
		if bound, ok := vm.env.Get(alias); ok {
			value = bound
		}
	}
	if value == nil {
		if fn, ok := interpreter.BuiltinFunctionMap[alias]; ok {
			value = fn
		} else {
			value = interpreter.NULL
		}
	}
	for _, field := range strings.Split(member, ".") {
		resolved, err := vm.executeGetField(value, field)
		if err != nil {
			return nil, err
		}
		value = resolved
	}
	return value, nil
}
