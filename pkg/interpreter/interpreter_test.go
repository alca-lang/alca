package interpreter

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/token"
)

func TestValuesAndHelpers(t *testing.T) {
	// Truthiness
	if IsTruthy(NULL) {
		t.Errorf("NULL should be falsy")
	}
	if IsTruthy(FALSE) {
		t.Errorf("FALSE should be falsy")
	}
	if IsTruthy(&IntegerValue{Val: 0}) {
		t.Errorf("0 should be falsy")
	}
	if IsTruthy(&FloatValue{Val: 0.0}) {
		t.Errorf("0.0 should be falsy")
	}
	if IsTruthy(&StringValue{Val: ""}) {
		t.Errorf("\"\" should be falsy")
	}
	if !IsTruthy(TRUE) {
		t.Errorf("TRUE should be truthy")
	}
	if !IsTruthy(&IntegerValue{Val: 42}) {
		t.Errorf("42 should be truthy")
	}
	if !IsTruthy(&StringValue{Val: "alca"}) {
		t.Errorf("\"alca\" should be truthy")
	}

	// Equality
	if !ValuesEqual(&IntegerValue{Val: 5}, &FloatValue{Val: 5.0}) {
		t.Errorf("5 should equal 5.0")
	}
	if ValuesEqual(&IntegerValue{Val: 5}, &IntegerValue{Val: 6}) {
		t.Errorf("5 should not equal 6")
	}
	if !ValuesEqual(&StringValue{Val: "abc"}, &StringValue{Val: "abc"}) {
		t.Errorf("string equality failed")
	}

	// List
	l1 := NewList(&IntegerValue{Val: 1}, &StringValue{Val: "x"})
	l2 := l1.Clone().(*ListValue)
	if !ValuesEqual(l1, l2) {
		t.Errorf("cloned list should equal original")
	}

	// Map
	m := NewMap()
	m.Set("k1", &IntegerValue{Val: 100})
	m.Set("k2", &StringValue{Val: "test"})
	if !m.Has("k1") || m.Len() != 2 {
		t.Errorf("map operations failed")
	}
	mCopy := m.Clone().(*MapValue)
	if !ValuesEqual(m, mCopy) {
		t.Errorf("cloned map should equal original")
	}

	// Secret
	sec := &SecretValue{Val: "super_secret"}
	if sec.Inspect() != "***REDACTED***" {
		t.Errorf("secret inspection should redact")
	}
}

func TestEnvironment(t *testing.T) {
	env := NewEnvironment()
	env.Set("x", &IntegerValue{Val: 10})
	env.SetConst("PI", &FloatValue{Val: 3.14})

	val, ok := env.Get("x")
	if !ok || val.(*IntegerValue).Val != 10 {
		t.Fatalf("failed to retrieve x")
	}

	// Update x
	if _, updated := env.Update("x", &IntegerValue{Val: 20}); !updated {
		t.Fatalf("failed to update x")
	}
	val, _ = env.Get("x")
	if val.(*IntegerValue).Val != 20 {
		t.Fatalf("expected x=20, got %v", val)
	}

	// Constants cannot be updated
	if _, updated := env.Update("PI", &FloatValue{Val: 3.0}); updated {
		t.Fatalf("should not update constant")
	}

	// Enclosed scope
	child := NewEnclosedEnvironment(env)
	child.Set("y", &StringValue{Val: "child"})
	if v, ok := child.Get("x"); !ok || v.(*IntegerValue).Val != 20 {
		t.Fatalf("child should resolve parent x")
	}

	// Delete
	env.Delete("x")
	if _, ok := env.Get("x"); ok {
		t.Fatalf("x should have been deleted")
	}
}

func TestBuiltins(t *testing.T) {
	// len
	lenFn := BuiltinFunctionMap["len"].Fn
	if l := lenFn(&StringValue{Val: "hello"}).(*IntegerValue).Val; l != 5 {
		t.Errorf("len(hello) expected 5, got %d", l)
	}
	if l := lenFn(NewList(&IntegerValue{Val: 1}, &IntegerValue{Val: 2})).(*IntegerValue).Val; l != 2 {
		t.Errorf("len(list) expected 2, got %d", l)
	}

	// type
	typeFn := BuiltinFunctionMap["type"].Fn
	if s := typeFn(&IntegerValue{Val: 1}).(*StringValue).Val; s != "INTEGER" {
		t.Errorf("type(1) expected INTEGER, got %s", s)
	}

	// range
	rangeFn := BuiltinFunctionMap["range"].Fn
	rList := rangeFn(&IntegerValue{Val: 1}, &IntegerValue{Val: 5}).(*ListValue)
	if len(rList.Elements) != 5 {
		t.Errorf("range(1, 5) len expected 5, got %d", len(rList.Elements))
	}

	// json
	encodeFn := BuiltinFunctionMap["json_encode"].Fn
	decodeFn := BuiltinFunctionMap["json_decode"].Fn

	m := NewMap()
	m.Set("name", &StringValue{Val: "alca"})
	m.Set("version", &IntegerValue{Val: 1})

	encoded := encodeFn(m).(*StringValue)
	decoded := decodeFn(encoded).(*MapValue)

	nameVal, _ := decoded.Get("name")
	if nameVal.(*StringValue).Val != "alca" {
		t.Errorf("json roundtrip failed for name")
	}
}

func TestMethodCalls(t *testing.T) {
	str := &StringValue{Val: " hello,world "}

	trimRes, err := CallMethod(str, "trim", nil)
	if err != nil || trimRes.(*StringValue).Val != "hello,world" {
		t.Fatalf(".trim() failed: %v", err)
	}

	splitRes, err := CallMethod(trimRes, "split", []Value{&StringValue{Val: ","}})
	if err != nil || splitRes.(*ListValue).Len() != 2 {
		t.Fatalf(".split() failed: %v", err)
	}

	list := NewList(&IntegerValue{Val: 3}, &IntegerValue{Val: 1}, &IntegerValue{Val: 2})
	sortRes, err := CallMethod(list, "sort", nil)
	if err != nil || sortRes.(*ListValue).Elements[0].(*IntegerValue).Val != 1 {
		t.Fatalf(".sort() failed: %v", err)
	}

	joinRes, err := CallMethod(sortRes, "join", []Value{&StringValue{Val: "-"}})
	if err != nil || joinRes.(*StringValue).Val != "1-2-3" {
		t.Fatalf(".join() failed: %v", err)
	}
}

func TestStreamOperations(t *testing.T) {
	env := NewEnvironment()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Overwrite >!
	res, err := HandleStream(&StringValue{Val: "initial data"}, ">!", &StringValue{Val: filePath}, env)
	if err != nil || res.(*StringValue).Val != "initial data" {
		t.Fatalf("HandleStream >! failed: %v", err)
	}

	// Append >>
	_, err = HandleStream(&StringValue{Val: "more data"}, ">>", &StringValue{Val: filePath}, env)
	if err != nil {
		t.Fatalf("HandleStream >> failed: %v", err)
	}

	// Read <<
	readVal, err := HandleStream(nil, "<<", &StringValue{Val: filePath}, env)
	if err != nil {
		t.Fatalf("HandleStream << failed: %v", err)
	}
	content := readVal.(*StringValue).Val
	if !strings.Contains(content, "initial data") || !strings.Contains(content, "more data") {
		t.Fatalf("stream content unexpected: %q", content)
	}

	// Stream to var >>>
	_, err = HandleStream(&IntegerValue{Val: 999}, ">>>", &StringValue{Val: "storedVar"}, env)
	if err != nil {
		t.Fatalf(">>> failed: %v", err)
	}
	v, ok := env.Get("storedVar")
	if !ok || v.(*IntegerValue).Val != 999 {
		t.Fatalf("expected storedVar=999, got %v", v)
	}

	// Stream discard >> null
	resNull, err := HandleStream(&StringValue{Val: "discard"}, ">>", &StringValue{Val: "null"}, env)
	if err != nil || resNull.Type() != NULL_VAL {
		t.Fatalf(">> null failed")
	}
}

func TestInterpreterBasicExecution(t *testing.T) {
	interp := New()

	prog := &ast.Program{
		Statements: []ast.Statement{
			// a = 10
			&ast.VarDeclaration{
				Name:  &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "a"}, Value: "a"},
				Value: &ast.IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10},
			},
			// b = 20
			&ast.VarDeclaration{
				Name:  &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "b"}, Value: "b"},
				Value: &ast.IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "20"}, Value: 20},
			},
			// sum = a + b
			&ast.VarDeclaration{
				Name: &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "sum"}, Value: "sum"},
				Value: &ast.InfixExpression{
					Left:     &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "a"}, Value: "a"},
					Operator: "+",
					Right:    &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "b"}, Value: "b"},
				},
			},
		},
	}

	err := interp.Execute(prog)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}

	sumVal, ok := interp.env.Get("sum")
	if !ok || sumVal.(*IntegerValue).Val != 30 {
		t.Fatalf("expected sum=30, got %v", sumVal)
	}
}

func TestInterpreterControlFlow(t *testing.T) {
	interp := New()

	// While loop counting up to 5
	prog := &ast.Program{
		Statements: []ast.Statement{
			&ast.VarDeclaration{
				Name:  &ast.Identifier{Token: token.Token{Type: token.IDENT, Literal: "cnt"}, Value: "cnt"},
				Value: &ast.IntegerLiteral{Value: 0},
			},
			&ast.WhileStatement{
				Condition: &ast.InfixExpression{
					Left:     &ast.Identifier{Value: "cnt"},
					Operator: "<",
					Right:    &ast.IntegerLiteral{Value: 5},
				},
				Body: &ast.BlockStatement{
					Statements: []ast.Statement{
						&ast.ExpressionStatement{
							Expression: &ast.AssignExpression{
								Target: &ast.Identifier{Value: "cnt"},
								Value: &ast.InfixExpression{
									Left:     &ast.Identifier{Value: "cnt"},
									Operator: "+",
									Right:    &ast.IntegerLiteral{Value: 1},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := interp.Execute(prog); err != nil {
		t.Fatalf("while execution failed: %v", err)
	}
	cntVal, _ := interp.env.Get("cnt")
	if cntVal.(*IntegerValue).Val != 5 {
		t.Fatalf("expected cnt=5, got %v", cntVal)
	}
}

func TestInterpreterFunctionsAndClosures(t *testing.T) {
	interp := New()

	// func add(x, y) { return x + y }
	prog := &ast.Program{
		Statements: []ast.Statement{
			&ast.FuncDeclaration{
				Name: "add",
				Params: []*ast.FuncParam{
					{Name: "x"},
					{Name: "y"},
				},
				Body: &ast.BlockStatement{
					Statements: []ast.Statement{
						&ast.ReturnStatement{
							Value: &ast.InfixExpression{
								Left:     &ast.Identifier{Value: "x"},
								Operator: "+",
								Right:    &ast.Identifier{Value: "y"},
							},
						},
					},
				},
			},
			&ast.VarDeclaration{
				Name: &ast.Identifier{Value: "res"},
				Value: &ast.CallExpression{
					Function: &ast.Identifier{Value: "add"},
					Arguments: []ast.Expression{
						&ast.IntegerLiteral{Value: 15},
						&ast.IntegerLiteral{Value: 27},
					},
				},
			},
		},
	}

	if err := interp.Execute(prog); err != nil {
		t.Fatalf("function execution failed: %v", err)
	}

	resVal, _ := interp.env.Get("res")
	if resVal.(*IntegerValue).Val != 42 {
		t.Fatalf("expected res=42, got %v", resVal)
	}
}

// parseTestProgram parses Alca source for interpreter-level tests.
func parseTestProgram(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if errs := p.SyntaxErrors(); len(errs) > 0 {
		t.Fatalf("syntax errors: %v", errs)
	}
	return prog
}

// TestReturningFunctionValueWithFuncAnnotation: a function annotated "-> func"
// may return a lambda or a nested function. Regression: the runtime return-type
// check did not recognise function values for the "func" annotation and failed
// with "invalid return type: expected func, got FUNCTION" (code 400), so no
// function could ever return a closure.
func TestReturningFunctionValueWithFuncAnnotation(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int64
	}{
		{
			name: "lambda",
			src: strings.Join([]string{
				"func make_adder(n: int) -> func:",
				"    return |x: int| x + n;",
				"add5 := make_adder(5);",
				"res := add5(2);",
			}, "\n"),
			want: 7,
		},
		{
			name: "nested function",
			src: strings.Join([]string{
				"func make_counter() -> func:",
				"    n := 0;",
				"    func next() -> int:",
				"        n = n + 1;",
				"        return n;",
				"    return next;",
				"c := make_counter();",
				"c();",
				"res := c();",
			}, "\n"),
			want: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			interp := New()
			if err := interp.Execute(parseTestProgram(t, tc.src)); err != nil {
				t.Fatalf("execution error: %v", err)
			}
			res, ok := interp.env.Get("res")
			if !ok {
				t.Fatalf("res is not bound")
			}
			iv, ok := res.(*IntegerValue)
			if !ok || iv.Val != tc.want {
				t.Fatalf("res = %v, want %d", res, tc.want)
			}
		})
	}
}

// TestVoidFunctionEndingInExpressionStatement: a '-> void' function may end
// with an expression statement such as println. Falling off the end of a body
// yields null, it is not an implicit return of the last expression. Regression:
// validateType rejected the println string with
// "invalid return type: expected void, got STRING" (code 400), so no void
// helper could end in a call.
func TestVoidFunctionEndingInExpressionStatement(t *testing.T) {
	src := strings.Join([]string{
		"func log(msg: string) -> void:",
		"    println(msg);",
		"func counter() -> void:",
		"    n := 1;",
		"log(\"hi\");",
		"counter();",
		"println(\"done\");",
	}, "\n")

	var out strings.Builder
	SetPrintHook(func(s string) { out.WriteString(s) })
	defer ResetPrintHook()

	interp := New()
	if err := interp.Execute(parseTestProgram(t, src)); err != nil {
		t.Fatalf("execution error: %v", err)
	}
	if got, want := out.String(), "hidone"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestInterpreterStructs(t *testing.T) {
	interp := New()

	// struct Point { x: int, y: int }
	prog := &ast.Program{
		Statements: []ast.Statement{
			&ast.StructDeclaration{
				Name: "Point",
				Fields: []*ast.StructField{
					{Name: "x", DefaultValue: &ast.IntegerLiteral{Value: 0}},
					{Name: "y", DefaultValue: &ast.IntegerLiteral{Value: 0}},
				},
			},
			&ast.VarDeclaration{
				Name: &ast.Identifier{Value: "pt"},
				Value: &ast.CallExpression{
					Function: &ast.Identifier{Value: "Point"},
					Arguments: []ast.Expression{
						&ast.IntegerLiteral{Value: 12},
						&ast.IntegerLiteral{Value: 34},
					},
				},
			},
		},
	}

	if err := interp.Execute(prog); err != nil {
		t.Fatalf("struct execution failed: %v", err)
	}

	ptVal, _ := interp.env.Get("pt")
	inst, ok := ptVal.(*StructInstance)
	if !ok || inst.Fields["x"].(*IntegerValue).Val != 12 || inst.Fields["y"].(*IntegerValue).Val != 34 {
		t.Fatalf("struct instance mismatch: %v", ptVal)
	}
}

func TestInterpreterShellFunc(t *testing.T) {
	interp := New()

	// shfunc test_echo() { print("hello shell") }
	echoCmd := "echo hello shell"
	shFn := &FuncValue{
		Name:    "test_echo",
		IsShell: true,
		Body: &ast.BlockStatement{
			Statements: []ast.Statement{
				&ast.ExpressionStatement{
					Expression: &ast.CallExpression{
						Function: &ast.Identifier{Value: "print"},
						Arguments: []ast.Expression{
							&ast.StringLiteral{Value: echoCmd},
						},
					},
				},
			},
		},
		Env: interp.env,
	}

	res := interp.executeShellFunc(shFn, nil)
	outStr := res.(*StringValue).Val
	if !strings.Contains(outStr, "hello shell") {
		t.Fatalf("shell output unexpected: %q", outStr)
	}
}

func TestInstantMemoryReleaseOnNull(t *testing.T) {
	interp := New()

	prog := &ast.Program{
		Statements: []ast.Statement{
			&ast.VarDeclaration{
				Name:  &ast.Identifier{Value: "tempData"},
				Value: &ast.StringLiteral{Value: "large data allocated"},
			},
			&ast.ExpressionStatement{
				Expression: &ast.AssignExpression{
					Target: &ast.Identifier{Value: "tempData"},
					Value:  &ast.NullLiteral{},
				},
			},
		},
	}

	if err := interp.Execute(prog); err != nil {
		t.Fatalf("memory release script failed: %v", err)
	}

	if _, exists := interp.env.Get("tempData"); exists {
		t.Fatalf("tempData should be deleted from env after null assignment")
	}
}

func TestMapMethodCallAndModuleDispatch(t *testing.T) {
	interp := New()

	myMod := NewMap()
	myMod.Set("greet", &BuiltinFunction{
		Name: "greet",
		Fn: func(args ...Value) Value {
			name := "World"
			if len(args) > 0 {
				name = CoerceToString(args[0])
			}
			return &StringValue{Val: "Hello, " + name}
		},
	})
	interp.env.Set("std.my_mod", myMod)

	// Test 1: loadStdModule with use statement
	useStmt := &ast.UseStatement{ModulePath: "std.my_mod"}
	interp.Eval(useStmt)

	// Test 2: Call my_mod.greet("Alca")
	callExpr := &ast.CallExpression{
		Function: &ast.DotExpression{
			Left:  &ast.Identifier{Value: "my_mod"},
			Field: "greet",
		},
		Arguments: []ast.Expression{
			&ast.StringLiteral{Value: "Alca"},
		},
	}
	res := interp.Eval(callExpr)
	if sVal, ok := res.(*StringValue); !ok || sVal.Val != "Hello, Alca" {
		t.Fatalf("expected 'Hello, Alca', got %v", res)
	}

	// Test 3: Direct call via imported symbol
	directCall := &ast.CallExpression{
		Function: &ast.Identifier{Value: "greet"},
		Arguments: []ast.Expression{
			&ast.StringLiteral{Value: "Direct"},
		},
	}
	res2 := interp.Eval(directCall)
	if sVal, ok := res2.(*StringValue); !ok || sVal.Val != "Hello, Direct" {
		t.Fatalf("expected 'Hello, Direct', got %v", res2)
	}
}
