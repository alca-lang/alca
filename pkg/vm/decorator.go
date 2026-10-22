package vm

import (
	"fmt"
	"os"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/limits"
)

// DecoratorSpec describes a decorator that is applied when the declaration it
// belongs to is executed. The tree-walking interpreter understands the same
// set: @oom_guard installs a memory limit, @error_log writes the errors of the
// program to a file, and @drop_on_oom releases a variable when the limit is
// reached. Decorators without an effect are ignored on both engines.
type DecoratorSpec struct {
	Name string
	Args []string
	// Slot is the global slot of the variable a decorator applies to, or -1
	// when the decorator is not bound to a variable.
	Slot int
}

// Type returns STRUCT_DEF so a decorator can be stored like any other
// definition value.
func (d *DecoratorSpec) Type() interpreter.ValueType { return interpreter.STRUCT_DEF }

// Inspect returns a short description of the decorator.
func (d *DecoratorSpec) Inspect() string { return fmt.Sprintf("<@%s>", d.Name) }

// Clone returns the spec itself; specs are immutable.
func (d *DecoratorSpec) Clone() interpreter.Value { return d }

// applyDecorator applies a decorator to the running program.
func (vm *VM) applyDecorator(spec *DecoratorSpec) {
	if spec == nil {
		return
	}
	switch spec.Name {
	case "oom_guard":
		for _, arg := range spec.Args {
			if !strings.HasPrefix(arg, "max=") {
				continue
			}
			size, err := limits.ParseSize(strings.TrimPrefix(arg, "max="))
			if err != nil || size <= 0 {
				continue
			}
			// A program may tighten the limit of its host, but it must not
			// raise it.
			liveLimit, allocLimit := limits.Limits()
			if liveLimit == 0 || size < liveLimit {
				limits.SetLimits(size, allocLimit)
			}
		}
	case "error_log":
		if len(spec.Args) > 0 {
			vm.errorLog = spec.Args[0]
		}
	case "drop_on_oom":
		if spec.Slot >= 0 {
			if vm.dropOnOOM == nil {
				vm.dropOnOOM = map[int]bool{}
			}
			vm.dropOnOOM[spec.Slot] = true
		}
	}
}

// logError appends an error to the error log of the program, if one is set.
// The same error is never written twice.
func (vm *VM) logError(errVal *interpreter.ErrorValue) {
	if vm.errorLog == "" || errVal == nil {
		return
	}
	if vm.loggedError == errVal {
		return
	}
	vm.loggedError = errVal
	entry := fmt.Sprintf("[%s] Error(%d): %s\n", errVal.Time, errVal.Code, errVal.Message)
	_ = os.WriteFile(vm.errorLog, []byte(entry), 0o644)
}

// dropOOMVariables releases the variables that were marked with @drop_on_oom.
func (vm *VM) dropOOMVariables() {
	if len(vm.dropOnOOM) == 0 {
		return
	}
	for slot := range vm.dropOnOOM {
		if slot >= 0 && slot < len(vm.globals) {
			vm.globals[slot] = StackSlot{tag: SlotNull}
		}
	}
}

// txSnapshot holds the local slots of a frame at the start of a transaction.
type txSnapshot struct {
	frameIdx int
	base     int
	locals   []StackSlot
}

// beginTransaction snapshots the local slots of the current frame.
func (vm *VM) beginTransaction(frame *Frame, bp int) {
	count := 0
	if frame != nil && frame.fn != nil {
		count = frame.fn.NumLocals
	}
	if count < 0 {
		count = 0
	}
	snapshot := txSnapshot{frameIdx: vm.framesIndex - 1, base: bp}
	if count > 0 && bp >= 0 && bp+count <= len(vm.stack) {
		snapshot.locals = append(snapshot.locals, vm.stack[bp:bp+count]...)
	}
	vm.txStack = append(vm.txStack, snapshot)
}

// commitTransaction discards the snapshot of the innermost transaction, so that
// the changes of the block stay.
func (vm *VM) commitTransaction() {
	if len(vm.txStack) == 0 {
		return
	}
	vm.txStack = vm.txStack[:len(vm.txStack)-1]
}

// rollbackTransactions restores the snapshots of the transactions that are
// abandoned when an error unwinds to a handler in the given frame.
func (vm *VM) rollbackTransactions(handlerFrame int) {
	for len(vm.txStack) > 0 {
		snapshot := vm.txStack[len(vm.txStack)-1]
		if snapshot.frameIdx < handlerFrame {
			return
		}
		vm.txStack = vm.txStack[:len(vm.txStack)-1]
		if len(snapshot.locals) == 0 {
			continue
		}
		if snapshot.base < 0 || snapshot.base+len(snapshot.locals) > len(vm.stack) {
			continue
		}
		copy(vm.stack[snapshot.base:snapshot.base+len(snapshot.locals)], snapshot.locals)
	}
}
