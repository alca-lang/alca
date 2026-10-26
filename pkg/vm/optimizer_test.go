// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package vm_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/vm"
)

func compileAndRun(t *testing.T, src string) string {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		t.Fatalf("syntax errors: %v", p.SyntaxErrors())
	}
	c := vm.NewCompiler()
	if err := c.Compile(prog); err != nil {
		t.Fatalf("compile error: %v", err)
	}
	bc := c.Bytecode()
	m := vm.New(bc)
	if err := m.Run(); err != nil {
		t.Fatalf("run error: %v", err)
	}
	val := m.LastPopped()
	if val == nil {
		return ""
	}
	return val.Inspect()
}

func TestPeepholeOptimizationWithJumps(t *testing.T) {
	// Function containing SetLocal + GetLocal inside branches and loops,
	// exercising peephole optimization with jump target relocation.
	src := `
func test_logic(x: int) -> int:
    a: int = x * 2;
    b: int = a + 3;
    if b > 10:
        c: int = b - 5;
        return c * 2;
    else:
        d: int = b + 5;
        return d * 3;

test_logic(4);
`
	got := compileAndRun(t, src)
	// x=4 -> a=8 -> b=11. 11 > 10 -> c=6 -> return 12.
	if got != "12" {
		t.Fatalf("expected 12, got %s", got)
	}

	src2 := `
func test_logic2(x: int) -> int:
    a: int = x * 2;
    b: int = a + 3;
    if b > 10:
        c: int = b - 5;
        return c * 2;
    else:
        d: int = b + 5;
        return d * 3;

test_logic2(2);
`
	got2 := compileAndRun(t, src2)
	// x=2 -> a=4 -> b=7. 7 <= 10 -> d=12 -> return 36.
	if got2 != "36" {
		t.Fatalf("expected 36, got %s", got2)
	}
}

func TestPeepholeDirectRelocation(t *testing.T) {
	// Construct synthetic bytecode:
	// 0: OpConstant 0 (push 42)      -> 3 bytes
	// 3: OpSetLocal 1                -> 2 bytes
	// 5: OpGetLocal 1                -> 2 bytes (fused with OpSetLocal -> -1 byte)
	// 7: OpJump 11                   -> 3 bytes (jumps past OpPop to OpReturnValue)
	// 10: OpPop                      -> 1 byte
	// 11: OpReturnValue              -> 1 byte
	opt := vm.NewOptimizer()
	ins := vm.Instructions{}
	ins = append(ins, vm.Make(vm.OpConstant, 0)...) // 3 bytes (0..2)
	ins = append(ins, vm.Make(vm.OpSetLocal, 1)...) // 2 bytes (3..4)
	ins = append(ins, vm.Make(vm.OpGetLocal, 1)...) // 2 bytes (5..6)
	ins = append(ins, vm.Make(vm.OpJump, 11)...)    // 3 bytes (7..9)
	ins = append(ins, vm.Make(vm.OpPop)...)         // 1 byte  (10)
	ins = append(ins, vm.Make(vm.OpReturnValue)...) // 1 byte  (11)

	optIns := opt.OptimizePeephole(ins)

	// OpSetLocal(2) + OpGetLocal(2) -> OpDup(1) + OpSetLocal(2) = 3 bytes (saving 1 byte)
	// Total length: 12 - 1 = 11 bytes.
	// OpJump moves from offset 7 to offset 6.
	// OpPop moves from offset 10 to offset 9.
	// OpReturnValue moves from offset 11 to offset 10.
	// OpJump target was 11, should now be relocated to 10!
	if len(optIns) != 11 {
		t.Fatalf("expected optimized length 11, got %d", len(optIns))
	}
	if vm.Opcode(optIns[6]) != vm.OpJump {
		t.Fatalf("expected OpJump at offset 6, got %v", optIns[6])
	}
	target := int(optIns[7])<<8 | int(optIns[8])
	if target != 10 {
		t.Fatalf("expected jump target 10, got %d", target)
	}

	// Also verify Pattern 1: Redundant jump to next instruction is pruned
	insRedundant := vm.Instructions{}
	insRedundant = append(insRedundant, vm.Make(vm.OpConstant, 0)...) // 3 bytes (0..2)
	insRedundant = append(insRedundant, vm.Make(vm.OpJump, 6)...)     // 3 bytes (3..5) jumps to offset 6
	insRedundant = append(insRedundant, vm.Make(vm.OpReturnValue)...) // 1 byte (6)

	optRedundant := opt.OptimizePeephole(insRedundant)
	// Redundant OpJump is removed! Length should be 3 + 1 = 4 bytes!
	if len(optRedundant) != 4 {
		t.Fatalf("expected length 4 after removing redundant jump, got %d", len(optRedundant))
	}
}
