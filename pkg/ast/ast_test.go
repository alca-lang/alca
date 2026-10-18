package ast

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/token"
)

func TestStatements(t *testing.T) {
	// 1. Program
	prog := &Program{
		Token: token.Token{Type: token.IDENT, Literal: "x"},
		Statements: []Statement{
			&VarDeclaration{
				Token: token.Token{Type: token.IDENT, Literal: "x"},
				Name:  &Identifier{Token: token.Token{Type: token.IDENT, Literal: "x"}, Value: "x"},
				TypeAnnotation: &TypeAnnotation{
					Token: token.Token{Type: token.TYPE_INT, Literal: "int"},
					Types: []string{"int"},
				},
				Value: &IntegerLiteral{
					Token: token.Token{Type: token.INT, Literal: "42"},
					Value: 42,
				},
			},
		},
	}
	if prog.TokenLiteral() != "x" {
		t.Errorf("prog.TokenLiteral() = %q, want 'x'", prog.TokenLiteral())
	}
	if !strings.Contains(prog.String(), "x: int = 42") {
		t.Errorf("prog.String() unexpected: %q", prog.String())
	}

	// 2. VarDeclaration (with Decorator)
	varDecl := &VarDeclaration{
		Token: token.Token{Type: token.IDENT, Literal: "counter"},
		Decorators: []*Decorator{
			{
				Token: token.Token{Type: token.AT, Literal: "@"},
				Name:  "export",
			},
		},
		Name: &Identifier{Token: token.Token{Type: token.IDENT, Literal: "counter"}, Value: "counter"},
		Value: &IntegerLiteral{
			Token: token.Token{Type: token.INT, Literal: "1"},
			Value: 1,
		},
	}
	if varDecl.TokenLiteral() != "counter" {
		t.Errorf("varDecl.TokenLiteral() = %q, want 'counter'", varDecl.TokenLiteral())
	}
	if !strings.Contains(varDecl.String(), "@export\ncounter = 1") {
		t.Errorf("varDecl.String() unexpected: %q", varDecl.String())
	}

	// 3. FuncDeclaration & 4. FuncParam
	funcDecl := &FuncDeclaration{
		Token:    token.Token{Type: token.FUNC, Literal: "func"},
		IsPublic: true,
		IsShell:  false,
		Name:     "add",
		Params: []*FuncParam{
			{
				Token:          token.Token{Type: token.IDENT, Literal: "a"},
				Name:           "a",
				TypeAnnotation: &TypeAnnotation{Types: []string{"int"}},
			},
			{
				Token:          token.Token{Type: token.IDENT, Literal: "b"},
				Name:           "b",
				TypeAnnotation: &TypeAnnotation{Types: []string{"int"}},
				DefaultValue:   &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "0"}, Value: 0},
			},
		},
		ReturnType: &TypeAnnotation{Types: []string{"int"}},
		Body: &BlockStatement{
			Token: token.Token{Type: token.LBRACE, Literal: "{"},
			Statements: []Statement{
				&ReturnStatement{
					Token: token.Token{Type: token.RETURN, Literal: "return"},
					Value: &InfixExpression{
						Token:    token.Token{Type: token.PLUS, Literal: "+"},
						Left:     &Identifier{Value: "a"},
						Operator: "+",
						Right:    &Identifier{Value: "b"},
					},
				},
			},
		},
	}
	if funcDecl.TokenLiteral() != "func" {
		t.Errorf("funcDecl.TokenLiteral() = %q, want 'func'", funcDecl.TokenLiteral())
	}
	if !strings.Contains(funcDecl.String(), "pub func add(a: int, b: int = 0) -> int") {
		t.Errorf("funcDecl.String() unexpected: %q", funcDecl.String())
	}

	// Shell function
	shFunc := &FuncDeclaration{
		Token:   token.Token{Type: token.SHFUNC, Literal: "shfunc"},
		IsShell: true,
		Name:    "deploy",
		Body:    &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if !strings.Contains(shFunc.String(), "shfunc deploy()") {
		t.Errorf("shFunc.String() unexpected: %q", shFunc.String())
	}

	// 5. StructDeclaration & 6. StructField
	structDecl := &StructDeclaration{
		Token:    token.Token{Type: token.STRUCT, Literal: "struct"},
		IsPublic: true,
		Name:     "User",
		Fields: []*StructField{
			{
				Token:          token.Token{Type: token.IDENT, Literal: "id"},
				Name:           "id",
				TypeAnnotation: &TypeAnnotation{Types: []string{"int"}},
			},
			{
				Token:          token.Token{Type: token.IDENT, Literal: "name"},
				Name:           "name",
				TypeAnnotation: &TypeAnnotation{Types: []string{"string"}},
				DefaultValue:   &StringLiteral{Token: token.Token{Type: token.STRING, Literal: "anon"}, Value: "anon"},
			},
		},
	}
	if structDecl.TokenLiteral() != "struct" {
		t.Errorf("structDecl.TokenLiteral() = %q, want 'struct'", structDecl.TokenLiteral())
	}
	if !strings.Contains(structDecl.String(), "pub struct User") {
		t.Errorf("structDecl.String() unexpected: %q", structDecl.String())
	}

	// 7. IfStatement & 8. ElifClause
	ifStmt := &IfStatement{
		Token:     token.Token{Type: token.IF, Literal: "if"},
		Condition: &BooleanLiteral{Token: token.Token{Type: token.TRUE, Literal: "true"}, Value: true},
		Consequence: &BlockStatement{
			Token: token.Token{Type: token.LBRACE, Literal: "{"},
		},
		Alternatives: []*ElifClause{
			{
				Token:     token.Token{Type: token.ELIF, Literal: "elif"},
				Condition: &BooleanLiteral{Token: token.Token{Type: token.FALSE, Literal: "false"}, Value: false},
				Body:      &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
			},
		},
		ElseBlock: &BlockStatement{
			Token: token.Token{Type: token.LBRACE, Literal: "{"},
		},
	}
	if ifStmt.TokenLiteral() != "if" {
		t.Errorf("ifStmt.TokenLiteral() = %q, want 'if'", ifStmt.TokenLiteral())
	}
	if !strings.Contains(ifStmt.String(), "if true") || !strings.Contains(ifStmt.String(), "elif false") || !strings.Contains(ifStmt.String(), "else") {
		t.Errorf("ifStmt.String() unexpected: %q", ifStmt.String())
	}

	// 9. WhileStatement
	whileStmt := &WhileStatement{
		Token:     token.Token{Type: token.WHILE, Literal: "while"},
		Condition: &BooleanLiteral{Value: true},
		Body:      &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if whileStmt.TokenLiteral() != "while" || !strings.Contains(whileStmt.String(), "while true") {
		t.Errorf("whileStmt unexpected: %q", whileStmt.String())
	}

	// 10. ForStatement
	forStmt := &ForStatement{
		Token:    token.Token{Type: token.FOR, Literal: "for"},
		Variable: "item",
		Iterable: &Identifier{Value: "items"},
		Body:     &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if forStmt.TokenLiteral() != "for" || !strings.Contains(forStmt.String(), "for item in items") {
		t.Errorf("forStmt unexpected: %q", forStmt.String())
	}

	// 11. ForEachStatement
	forEachStmt := &ForEachStatement{
		Token:    token.Token{Type: token.FOREACH, Literal: "foreach"},
		Variable: "user",
		Iterable: &Identifier{Value: "users"},
		Body:     &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if forEachStmt.TokenLiteral() != "foreach" || !strings.Contains(forEachStmt.String(), "foreach user in users") {
		t.Errorf("forEachStmt unexpected: %q", forEachStmt.String())
	}

	// 12. LoopStatement
	loopStmt := &LoopStatement{
		Token: token.Token{Type: token.LOOP, Literal: "loop"},
		Body:  &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if loopStmt.TokenLiteral() != "loop" || !strings.Contains(loopStmt.String(), "loop") {
		t.Errorf("loopStmt unexpected: %q", loopStmt.String())
	}

	// 13. BlockStatement, 15. BreakStatement, 16. ContinueStatement
	blockStmt := &BlockStatement{
		Token: token.Token{Type: token.LBRACE, Literal: "{"},
		Statements: []Statement{
			&BreakStatement{Token: token.Token{Type: token.BREAK, Literal: "break"}},
			&ContinueStatement{Token: token.Token{Type: token.CONTINUE, Literal: "continue"}},
		},
	}
	if blockStmt.TokenLiteral() != "{" || !strings.Contains(blockStmt.String(), "break") || !strings.Contains(blockStmt.String(), "continue") {
		t.Errorf("blockStmt unexpected: %q", blockStmt.String())
	}

	// 17. ExpressionStatement
	exprStmt := &ExpressionStatement{
		Token:      token.Token{Type: token.INT, Literal: "10"},
		Expression: &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "10"}, Value: 10},
	}
	if exprStmt.TokenLiteral() != "10" || exprStmt.String() != "10" {
		t.Errorf("exprStmt unexpected: %q", exprStmt.String())
	}

	// 18. UseStatement
	useStmt := &UseStatement{
		Token:      token.Token{Type: token.USE, Literal: "use"},
		ModulePath: "net/http",
		Imports:    []string{"Get", "Post"},
		Alias:      "http",
	}
	if useStmt.TokenLiteral() != "use" || !strings.Contains(useStmt.String(), "use net/http [Get, Post] as http") {
		t.Errorf("useStmt unexpected: %q", useStmt.String())
	}

	// 19. WatchStatement
	watchStmt := &WatchStatement{
		Token:  token.Token{Type: token.WATCH, Literal: "watch"},
		Target: &StringLiteral{Value: "./src"},
		Body:   &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if watchStmt.TokenLiteral() != "watch" || !strings.Contains(watchStmt.String(), "watch") {
		t.Errorf("watchStmt unexpected: %q", watchStmt.String())
	}

	// 20. TestStatement
	testStmt := &TestStatement{
		Token: token.Token{Type: token.TEST, Literal: "test"},
		Name:  "calculates sum correctly",
		Body:  &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if testStmt.TokenLiteral() != "test" || !strings.Contains(testStmt.String(), `test "calculates sum correctly"`) {
		t.Errorf("testStmt unexpected: %q", testStmt.String())
	}

	// 21. MockStatement
	mockStmt := &MockStatement{
		Token:       token.Token{Type: token.MOCK, Literal: "mock"},
		Target:      &Identifier{Value: "fetchData"},
		Replacement: &Identifier{Value: "mockFetchData"},
	}
	if mockStmt.TokenLiteral() != "mock" || !strings.Contains(mockStmt.String(), "mock fetchData = mockFetchData") {
		t.Errorf("mockStmt unexpected: %q", mockStmt.String())
	}

	// 22. BaStatement
	baStmt := &BaStatement{
		Token:    token.Token{Type: token.BA, Literal: "ba"},
		Timeout:  &Identifier{Value: "5s"},
		MemLimit: "256MB",
		Limit:    &IntegerLiteral{Value: 10},
		Body:     &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if baStmt.TokenLiteral() != "ba" || !strings.Contains(baStmt.String(), "ba ~ 5s mem=256MB limit=10") {
		t.Errorf("baStmt unexpected: %q", baStmt.String())
	}

	// 23. BaForStatement
	baForStmt := &BaForStatement{
		Token:    token.Token{Type: token.BA, Literal: "ba"},
		Variable: "job",
		Iterable: &Identifier{Value: "jobs"},
		Limit:    &IntegerLiteral{Value: 4},
		Body:     &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if baForStmt.TokenLiteral() != "ba" || !strings.Contains(baForStmt.String(), "ba for job in jobs limit=4") {
		t.Errorf("baForStmt unexpected: %q", baForStmt.String())
	}

	// 24. ThreadsStatement
	threadsStmt := &ThreadsStatement{
		Token: token.Token{Type: token.THREADS, Literal: "threads"},
		Body:  &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if threadsStmt.TokenLiteral() != "threads" || !strings.Contains(threadsStmt.String(), "threads") {
		t.Errorf("threadsStmt unexpected: %q", threadsStmt.String())
	}

	// 25. TxStatement
	txStmt := &TxStatement{
		Token:   token.Token{Type: token.TX, Literal: "tx"},
		DB:      &Identifier{Value: "dbConn"},
		Timeout: &Identifier{Value: "10s"},
		Body:    &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if txStmt.TokenLiteral() != "tx" || !strings.Contains(txStmt.String(), "tx dbConn ~ 10s") {
		t.Errorf("txStmt unexpected: %q", txStmt.String())
	}

	// 26. WaitStatement
	waitStmt := &WaitStatement{Token: token.Token{Type: token.WAIT, Literal: "wait"}}
	if waitStmt.TokenLiteral() != "wait" || waitStmt.String() != "wait" {
		t.Errorf("waitStmt unexpected: %q", waitStmt.String())
	}

	// 27. AssertStatement
	assertStmt := &AssertStatement{
		Token:      token.Token{Type: token.ASSERT, Literal: "assert"},
		Expression: &BooleanLiteral{Value: true},
	}
	if assertStmt.TokenLiteral() != "assert" || !strings.Contains(assertStmt.String(), "assert true") {
		t.Errorf("assertStmt unexpected: %q", assertStmt.String())
	}

	// 28. Decorator
	decorator := &Decorator{
		Token: token.Token{Type: token.AT, Literal: "@"},
		Name:  "route",
		Args:  []Expression{&StringLiteral{Value: "/api/users"}},
	}
	if decorator.TokenLiteral() != "@" || !strings.Contains(decorator.String(), `@route("/api/users")`) {
		t.Errorf("decorator unexpected: %q", decorator.String())
	}

	// 29. MatchStatement & 30. MatchCase
	matchStmt := &MatchStatement{
		Token:   token.Token{Type: token.MATCH, Literal: "match"},
		Subject: &Identifier{Value: "status"},
		Cases: []*MatchCase{
			{
				Token:   token.Token{Type: token.CASE, Literal: "case"},
				Pattern: &IntegerLiteral{Value: 200},
				Body:    &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
			},
		},
		Default: &BlockStatement{Token: token.Token{Type: token.LBRACE, Literal: "{"}},
	}
	if matchStmt.TokenLiteral() != "match" || !strings.Contains(matchStmt.String(), "match status") || !strings.Contains(matchStmt.String(), "case 200") || !strings.Contains(matchStmt.String(), "default") {
		t.Errorf("matchStmt unexpected: %q", matchStmt.String())
	}
}

func TestExpressions(t *testing.T) {
	// 1. Identifier
	ident := &Identifier{Token: token.Token{Type: token.IDENT, Literal: "foo"}, Value: "foo"}
	if ident.TokenLiteral() != "foo" || ident.String() != "foo" {
		t.Errorf("ident unexpected: %q", ident.String())
	}

	// 2. IntegerLiteral
	intLit := &IntegerLiteral{Token: token.Token{Type: token.INT, Literal: "123"}, Value: 123}
	if intLit.TokenLiteral() != "123" || intLit.String() != "123" {
		t.Errorf("intLit unexpected: %q", intLit.String())
	}

	// 3. FloatLiteral
	floatLit := &FloatLiteral{Token: token.Token{Type: token.FLOAT, Literal: "3.14"}, Value: 3.14}
	if floatLit.TokenLiteral() != "3.14" || floatLit.String() != "3.14" {
		t.Errorf("floatLit unexpected: %q", floatLit.String())
	}

	// 4. StringLiteral
	strLit := &StringLiteral{Token: token.Token{Type: token.STRING, Literal: `"hello"`}, Value: "hello"}
	if strLit.TokenLiteral() != `"hello"` || strLit.String() != `"hello"` {
		t.Errorf("strLit unexpected: %q", strLit.String())
	}

	// 5. BooleanLiteral
	boolLit := &BooleanLiteral{Token: token.Token{Type: token.TRUE, Literal: "true"}, Value: true}
	if boolLit.TokenLiteral() != "true" || boolLit.String() != "true" {
		t.Errorf("boolLit unexpected: %q", boolLit.String())
	}

	// 6. NullLiteral
	nullLit := &NullLiteral{Token: token.Token{Type: token.NULL, Literal: "null"}}
	if nullLit.TokenLiteral() != "null" || nullLit.String() != "null" {
		t.Errorf("nullLit unexpected: %q", nullLit.String())
	}

	// 7. ListLiteral
	listLit := &ListLiteral{
		Token: token.Token{Type: token.LBRACKET, Literal: "["},
		Elements: []Expression{
			&IntegerLiteral{Value: 1},
			&IntegerLiteral{Value: 2},
		},
	}
	if listLit.TokenLiteral() != "[" || listLit.String() != "[1, 2]" {
		t.Errorf("listLit unexpected: %q", listLit.String())
	}

	// 8. MapLiteral & MapPair
	mapLit := &MapLiteral{
		Token: token.Token{Type: token.LBRACE, Literal: "{"},
		Pairs: []*MapPair{
			{
				Token: token.Token{Type: token.COLON, Literal: ":"},
				Key:   &StringLiteral{Value: "name"},
				Value: &StringLiteral{Value: "Alca"},
			},
		},
	}
	if mapLit.TokenLiteral() != "{" || mapLit.String() != `{"name": "Alca"}` {
		t.Errorf("mapLit unexpected: %q", mapLit.String())
	}

	// 9. PrefixExpression
	prefix := &PrefixExpression{
		Token:    token.Token{Type: token.MINUS, Literal: "-"},
		Operator: "-",
		Right:    &IntegerLiteral{Value: 5},
	}
	if prefix.TokenLiteral() != "-" || prefix.String() != "(-5)" {
		t.Errorf("prefix unexpected: %q", prefix.String())
	}

	// 10. InfixExpression
	infix := &InfixExpression{
		Token:    token.Token{Type: token.PLUS, Literal: "+"},
		Left:     &IntegerLiteral{Value: 1},
		Operator: "+",
		Right:    &IntegerLiteral{Value: 2},
	}
	if infix.TokenLiteral() != "+" || infix.String() != "(1 + 2)" {
		t.Errorf("infix unexpected: %q", infix.String())
	}

	// 11. CallExpression (with KwArgs)
	call := &CallExpression{
		Token:    token.Token{Type: token.LPAREN, Literal: "("},
		Function: &Identifier{Value: "log"},
		Arguments: []Expression{
			&StringLiteral{Value: "failed"},
		},
		KwArgs: map[string]Expression{
			"code":  &IntegerLiteral{Value: 500},
			"fatal": &BooleanLiteral{Value: true},
		},
	}
	if call.TokenLiteral() != "(" || !strings.Contains(call.String(), `log("failed", code=500, fatal=true)`) {
		t.Errorf("call unexpected: %q", call.String())
	}

	// 12. IndexExpression
	index := &IndexExpression{
		Token: token.Token{Type: token.LBRACKET, Literal: "["},
		Left:  &Identifier{Value: "arr"},
		Index: &IntegerLiteral{Value: 0},
	}
	if index.TokenLiteral() != "[" || index.String() != "(arr[0])" {
		t.Errorf("index unexpected: %q", index.String())
	}

	// 13. DotExpression
	dot := &DotExpression{
		Token: token.Token{Type: token.DOT, Literal: "."},
		Left:  &Identifier{Value: "user"},
		Field: "name",
	}
	if dot.TokenLiteral() != "." || dot.String() != "user.name" {
		t.Errorf("dot unexpected: %q", dot.String())
	}

	// 14. AssignExpression
	assign := &AssignExpression{
		Token:  token.Token{Type: token.ASSIGN, Literal: "="},
		Target: &Identifier{Value: "x"},
		Value:  &IntegerLiteral{Value: 100},
	}
	if assign.TokenLiteral() != "=" || assign.String() != "x = 100" {
		t.Errorf("assign unexpected: %q", assign.String())
	}

	// 15. StreamExpression
	streamOps := []string{">>", ">!", ">>>", "<<", "?>>", "?>>>", "&>>", "&&>>"}
	for _, op := range streamOps {
		stream := &StreamExpression{
			Token:    token.Token{Literal: op},
			Source:   &Identifier{Value: "cmd"},
			Operator: op,
			Target:   &Identifier{Value: "output"},
		}
		if stream.TokenLiteral() != op || stream.String() != "cmd "+op+" output" {
			t.Errorf("stream unexpected for %s: %q", op, stream.String())
		}
	}

	// 16. PipeExpression
	pipe := &PipeExpression{
		Token: token.Token{Type: token.PIPE_ARROW, Literal: "|>"},
		Left:  &Identifier{Value: "data"},
		Right: &Identifier{Value: "process"},
	}
	if pipe.TokenLiteral() != "|>" || pipe.String() != "data |> process" {
		t.Errorf("pipe unexpected: %q", pipe.String())
	}

	// 17. TimeoutExpression
	timeout := &TimeoutExpression{
		Token:      token.Token{Type: token.TILDE, Literal: "~"},
		Expression: &Identifier{Value: "request"},
		Duration:   &Identifier{Value: "2s"},
	}
	if timeout.TokenLiteral() != "~" || timeout.String() != "request ~ 2s" {
		t.Errorf("timeout unexpected: %q", timeout.String())
	}

	// 18. ConsoleLiteral
	console := &ConsoleLiteral{
		Token:  token.Token{Type: token.IDENT, Literal: "$"},
		Prompt: &StringLiteral{Value: "Enter name: "},
	}
	if console.TokenLiteral() != "$" || console.String() != `$("Enter name: ")` {
		t.Errorf("console unexpected: %q", console.String())
	}

	// 19. LambdaExpression
	lambda := &LambdaExpression{
		Token: token.Token{Type: token.ARROW, Literal: "->"},
		Params: []*FuncParam{
			{Name: "x", TypeAnnotation: &TypeAnnotation{Types: []string{"int"}}},
		},
		Body: &InfixExpression{
			Left:     &Identifier{Value: "x"},
			Operator: "*",
			Right:    &IntegerLiteral{Value: 2},
		},
	}
	if lambda.TokenLiteral() != "->" || !strings.Contains(lambda.String(), "(x: int) -> (x * 2)") {
		t.Errorf("lambda unexpected: %q", lambda.String())
	}

	// 20. TypeAnnotation
	typeAnn := &TypeAnnotation{
		Token: token.Token{Type: token.TYPE_INT, Literal: "int"},
		Types: []string{"int", "string"},
	}
	if typeAnn.TokenLiteral() != "int" || typeAnn.String() != "int | string" {
		t.Errorf("typeAnn unexpected: %q", typeAnn.String())
	}

	// 21. ErrExpression
	errExpr := &ErrExpression{
		Token:   token.Token{Type: token.ERR, Literal: "err"},
		Message: &StringLiteral{Value: "not found"},
		Code:    &IntegerLiteral{Value: 404},
	}
	if errExpr.TokenLiteral() != "err" || errExpr.String() != `err("not found", code=404)` {
		t.Errorf("errExpr unexpected: %q", errExpr.String())
	}

	// 22. RangeLiteral
	rng := &RangeLiteral{
		Token: token.Token{Type: token.RANGE, Literal: ".."},
		Start: &IntegerLiteral{Value: 0},
		End:   &IntegerLiteral{Value: 10},
	}
	if rng.TokenLiteral() != ".." || rng.String() != "0..10" {
		t.Errorf("rng unexpected: %q", rng.String())
	}

	// 23. StructInstantiation
	structInst := &StructInstantiation{
		Token: token.Token{Type: token.IDENT, Literal: "User"},
		Name:  "User",
		Args: []Expression{
			&IntegerLiteral{Value: 1},
			&StringLiteral{Value: "Alice"},
		},
	}
	if structInst.TokenLiteral() != "User" || structInst.String() != `User(1, "Alice")` {
		t.Errorf("structInst unexpected: %q", structInst.String())
	}
}
