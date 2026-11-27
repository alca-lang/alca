// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "lexer/lexer.h"
#include "microtest/microtest.h"
#include "parser/parser.h"
#include "typecheck/typecheck.h"
#include "ir/ir.h"
#include "ir/lower.h"

using namespace ulang;

MT_TEST(ir, simple_arithmetic) {
    std::string source = "x := 10 + 20\n";
    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("const_int 10") != std::string::npos);
    MT_CHECK(irStr.find("const_int 20") != std::string::npos);
    MT_CHECK(irStr.find("add") != std::string::npos);
}

MT_TEST(ir, functions_and_calls) {
    std::string source = 
        "func add(a: int, b: int) -> int {\n"
        "    return a + b\n"
        "}\n"
        "res := add(5, 7)\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("fn @add(v0:i64, v1:i64) -> i64") != std::string::npos);
    MT_CHECK(irStr.find("call @add(v") != std::string::npos);
}

MT_TEST(ir, control_flow) {
    std::string source = 
        "x := 1\n"
        "if x > 0 {\n"
        "    x = x + 1\n"
        "} else {\n"
        "    x = 0\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("branch") != std::string::npos);
    MT_CHECK(irStr.find("if_then") != std::string::npos);
    MT_CHECK(irStr.find("if_else") != std::string::npos);
    MT_CHECK(irStr.find("if_merge") != std::string::npos);
}

MT_TEST(ir, string_equality_uses_runtime_helper) {
    std::string source =
        "a := \"hello\"\n"
        "b := \"hel\" + \"lo\"\n"
        "same := a == b\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    MT_CHECK(irRes.program->toString().find("call @ulang_str_eq") != std::string::npos);
}

MT_TEST(ir, list_equality_uses_element_loop) {
    std::string source =
        "a := [1, 2, 3]\n"
        "b := [1, 2] + [3]\n"
        "same := a == b\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("list_eq_cond") != std::string::npos);
    MT_CHECK(irStr.find("list_eq_elem") != std::string::npos);
}

MT_TEST(ir, assert_condition_is_lowered) {
    std::string source =
        "values := [1, 2, 3]\n"
        "assert len(values) == 3\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    MT_CHECK(irRes.program->toString().find("assert_fail") != std::string::npos);
}

MT_TEST(ir, math_module_lowering) {
    std::string source =
        "use std.math\n"
        "a := math.sqrt(16.0)\n"
        "b := math.pow(2.0, 10.0)\n"
        "c := math.clamp(15, 0, 10)\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("sqrt") != std::string::npos);
    MT_CHECK(irStr.find("pow") != std::string::npos);
    MT_CHECK(irStr.find("clamp_high") != std::string::npos);
}

MT_TEST(ir, baremetal_os_intrinsics) {
    std::string source =
        "use std.cpu\n"
        "use std.mem\n"
        "cpu.cli()\n"
        "cpu.out8(1016, 65)\n"
        "b := cpu.in8(1016)\n"
        "mem.write_u16(753664, 3905)\n"
        "v := mem.read_u16(753664)\n"
        "cpu.sti()\n"
        "cpu.hlt()\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("os.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);
    MT_CHECK(irRes.program != nullptr);
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("cpu_cli") != std::string::npos);
    MT_CHECK(irStr.find("out8") != std::string::npos);
    MT_CHECK(irStr.find("in8") != std::string::npos);
    MT_CHECK(irStr.find("raw_store16") != std::string::npos);
    MT_CHECK(irStr.find("raw_load16") != std::string::npos);
    MT_CHECK(irStr.find("cpu_sti") != std::string::npos);
    MT_CHECK(irStr.find("cpu_hlt") != std::string::npos);
}

MT_TEST(ir, baremetal_extended_and_module_constants) {
    std::string source =
        "use std.cpu\n"
        "BASE := 1016\n"
        "func out_char() {\n"
        "    cpu.out8(BASE, 65)\n"
        "}\n"
        "cpu.cli()\n"
        "r := cpu.rdmsr(3221225472)\n"
        "cpu.wrmsr(3221225472, r)\n"
        "a := cpu.cpuid_eax(1, 0)\n"
        "cpu.lidt(524272)\n"
        "x := 1 << 3\n"
        "cpu.iret()\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("os2.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    MT_CHECK(irRes.program != nullptr);
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("rdmsr") != std::string::npos);
    MT_CHECK(irStr.find("wrmsr") != std::string::npos);
    MT_CHECK(irStr.find("cpuid") != std::string::npos);
    MT_CHECK(irStr.find("lidt") != std::string::npos);
    MT_CHECK(irStr.find("iret") != std::string::npos);
    MT_CHECK(irStr.find("shl") != std::string::npos);
}

MT_TEST(ir, mutable_globals) {
    std::string source =
        "count := 0\n"
        "func bump() {\n"
        "    count = count + 1\n"
        "}\n"
        "func main() {\n"
        "    bump()\n"
        "    count = 5\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("globals.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    MT_CHECK_EQ(irRes.program->globals.size(), 1u);
    MT_CHECK_EQ(irRes.program->globals[0].name, "count");
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("global @count") != std::string::npos);
    MT_CHECK(irStr.find("load_global @count") != std::string::npos);
    MT_CHECK(irStr.find("store_global @count") != std::string::npos);
}

MT_TEST(ir, function_addresses_and_indirect_calls) {
    std::string source =
        "func callback() {\n"
        "    println(\"hi\")\n"
        "}\n"
        "func main() {\n"
        "    f := callback\n"
        "    f()\n"
        "    table := [callback, callback]\n"
        "    table[0]()\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("callbacks.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("func_addr @callback") != std::string::npos);
    MT_CHECK(irStr.find("call_ptr") != std::string::npos);
}

MT_TEST(ir, generics_and_overload_monomorphization) {
    std::string source =
        "func identity[T](x: T) -> T {\n"
        "    return x\n"
        "}\n"
        "func twice(a: int) -> int {\n"
        "    return a + a\n"
        "}\n"
        "func twice(a: float) -> float {\n"
        "    return a + a\n"
        "}\n"
        "func main() {\n"
        "    a := identity(42)\n"
        "    b := identity(3.5)\n"
        "    c := twice(2)\n"
        "    d := twice(2.5)\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("generics.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("call @identity_int") != std::string::npos);
    MT_CHECK(irStr.find("call @identity_float") != std::string::npos);
    MT_CHECK(irStr.find("call @twice_int") != std::string::npos);
    MT_CHECK(irStr.find("call @twice_float") != std::string::npos);
    MT_CHECK(irStr.find("fn @identity_int") != std::string::npos);
    MT_CHECK(irStr.find("fn @identity_float") != std::string::npos);
}

MT_TEST(ir, defer_is_lowered_on_every_return) {
    std::string source =
        "func f() -> int {\n"
        "    defer println(\"CLEANUP\")\n"
        "    return 42\n"
        "}\n"
        "func main() {\n"
        "    x := f()\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("defer.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    std::string irStr = irRes.program->toString();
    size_t cleanup = irStr.find("println");
    size_t ret = irStr.find("ret v0");
    MT_CHECK(cleanup != std::string::npos);
    MT_CHECK(ret != std::string::npos);
    MT_CHECK(cleanup < ret); // deferred call emitted before returning
}

MT_TEST(ir, implicit_destructors_on_return) {
    std::string source =
        "struct A {\n"
        "    id: int\n"
        "}\n"
        "func ~A(self: A) {\n"
        "    println(\"DTOR\")\n"
        "}\n"
        "func f() -> int {\n"
        "    a := A(1)\n"
        "    b := A(2)\n"
        "    return 7\n"
        "}\n"
        "func main() {\n"
        "    x := f()\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("dtor.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    std::string irStr = irRes.program->toString();
    size_t first = irStr.find("call @A");
    MT_CHECK(first != std::string::npos);
    MT_CHECK(irStr.find("call @A", first + 1) != std::string::npos);
}

MT_TEST(ir, exceptions_with_unwinding) {
    std::string source =
        "struct A {\n"
        "    id: int\n"
        "}\n"
        "func ~A(self: A) {\n"
        "    println(\"DTOR\")\n"
        "}\n"
        "func raise() {\n"
        "    a := A(1)\n"
        "    throw \"boom\"\n"
        "}\n"
        "func main() {\n"
        "    try {\n"
        "        raise()\n"
        "    } catch (e) {\n"
        "        println(\"CAUGHT\")\n"
        "    }\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("exceptions.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("set_err") != std::string::npos);
    MT_CHECK(irStr.find("clear_err") != std::string::npos);
    MT_CHECK(irStr.find("get_err") != std::string::npos);
    MT_CHECK(irStr.find("call @A") != std::string::npos); // destructor during unwind
}

MT_TEST(ir, constexpr_folded_without_runtime_arithmetic) {
    std::string source =
        "constexpr N := 2 + 3 * 4\n"
        "constexpr M := 1 << 4\n"
        "func main() {\n"
        "    x := N + M\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("constexpr.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK(irRes.ok());
    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("const_int 14") != std::string::npos);
    MT_CHECK(irStr.find("const_int 16") != std::string::npos);
    MT_CHECK(irStr.find("global @N") == std::string::npos);
}

MT_TEST(ir, list_and_for_loop) {
    std::string source = 
        "total := 0\n"
        "for i in 1..5 {\n"
        "    total = total + i\n"
        "}\n";

    auto lex = std::make_unique<lexer::Lexer>(source);
    parser::Parser p(std::move(lex));
    auto astProg = p.parseProgram();
    MT_CHECK_EQ(p.syntaxErrors().size(), 0u);

    typecheck::Checker tc("test.al", source);
    auto errs = tc.check(astProg);
    MT_CHECK_EQ(errs.size(), 0u);

    auto irRes = ir::lower(astProg, tc);
    MT_CHECK_EQ(irRes.ok(), true);

    std::string irStr = irRes.program->toString();
    MT_CHECK(irStr.find("for_cond") != std::string::npos);
    MT_CHECK(irStr.find("for_body") != std::string::npos);
    MT_CHECK(irStr.find("for_step") != std::string::npos);
}
