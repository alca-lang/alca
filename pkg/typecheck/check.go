package typecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/diag"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/token"
)

type symKind int

const (
	kVar symKind = iota
	kParam
	kFunc
	kStruct
	kModule
	kBuiltin
)

type symbol struct {
	kind     symKind
	typ      Type
	declared Type // declared type of a variable (typ may be narrowed)
}

type scope struct {
	syms     map[string]*symbol
	outer    *scope
	narrowed map[string]Type
}

func newScope(outer *scope) *scope {
	return &scope{syms: map[string]*symbol{}, outer: outer, narrowed: map[string]Type{}}
}

// lookup returns the symbol and its effective (possibly narrowed) type.
func (s *scope) lookup(name string) (*symbol, Type) {
	for sc := s; sc != nil; sc = sc.outer {
		if t, ok := sc.narrowed[name]; ok {
			if sym, ok := sc.lookupLocalChain(name); ok {
				return sym, t
			}
		}
		if sym, ok := sc.syms[name]; ok {
			return sym, sym.typ
		}
	}
	return nil, nil
}

func (s *scope) lookupLocalChain(name string) (*symbol, bool) {
	for sc := s; sc != nil; sc = sc.outer {
		if sym, ok := sc.syms[name]; ok {
			return sym, true
		}
	}
	return nil, false
}

func (s *scope) clearNarrow(name string) {
	for sc := s; sc != nil; sc = sc.outer {
		delete(sc.narrowed, name)
		if _, ok := sc.syms[name]; ok {
			return
		}
	}
}

func (s *scope) names() []string {
	var out []string
	for sc := s; sc != nil; sc = sc.outer {
		for n := range sc.syms {
			out = append(out, n)
		}
	}
	return out
}

type funcCtx struct {
	name string
	ret  Type
}

// Checker performs strict static type verification over Alca ASTs.
type Checker struct {
	filename string
	source   string
	errors   []*diag.SyntaxError
	structs  map[string]*StructType
	types    map[ast.Expression]Type
	global   *scope
	fn       *funcCtx
	loop     int
	imported map[string]bool // files currently being analysed (cycle guard)
}

// NewChecker creates a checker for one source file.
func NewChecker(filename string, source string) *Checker {
	return &Checker{
		filename: filename,
		source:   source,
		errors:   make([]*diag.SyntaxError, 0),
		structs:  make(map[string]*StructType),
		types:    make(map[ast.Expression]Type),
		imported: map[string]bool{},
	}
}

// Errors returns all accumulated type diagnostics.
func (c *Checker) Errors() []*diag.SyntaxError { return c.errors }

// TypeOf returns the static type recorded for an expression (nil if unknown).
func (c *Checker) TypeOf(e ast.Expression) Type { return c.types[e] }

func (c *Checker) addError(tok token.Token, msg, suggestion string) {
	length := len(tok.Literal)
	if length <= 0 {
		length = 1
	}
	c.errors = append(c.errors, &diag.SyntaxError{
		Filename:   c.filename,
		Line:       tok.Line,
		Column:     tok.Column,
		Message:    msg,
		Token:      tok.Literal,
		Length:     length,
		Suggestion: suggestion,
	})
}

// Check performs a full static type check of the program.
func (c *Checker) Check(program *ast.Program) []*diag.SyntaxError {
	c.initGlobal()
	c.hoist(program.Statements, c.global)
	c.stmts(program.Statements, c.global)
	return c.errors
}

// CheckSnippet performs an incremental check of a REPL-style snippet. Unlike
// Check it keeps the symbols, structs and functions declared by earlier
// snippets, and reports only the diagnostics produced by this one. Callers
// that reject a snippet should Restore a Snapshot taken before the call so the
// checker stays in sync with the evaluated state.
func (c *Checker) CheckSnippet(program *ast.Program) []*diag.SyntaxError {
	c.errors = c.errors[:0]
	if c.global == nil {
		c.initGlobal()
	}
	c.hoist(program.Statements, c.global)
	c.stmts(program.Statements, c.global)
	return c.errors
}

// Snapshot captures the top-level declarations of the checker.
type Snapshot struct {
	syms    map[string]*symbol
	structs map[string]*StructType
}

// Snapshot records the current top-level declarations so a failed
// CheckSnippet can be rolled back with Restore.
func (c *Checker) Snapshot() *Snapshot {
	if c.global == nil {
		c.initGlobal()
	}
	s := &Snapshot{
		syms:    make(map[string]*symbol, len(c.global.syms)),
		structs: make(map[string]*StructType, len(c.structs)),
	}
	for name, sym := range c.global.syms {
		s.syms[name] = sym
	}
	for name, st := range c.structs {
		cp := *st
		cp.Fields = append([]Field(nil), st.Fields...)
		s.structs[name] = &cp
	}
	return s
}

// Restore returns the checker to the state captured by Snapshot.
func (c *Checker) Restore(s *Snapshot) {
	c.global.syms = s.syms
	c.structs = s.structs
}

// initGlobal installs the builtin symbols and the 'std' namespace.
func (c *Checker) initGlobal() {
	c.global = newScope(nil)
	for name, t := range globalBuiltins() {
		c.global.syms[name] = &symbol{kind: kBuiltin, typ: t}
	}
	for _, name := range []string{"len", "keys", "values", "push", "pop", "range", "list"} {
		c.global.syms[name] = &symbol{kind: kBuiltin, typ: &FuncType{Dynamic: true, Ret: JSON}}
	}
	c.global.syms["std"] = &symbol{kind: kModule, typ: stdNamespace()}
}

// hoist registers top-level structs and functions so they may be used before
// their definition (mutual recursion, helpers placed after main code).
func (c *Checker) hoist(stmts []ast.Statement, sc *scope) {
	// pass 1: struct names (fields may refer to other structs)
	for _, st := range stmts {
		if s, ok := st.(*ast.StructDeclaration); ok {
			if _, dup := c.structs[s.Name]; dup {
				c.addError(s.Token, fmt.Sprintf("struct '%s' is already defined", s.Name), "")
				continue
			}
			c.structs[s.Name] = &StructType{Name: s.Name}
			sc.syms[s.Name] = &symbol{kind: kStruct, typ: c.structs[s.Name]}
		}
	}
	// pass 2: struct fields
	for _, st := range stmts {
		if s, ok := st.(*ast.StructDeclaration); ok {
			stype := c.structs[s.Name]
			if stype == nil || len(stype.Fields) > 0 {
				continue
			}
			seen := map[string]bool{}
			for _, f := range s.Fields {
				if seen[f.Name] {
					c.addError(f.Token, fmt.Sprintf("duplicate field '%s' in struct %s", f.Name, s.Name), "")
					continue
				}
				seen[f.Name] = true
				ft := c.resolve(f.TypeAnnotation)
				if f.DefaultValue != nil {
					dt := c.expr(f.DefaultValue, sc, ft)
					ne, ok := c.fit(ft, f.DefaultValue, dt)
					if !ok {
						c.addError(f.Token, fmt.Sprintf("default value of field '%s' has type %s, expected %s", f.Name, dt, ft), conversionHint(ft, dt))
					}
					f.DefaultValue = ne
				}
				stype.Fields = append(stype.Fields, Field{Name: f.Name, Type: ft, HasDefault: f.DefaultValue != nil})
			}
		}
	}
	// pass 3: function signatures
	for _, st := range stmts {
		if f, ok := st.(*ast.FuncDeclaration); ok {
			if prev, dup := sc.syms[f.Name]; dup && prev.kind != kBuiltin {
				c.addError(f.Token, fmt.Sprintf("'%s' is already defined", f.Name), "")
				continue
			}
			sc.syms[f.Name] = &symbol{kind: kFunc, typ: c.funcSig(f)}
		}
	}
}

func (c *Checker) funcSig(f *ast.FuncDeclaration) *FuncType {
	ft := &FuncType{Ret: Void}
	seen := map[string]bool{}
	for _, p := range f.Params {
		if seen[p.Name] {
			c.addError(p.Token, fmt.Sprintf("duplicate parameter '%s'", p.Name), "")
		}
		seen[p.Name] = true
		if p.TypeAnnotation == nil {
			c.addError(p.Token, fmt.Sprintf("parameter '%s' needs a type annotation", p.Name),
				fmt.Sprintf("write '%s: type'", p.Name))
			ft.Params = append(ft.Params, Param{Name: p.Name, Type: JSON, Optional: p.DefaultValue != nil})
			continue
		}
		ft.Params = append(ft.Params, Param{Name: p.Name, Type: c.resolve(p.TypeAnnotation), Optional: p.DefaultValue != nil})
	}
	if f.ReturnType != nil {
		ft.Ret = c.resolve(f.ReturnType)
	}
	return ft
}

// resolve converts a type annotation to a Type; reports unknown types.
func (c *Checker) resolve(ta *ast.TypeAnnotation) Type {
	if ta == nil {
		return JSON
	}
	src := strings.Join(ta.Types, " | ")
	t, ok := ParseType(src, c.structs)
	if !ok {
		name := ta.Types[0]
		var cands []string
		for n := range c.structs {
			cands = append(cands, n)
		}
		cands = append(cands, "int", "float", "string", "bool", "list", "map", "blob", "secret", "json", "null", "error")
		hint := ""
		if s := suggest(strings.SplitN(name, "[", 2)[0], cands); s != "" {
			hint = fmt.Sprintf("did you mean '%s'?", s)
		}
		c.addError(ta.Token, fmt.Sprintf("unknown type '%s'", src), hint)
		return JSON
	}
	if containsAny(t) {
		c.addError(ta.Token, "type 'any' cannot be used in declarations", "use a concrete type, a union (int | string) or json")
		return JSON
	}
	return t
}

func containsAny(t Type) bool {
	switch x := t.(type) {
	case *Basic:
		return x == Any
	case *ListType:
		return containsAny(x.Elem)
	case *MapType:
		return containsAny(x.Key) || containsAny(x.Val)
	case *UnionType:
		for _, m := range x.Types {
			if containsAny(m) {
				return true
			}
		}
	}
	return false
}

func (c *Checker) assignable(dst, src Type, e ast.Expression) bool {
	if Assignable(dst, src) {
		return true
	}
	// An integer literal may initialise a float.
	if dst == Float && src == Int && isIntLiteral(e) {
		return true
	}
	return false
}

func isIntLiteral(e ast.Expression) bool {
	switch x := e.(type) {
	case *ast.IntegerLiteral:
		return true
	case *ast.PrefixExpression:
		_, ok := x.Right.(*ast.IntegerLiteral)
		return ok && x.Operator == "-"
	}
	return false
}

// declare adds a variable to sc, rejecting redeclaration and shadowing.
func (c *Checker) declare(name string, tok token.Token, t Type, kind symKind, sc *scope) {
	if name == "_" {
		return
	}
	if prev, ok := sc.syms[name]; ok && prev.kind != kBuiltin {
		c.addError(tok, fmt.Sprintf("'%s' is already declared in this scope", name), "")
		return
	}
	if kind == kVar {
		if prev, _ := sc.lookup(name); prev != nil && prev.kind != kBuiltin {
			c.addError(tok, fmt.Sprintf("'%s' shadows an existing %s", name, kindName(prev.kind)),
				"choose a different name; Alca does not allow shadowing")
			return
		}
	}
	sc.syms[name] = &symbol{kind: kind, typ: t, declared: t}
}

func kindName(k symKind) string {
	switch k {
	case kFunc:
		return "function"
	case kStruct:
		return "struct"
	case kModule:
		return "module"
	case kParam:
		return "parameter"
	}
	return "variable"
}

// ---------------------------------------------------------------------------
// Statements
// ---------------------------------------------------------------------------

func (c *Checker) stmts(list []ast.Statement, sc *scope) {
	for _, st := range list {
		c.stmt(st, sc)
	}
}

func (c *Checker) block(b *ast.BlockStatement, sc *scope) {
	if b == nil {
		return
	}
	inner := newScope(sc)
	c.hoist(b.Statements, inner)
	c.stmts(b.Statements, inner)
}

func (c *Checker) condition(e ast.Expression, sc *scope, what string) {
	t := c.expr(e, sc, Bool)
	if t != Bool && t != Never && t != Invalid {
		c.addError(tokOf(e), fmt.Sprintf("%s must be bool, got %s", what, t), "Alca has no truthiness: compare explicitly (e.g. x != 0, s != \"\", x != null)")
	}
}

func (c *Checker) stmt(st ast.Statement, sc *scope) {
	switch s := st.(type) {
	case *ast.VarDeclaration:
		c.varDecl(s, sc)

	case *ast.FuncDeclaration:
		if sym, ok := sc.syms[s.Name]; !ok || sym.kind != kFunc {
			c.declare(s.Name, s.Token, c.funcSig(s), kFunc, sc)
		}
		c.funcBody(s, sc)

	case *ast.StructDeclaration:
		// registered by hoist

	case *ast.ExpressionStatement:
		c.expr(s.Expression, sc, nil)

	case *ast.IfStatement:
		c.ifStmt(s, sc)

	case *ast.WhileStatement:
		c.condition(s.Condition, sc, "while condition")
		c.loop++
		c.block(s.Body, sc)
		c.loop--

	case *ast.LoopStatement:
		c.loop++
		c.block(s.Body, sc)
		c.loop--

	case *ast.ForStatement:
		c.forLoop(s.Variable, s.Token, s.Iterable, s.Body, sc)
	case *ast.ForEachStatement:
		c.forLoop(s.Variable, s.Token, s.Iterable, s.Body, sc)

	case *ast.ReturnStatement:
		c.returnStmt(s, sc)

	case *ast.BreakStatement:
		if c.loop == 0 {
			c.addError(s.Token, "'break' outside of a loop", "")
		}
	case *ast.ContinueStatement:
		if c.loop == 0 {
			c.addError(s.Token, "'continue' outside of a loop", "")
		}

	case *ast.BlockStatement:
		c.block(s, sc)

	case *ast.UseStatement:
		c.useStmt(s, sc)

	case *ast.AssertStatement:
		c.condition(s.Expression, sc, "assert expression")

	case *ast.TestStatement:
		saved, savedLoop := c.fn, c.loop
		c.fn, c.loop = &funcCtx{name: "test", ret: Void}, 0
		c.block(s.Body, sc)
		c.fn, c.loop = saved, savedLoop

	case *ast.WatchStatement:
		t := c.expr(s.Target, sc, String)
		if t != String && t != JSON {
			c.addError(tokOf(s.Target), fmt.Sprintf("watch target must be a string path, got %s", t), "")
		}
		c.block(s.Body, sc)

	case *ast.MockStatement:
		tt := c.expr(s.Target, sc, nil)
		rt := c.expr(s.Replacement, sc, tt)
		if tt != nil && rt != nil {
			if _, isF := tt.(*FuncType); isF && !Assignable(tt, rt) && tt != JSON {
				c.addError(tokOf(s.Replacement), fmt.Sprintf("mock replacement has type %s, expected %s", rt, tt), "")
			}
		}

	case *ast.BaStatement:
		if s.Limit != nil {
			if t := c.expr(s.Limit, sc, Int); t != Int {
				c.addError(tokOf(s.Limit), fmt.Sprintf("ba limit must be int, got %s", t), "")
			}
		}
		if s.Timeout != nil {
			c.expr(s.Timeout, sc, nil)
		}
		c.block(s.Body, sc)

	case *ast.BaForStatement:
		if s.Limit != nil {
			c.expr(s.Limit, sc, Int)
		}
		c.forLoop(s.Variable, s.Token, s.Iterable, s.Body, sc)

	case *ast.ThreadsStatement:
		c.block(s.Body, sc)

	case *ast.TxStatement:
		c.expr(s.DB, sc, nil)
		if s.Timeout != nil {
			c.expr(s.Timeout, sc, nil)
		}
		c.block(s.Body, sc)

	case *ast.WaitStatement:
		// nothing to check

	case *ast.MatchStatement:
		c.matchStmt(s, sc)
	}
}

func (c *Checker) varDecl(s *ast.VarDeclaration, sc *scope) {
	var declared Type
	if s.TypeAnnotation != nil {
		declared = c.resolve(s.TypeAnnotation)
	}
	if s.Value == nil {
		if declared != nil {
			c.declare(s.Name.Value, s.Name.Token, declared, kVar, sc)
		}
		return
	}
	vt := c.expr(s.Value, sc, declared)
	switch {
	case declared == nil: // inferred ':='
		switch {
		case vt == Null:
			c.addError(s.Name.Token, "cannot infer a type from 'null'", fmt.Sprintf("annotate it: '%s: type | null = null;'", s.Name.Value))
			vt = JSON
		case vt == Void:
			c.addError(tokOf(s.Value), "expression has no value (void)", "")
			vt = JSON
		case vt == Never:
			vt = JSON
		}
		if lt, ok := vt.(*ListType); ok && lt.Elem == Never {
			c.addError(s.Name.Token, "cannot infer the element type of an empty list", fmt.Sprintf("annotate it: '%s: list[int] = [];'", s.Name.Value))
		}
		c.declare(s.Name.Value, s.Name.Token, vt, kVar, sc)
	default:
		ne, ok := c.fit(declared, s.Value, vt)
		if !ok {
			c.addError(tokOf(s.Value), fmt.Sprintf("cannot initialize '%s' (%s) with a value of type %s", s.Name.Value, declared, vt), conversionHint(declared, vt))
		}
		s.Value = ne
		c.declare(s.Name.Value, s.Name.Token, declared, kVar, sc)
	}
}

func conversionHint(dst, src Type) string {
	if dst == Float && src == Int {
		return "use float(x) to convert explicitly"
	}
	if dst == Int && src == Float {
		return "use int(x) to convert (truncates) or math.round/floor/ceil"
	}
	if dst == String && src != String {
		return "use string(x) to convert"
	}
	if (dst == Int || dst == Float || dst == Bool) && (src == String || src == JSON) {
		return fmt.Sprintf("use %s(x) to convert", dst)
	}
	if Nullable(src) && !Nullable(dst) {
		return "the value may be null: check 'x != null' first or declare the target as 'T | null'"
	}
	return ""
}

func (c *Checker) funcBody(f *ast.FuncDeclaration, sc *scope) {
	sig, _ := sc.lookup(f.Name)
	var ft *FuncType
	if sig != nil {
		ft, _ = sig.typ.(*FuncType)
	}
	if ft == nil {
		ft = c.funcSig(f)
	}
	inner := newScope(sc)
	for i, p := range f.Params {
		var pt Type = JSON
		if i < len(ft.Params) {
			pt = ft.Params[i].Type
		}
		if p.DefaultValue != nil {
			dt := c.expr(p.DefaultValue, sc, pt)
			ne, ok := c.fit(pt, p.DefaultValue, dt)
			if !ok {
				c.addError(p.Token, fmt.Sprintf("default value of '%s' has type %s, expected %s", p.Name, dt, pt), conversionHint(pt, dt))
			}
			p.DefaultValue = ne
		}
		inner.syms[p.Name] = &symbol{kind: kParam, typ: pt, declared: pt}
	}
	savedFn, savedLoop := c.fn, c.loop
	c.fn, c.loop = &funcCtx{name: f.Name, ret: ft.Ret}, 0
	if f.Body != nil {
		c.hoist(f.Body.Statements, inner)
		c.stmts(f.Body.Statements, inner)
		if ft.Ret != Void && !blockTerminates(f.Body) {
			c.addError(f.Token, fmt.Sprintf("function '%s' must return %s on every path", f.Name, ft.Ret),
				"add a 'return' (or 'err(...)') at the end, or declare the function without a return type")
		}
	}
	c.fn, c.loop = savedFn, savedLoop
}

func (c *Checker) returnStmt(s *ast.ReturnStatement, sc *scope) {
	if c.fn == nil {
		c.addError(s.Token, "'return' outside of a function", "")
		if s.Value != nil {
			c.expr(s.Value, sc, nil)
		}
		return
	}
	want := c.fn.ret
	if s.Value == nil {
		if want != Void && !Nullable(want) {
			c.addError(s.Token, fmt.Sprintf("function '%s' must return a value of type %s", c.fn.name, want), "")
		}
		return
	}
	got := c.expr(s.Value, sc, want)
	if want == Void {
		c.addError(tokOf(s.Value), fmt.Sprintf("function '%s' is declared without a return type but returns a value", c.fn.name),
			"declare the return type: '-> "+got.String()+"'")
		return
	}
	ne, ok := c.fit(want, s.Value, got)
	if !ok {
		c.addError(tokOf(s.Value), fmt.Sprintf("function '%s' returns %s, but the value has type %s", c.fn.name, want, got), conversionHint(want, got))
	}
	s.Value = ne
}

func (c *Checker) forLoop(name string, tok token.Token, iter ast.Expression, body *ast.BlockStatement, sc *scope) {
	it := c.expr(iter, sc, nil)
	var elem Type = JSON
	if it == Invalid {
		elem = Invalid
	}
	switch t := it.(type) {
	case *ListType:
		elem = t.Elem
	case *MapType:
		elem = t.Key
	case *Basic:
		switch t {
		case String:
			elem = String
		case Blob:
			elem = Int
		case JSON:
			elem = JSON
		case Int:
			elem = Int // 'for i in 10' iterates 0..9
		case Invalid:
		default:
			c.addError(tokOf(iter), fmt.Sprintf("cannot iterate over %s", it), "iterate over a list, map, string or a range (a..b)")
		}
	default:
		c.addError(tokOf(iter), fmt.Sprintf("cannot iterate over %s", it), "iterate over a list, map, string or a range (a..b)")
	}
	inner := newScope(sc)
	c.declare(name, tok, elem, kVar, inner)
	c.loop++
	c.block(body, inner)
	c.loop--
}

func (c *Checker) matchStmt(s *ast.MatchStatement, sc *scope) {
	st := c.expr(s.Subject, sc, nil)
	for _, cs := range s.Cases {
		pt := c.expr(cs.Pattern, sc, st)
		if pt != nil && st != nil && !Assignable(st, pt) && !Assignable(pt, st) && !c.assignable(st, pt, cs.Pattern) {
			c.addError(tokOf(cs.Pattern), fmt.Sprintf("match pattern has type %s, but the subject has type %s", pt, st), "")
		}
		c.block(cs.Body, sc)
	}
	if s.Default != nil {
		c.block(s.Default, sc)
	}
}

// ifStmt checks an if/elif/else chain with null-check narrowing.
func (c *Checker) ifStmt(s *ast.IfStatement, sc *scope) {
	c.condition(s.Condition, sc, "if condition")
	whenTrue, whenFalse := narrowings(s.Condition, sc)

	thenScope := newScope(sc)
	for name, t := range whenTrue {
		thenScope.narrowed[name] = t
	}
	c.hoist(s.Consequence.Statements, thenScope)
	c.stmts(s.Consequence.Statements, thenScope)

	elseScope := sc
	for _, alt := range s.Alternatives {
		elseScope = newScope(elseScope)
		for name, t := range whenFalse {
			elseScope.narrowed[name] = t
		}
		c.condition(alt.Condition, elseScope, "elif condition")
		wt, wf := narrowings(alt.Condition, elseScope)
		altScope := newScope(elseScope)
		for name, t := range wt {
			altScope.narrowed[name] = t
		}
		c.hoist(alt.Body.Statements, altScope)
		c.stmts(alt.Body.Statements, altScope)
		for name, t := range wf {
			whenFalse[name] = t
		}
	}
	if s.ElseBlock != nil {
		eb := newScope(elseScope)
		for name, t := range whenFalse {
			eb.narrowed[name] = t
		}
		c.hoist(s.ElseBlock.Statements, eb)
		c.stmts(s.ElseBlock.Statements, eb)
	}

	// Guard clause: 'if x == null: return;' narrows x for the rest of the block.
	if s.ElseBlock == nil && len(s.Alternatives) == 0 && blockTerminates(s.Consequence) {
		for name, t := range whenFalse {
			sc.narrowed[name] = t
		}
	}
}

// narrowings derives null-check refinements from a condition.
func narrowings(cond ast.Expression, sc *scope) (whenTrue, whenFalse map[string]Type) {
	whenTrue, whenFalse = map[string]Type{}, map[string]Type{}
	var visit func(e ast.Expression, positive bool)
	visit = func(e ast.Expression, positive bool) {
		inf, ok := e.(*ast.InfixExpression)
		if !ok {
			return
		}
		if inf.Operator == "and" && positive {
			visit(inf.Left, true)
			visit(inf.Right, true)
			return
		}
		if inf.Operator != "==" && inf.Operator != "!=" {
			return
		}
		id, ok := inf.Left.(*ast.Identifier)
		_, isNull := inf.Right.(*ast.NullLiteral)
		if !ok || !isNull {
			id2, ok2 := inf.Right.(*ast.Identifier)
			_, isNull2 := inf.Left.(*ast.NullLiteral)
			if !ok2 || !isNull2 {
				return
			}
			id = id2
		}
		sym, cur := sc.lookup(id.Value)
		if sym == nil || cur == JSON || !Nullable(cur) {
			return
		}
		nonNull := StripNull(cur)
		isNotNull := inf.Operator == "!="
		if isNotNull {
			whenTrue[id.Value] = nonNull
			if e == cond {
				whenFalse[id.Value] = Null
			}
		} else {
			whenFalse[id.Value] = nonNull
			if e == cond {
				whenTrue[id.Value] = Null
			}
		}
	}
	visit(cond, true)
	return
}

// blockTerminates reports whether every path through b ends in return/err/exit.
func blockTerminates(b *ast.BlockStatement) bool {
	if b == nil {
		return false
	}
	for _, st := range b.Statements {
		if stmtTerminates(st) {
			return true
		}
	}
	return false
}

func stmtTerminates(st ast.Statement) bool {
	switch s := st.(type) {
	case *ast.ReturnStatement:
		return true
	case *ast.ExpressionStatement:
		switch e := s.Expression.(type) {
		case *ast.ErrExpression:
			return true
		case *ast.CallExpression:
			if id, ok := e.Function.(*ast.Identifier); ok && id.Value == "exit" {
				return true
			}
		}
	case *ast.IfStatement:
		if s.ElseBlock == nil || !blockTerminates(s.Consequence) || !blockTerminates(s.ElseBlock) {
			return false
		}
		for _, alt := range s.Alternatives {
			if !blockTerminates(alt.Body) {
				return false
			}
		}
		return true
	case *ast.MatchStatement:
		if s.Default == nil || !blockTerminates(s.Default) {
			return false
		}
		for _, cs := range s.Cases {
			if !blockTerminates(cs.Body) {
				return false
			}
		}
		return true
	case *ast.LoopStatement:
		return !containsBreak(s.Body)
	case *ast.BlockStatement:
		return blockTerminates(s)
	}
	return false
}

func containsBreak(b *ast.BlockStatement) bool {
	if b == nil {
		return false
	}
	var inStmt func(st ast.Statement) bool
	inStmt = func(st ast.Statement) bool {
		switch s := st.(type) {
		case *ast.BreakStatement:
			return true
		case *ast.IfStatement:
			if containsBreak(s.Consequence) || containsBreak(s.ElseBlock) {
				return true
			}
			for _, a := range s.Alternatives {
				if containsBreak(a.Body) {
					return true
				}
			}
		case *ast.MatchStatement:
			for _, cs := range s.Cases {
				if containsBreak(cs.Body) {
					return true
				}
			}
			return containsBreak(s.Default)
		case *ast.BlockStatement:
			return containsBreak(s)
		}
		return false // nested loops own their breaks
	}
	for _, st := range b.Statements {
		if inStmt(st) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Modules
// ---------------------------------------------------------------------------

func (c *Checker) useStmt(s *ast.UseStatement, sc *scope) {
	path := s.ModulePath
	var mod *ModuleType
	bind := s.Alias

	if strings.HasPrefix(path, "std.") || path == "std" {
		rest := strings.TrimPrefix(path, "std.")
		segs := strings.Split(rest, ".")
		if m, ok := moduleByName(segs[0]); ok && len(segs) == 1 {
			mod = m
			if bind == "" {
				bind = segs[0]
			}
		} else if ok {
			mod = &ModuleType{Name: rest, Open: true}
			if bind == "" {
				bind = segs[len(segs)-1]
			}
		} else {
			var cands []string
			for n := range moduleMembers {
				cands = append(cands, n)
			}
			hint := ""
			if sg := suggest(segs[0], cands); sg != "" {
				hint = fmt.Sprintf("did you mean 'std.%s'?", sg)
			}
			c.addError(s.Token, fmt.Sprintf("unknown standard module '%s'", path), hint)
			return
		}
	} else {
		mod = c.loadUserModule(path, s)
		if bind == "" {
			bind = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
	}

	if len(s.Imports) > 0 {
		for _, name := range s.Imports {
			var t Type = JSON
			if mt, ok := mod.Members[name]; ok {
				t = mt
			} else if !mod.Open {
				c.addError(s.Token, fmt.Sprintf("module '%s' has no member '%s'", path, name), "")
			} else {
				t = &FuncType{Dynamic: true, Ret: JSON}
			}
			if _, exists := sc.syms[name]; exists && sc.syms[name].kind != kBuiltin {
				continue
			}
			kind := kVar
			if _, isF := t.(*FuncType); isF {
				kind = kFunc
			}
			if _, isO := t.(*Overloads); isO {
				kind = kFunc
			}
			sc.syms[name] = &symbol{kind: kind, typ: t, declared: t}
		}
		return
	}
	if bind != "" {
		sc.syms[bind] = &symbol{kind: kModule, typ: mod}
	}
	// 'use std.math' also makes 'std.math' resolvable through the std namespace (already global).
}

// loadUserModule analyses an imported .al file and exposes its public API.
func (c *Checker) loadUserModule(path string, s *ast.UseStatement) *ModuleType {
	open := &ModuleType{Name: path, Open: true}
	if c.filename == "" {
		return open
	}
	file := path
	if !strings.HasSuffix(file, ".al") {
		file += ".al"
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(filepath.Dir(c.filename), file)
	}
	abs, err := filepath.Abs(file)
	if err != nil || c.imported[abs] {
		return open
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		c.addError(s.Token, fmt.Sprintf("cannot read module '%s'", path), err.Error())
		return open
	}
	src := string(data)
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.SyntaxErrors()) > 0 {
		c.addError(s.Token, fmt.Sprintf("module '%s' has syntax errors", path), "run 'alca check "+file+"'")
		return open
	}
	sub := NewChecker(abs, src)
	for k := range c.imported {
		sub.imported[k] = true
	}
	sub.imported[abs] = true
	sub.Check(prog)
	if len(sub.errors) > 0 {
		c.addError(s.Token, fmt.Sprintf("module '%s' has %d type error(s)", path, len(sub.errors)), "run 'alca check "+file+"'")
	}
	mod := &ModuleType{Name: path, Members: map[string]Type{}}
	for _, st := range prog.Statements {
		switch d := st.(type) {
		case *ast.FuncDeclaration:
			if sym, ok := sub.global.syms[d.Name]; ok {
				mod.Members[d.Name] = sym.typ
			}
		case *ast.StructDeclaration:
			if sym, ok := sub.global.syms[d.Name]; ok {
				mod.Members[d.Name] = sym.typ
				c.structs[d.Name] = sub.structs[d.Name]
			}
		case *ast.VarDeclaration:
			if sym, ok := sub.global.syms[d.Name.Value]; ok {
				mod.Members[d.Name.Value] = sym.typ
			}
		}
	}
	return mod
}

// sortedKeys returns map keys in deterministic order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
