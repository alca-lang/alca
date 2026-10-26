// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package typecheck

import (
	"fmt"
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/token"
)

// tokOf returns a representative source token for diagnostics.
func tokOf(e ast.Expression) token.Token {
	switch n := e.(type) {
	case *ast.Identifier:
		return n.Token
	case *ast.IntegerLiteral:
		return n.Token
	case *ast.FloatLiteral:
		return n.Token
	case *ast.StringLiteral:
		return n.Token
	case *ast.BooleanLiteral:
		return n.Token
	case *ast.NullLiteral:
		return n.Token
	case *ast.ListLiteral:
		return n.Token
	case *ast.MapLiteral:
		return n.Token
	case *ast.PrefixExpression:
		return n.Token
	case *ast.InfixExpression:
		if t := tokOf(n.Left); t.Line != 0 {
			return t
		}
		return n.Token
	case *ast.CallExpression:
		if t := tokOf(n.Function); t.Line != 0 {
			return t
		}
		return n.Token
	case *ast.IndexExpression:
		if t := tokOf(n.Left); t.Line != 0 {
			return t
		}
		return n.Token
	case *ast.DotExpression:
		if t := tokOf(n.Left); t.Line != 0 {
			return t
		}
		return n.Token
	case *ast.AssignExpression:
		if t := tokOf(n.Target); t.Line != 0 {
			return t
		}
		return n.Token
	case *ast.StreamExpression:
		return n.Token
	case *ast.PipeExpression:
		return n.Token
	case *ast.TimeoutExpression:
		return n.Token
	case *ast.ConsoleLiteral:
		return n.Token
	case *ast.LambdaExpression:
		return n.Token
	case *ast.ErrExpression:
		return n.Token
	case *ast.RangeLiteral:
		return n.Token
	case *ast.StructInstantiation:
		return n.Token
	case *ast.AwaitExpression:
		return n.Token
	}
	return token.Token{}
}

// fit checks that e (of type got) can be stored in dst. It also performs the single
// permitted implicit conversion: an integer literal used where a float is expected is
// rewritten into a float literal (so the runtime value really is a float).
func (c *Checker) fit(dst Type, e ast.Expression, got Type) (ast.Expression, bool) {
	if Assignable(dst, got) {
		return e, true
	}
	if dst == Float && got == Int {
		switch x := e.(type) {
		case *ast.IntegerLiteral:
			return &ast.FloatLiteral{Token: x.Token, Value: float64(x.Value)}, true
		case *ast.PrefixExpression:
			if lit, ok := x.Right.(*ast.IntegerLiteral); ok && x.Operator == "-" {
				return &ast.FloatLiteral{Token: x.Token, Value: -float64(lit.Value)}, true
			}
		}
	}
	return e, false
}

func (c *Checker) expr(e ast.Expression, sc *scope, expected Type) Type {
	if e == nil {
		return Void
	}
	before := len(c.errors)
	t := c.expr0(e, sc, expected)
	if t == nil {
		t = JSON
	}
	if len(c.errors) > before && t == JSON {
		t = Invalid // already diagnosed: suppress follow-up errors
	}
	c.types[e] = t
	return t
}

func (c *Checker) expr0(e ast.Expression, sc *scope, expected Type) Type {
	switch n := e.(type) {
	case *ast.IntegerLiteral:
		return Int
	case *ast.FloatLiteral:
		return Float
	case *ast.StringLiteral:
		return String
	case *ast.BooleanLiteral:
		return Bool
	case *ast.NullLiteral:
		return Null

	case *ast.Identifier:
		return c.identifier(n, sc)

	case *ast.ListLiteral:
		return c.listLiteral(n, sc, expected)
	case *ast.MapLiteral:
		return c.mapLiteral(n, sc, expected)

	case *ast.PrefixExpression:
		rt := c.expr(n.Right, sc, nil)
		if rt == Invalid {
			return Invalid
		}
		switch n.Operator {
		case "-":
			if IsNumeric(rt) {
				return rt
			}
			c.addError(n.Token, fmt.Sprintf("cannot negate a value of type %s", rt), "")
			return Int
		case "not", "!":
			if rt != Bool {
				c.addError(n.Token, fmt.Sprintf("'%s' requires a bool operand, got %s", n.Operator, rt), "compare explicitly instead of relying on truthiness")
			}
			return Bool
		}
		return rt

	case *ast.InfixExpression:
		return c.infix(n, sc)

	case *ast.AssignExpression:
		return c.assign(n, sc)

	case *ast.CallExpression:
		return c.call(n, sc)

	case *ast.StructInstantiation:
		call := &ast.CallExpression{Token: n.Token, Function: &ast.Identifier{Token: n.Token, Value: n.Name}, Arguments: n.Args}
		return c.call(call, sc)

	case *ast.IndexExpression:
		return c.index(n, sc)

	case *ast.DotExpression:
		return c.dot(n, sc)

	case *ast.RangeLiteral:
		for _, side := range []*ast.Expression{&n.Start, &n.End} {
			if *side == nil {
				continue
			}
			if t := c.expr(*side, sc, Int); t != Int && t != JSON {
				c.addError(tokOf(*side), fmt.Sprintf("range bounds must be int, got %s", t), "")
			}
		}
		return &ListType{Elem: Int}

	case *ast.StreamExpression:
		return c.stream(n, sc)

	case *ast.PipeExpression:
		lt := c.expr(n.Left, sc, nil)
		rt := c.expr(n.Right, sc, nil)
		switch f := rt.(type) {
		case *FuncType:
			if f.Dynamic {
				return JSON
			}
			if len(f.Params) == 0 {
				c.addError(n.Token, "pipe target takes no arguments", "")
				return f.Ret
			}
			if !Assignable(f.Params[0].Type, lt) {
				c.addError(n.Token, fmt.Sprintf("pipe passes %s, but the function expects %s", lt, f.Params[0].Type), "")
			}
			return f.Ret
		case *Overloads:
			for _, sg := range f.Sigs {
				if len(sg.Params) > 0 && Assignable(sg.Params[0].Type, lt) {
					return sg.Ret
				}
			}
			c.addError(n.Token, fmt.Sprintf("no overload accepts %s", lt), "")
			return JSON
		}
		if rt != JSON {
			c.addError(n.Token, fmt.Sprintf("pipe target must be a function, got %s", rt), "")
		}
		return JSON

	case *ast.TimeoutExpression:
		inner := n.Expression
		if inner == nil {
			inner = n.Expr
		}
		var t Type = JSON
		if inner != nil {
			t = c.expr(inner, sc, nil)
		}
		if n.Duration != nil {
			c.expr(n.Duration, sc, nil)
		}
		return t

	case *ast.ConsoleLiteral:
		if n.Prompt != nil {
			if pt := c.expr(n.Prompt, sc, String); pt != String {
				c.addError(tokOf(n.Prompt), fmt.Sprintf("console prompt must be string, got %s", pt), "")
			}
		}
		return String

	case *ast.ErrExpression:
		if n.Message != nil {
			if mt := c.expr(n.Message, sc, String); mt != String {
				c.addError(tokOf(n.Message), fmt.Sprintf("error message must be string, got %s", mt), "use string(x)")
			}
		}
		if n.Code != nil {
			if ct := c.expr(n.Code, sc, Int); ct != Int {
				c.addError(tokOf(n.Code), fmt.Sprintf("error code must be int, got %s", ct), "")
			}
		}
		return Never

	case *ast.LambdaExpression:
		return c.lambda(n, sc)

	case *ast.AwaitExpression:
		return c.expr(n.Expression, sc, expected)
	}
	return JSON
}

func (c *Checker) identifier(n *ast.Identifier, sc *scope) Type {
	sym, t := sc.lookup(n.Value)
	if sym == nil {
		if mod, ok := deprecatedGlobalAliases[n.Value]; ok {
			c.addError(n.Token, fmt.Sprintf("global alias '%s' is deprecated and has been removed", n.Value),
				fmt.Sprintf("import the module first: 'use std.%s;' then '%s.%s(...)'", mod, mod, n.Value))
			return JSON
		}
		if strings.HasPrefix(n.Value, "gui_") {
			c.addError(n.Token, fmt.Sprintf("global alias '%s' is deprecated and has been removed", n.Value),
				fmt.Sprintf("use 'use std.gui;' then 'gui.%s(...)'", strings.TrimPrefix(n.Value, "gui_")))
			return JSON
		}
		hint := ""
		if s := suggest(n.Value, sc.names()); s != "" {
			hint = fmt.Sprintf("did you mean '%s'?", s)
		} else if _, ok := moduleByName(n.Value); ok {
			hint = fmt.Sprintf("import the module first: 'use std.%s;'", n.Value)
		}
		c.addError(n.Token, fmt.Sprintf("undefined name '%s'", n.Value), hint)
		return JSON
	}
	return t
}

func (c *Checker) listLiteral(n *ast.ListLiteral, sc *scope, expected Type) Type {
	var want Type
	if lt, ok := expected.(*ListType); ok {
		want = lt.Elem
	} else if u, ok := expected.(*UnionType); ok {
		for _, m := range u.Types {
			if lt, ok := m.(*ListType); ok {
				want = lt.Elem
			}
		}
	}
	if want != nil {
		for i, el := range n.Elements {
			et := c.expr(el, sc, want)
			ne, ok := c.fit(want, el, et)
			if !ok {
				c.addError(tokOf(el), fmt.Sprintf("list element has type %s, expected %s", et, want), conversionHint(want, et))
			}
			n.Elements[i] = ne
		}
		return &ListType{Elem: want}
	}
	if len(n.Elements) == 0 {
		return &ListType{Elem: Never}
	}
	first := c.expr(n.Elements[0], sc, nil)
	for i := 1; i < len(n.Elements); i++ {
		et := c.expr(n.Elements[i], sc, first)
		ne, ok := c.fit(first, n.Elements[i], et)
		if !ok {
			c.addError(tokOf(n.Elements[i]), fmt.Sprintf("list elements must all have the same type: expected %s, got %s", first, et),
				"for mixed data use 'list[json]' or a union element type such as 'list[int | string]'")
		}
		n.Elements[i] = ne
	}
	return &ListType{Elem: first}
}

func (c *Checker) mapLiteral(n *ast.MapLiteral, sc *scope, expected Type) Type {
	var wk, wv Type
	if mt, ok := expected.(*MapType); ok {
		wk, wv = mt.Key, mt.Val
	} else if u, ok := expected.(*UnionType); ok {
		for _, m := range u.Types {
			if mt, ok := m.(*MapType); ok {
				wk, wv = mt.Key, mt.Val
			}
		}
	}
	if len(n.Pairs) == 0 {
		if wk != nil {
			return &MapType{Key: wk, Val: wv}
		}
		return &MapType{Key: Never, Val: Never}
	}
	for _, p := range n.Pairs {
		kt := c.expr(p.Key, sc, wk)
		vt := c.expr(p.Value, sc, wv)
		if wk == nil {
			wk = kt
			if wk != String && wk != Int {
				c.addError(tokOf(p.Key), fmt.Sprintf("map keys must be string or int, got %s", kt), "")
			}
		} else if nk, ok := c.fit(wk, p.Key, kt); !ok {
			c.addError(tokOf(p.Key), fmt.Sprintf("map key has type %s, expected %s", kt, wk), "")
		} else {
			p.Key = nk
		}
		if wv == nil {
			wv = vt
		} else if nv, ok := c.fit(wv, p.Value, vt); !ok {
			c.addError(tokOf(p.Value), fmt.Sprintf("map value has type %s, expected %s", vt, wv),
				"for heterogeneous values declare the map as 'map[string, json]'")
		} else {
			p.Value = nv
		}
	}
	return &MapType{Key: wk, Val: wv}
}

// widenLiterals rewrites an int literal operand to float when the other side is float.
func (c *Checker) widenLiterals(n *ast.InfixExpression, lt, rt Type) (Type, Type) {
	if lt == Float && rt == Int {
		if ne, ok := c.fit(Float, n.Right, rt); ok {
			n.Right = ne
			return lt, Float
		}
	}
	if rt == Float && lt == Int {
		if ne, ok := c.fit(Float, n.Left, lt); ok {
			n.Left = ne
			return Float, rt
		}
	}
	return lt, rt
}

func (c *Checker) infix(n *ast.InfixExpression, sc *scope) Type {
	lt := c.expr(n.Left, sc, nil)
	var rexp Type
	if n.Operator != "==" && n.Operator != "!=" && n.Operator != "===" &&
		(lt == Float || lt == Int || lt == String) {
		rexp = lt
	}
	rt := c.expr(n.Right, sc, rexp)
	op := n.Operator
	if lt == Invalid || rt == Invalid {
		return Invalid
	}

	bad := func(msg, hint string) Type {
		c.addError(n.Token, msg, hint)
		return JSON
	}
	if lt == Void || rt == Void {
		return bad("operand has no value (void)", "")
	}

	switch op {
	case "and", "or":
		if lt != Bool || rt != Bool {
			return bad(fmt.Sprintf("'%s' requires bool operands, got %s and %s", op, lt, rt), "compare explicitly instead of relying on truthiness")
		}
		return Bool

	case "==", "!=":
		lt, rt = c.widenLiterals(n, lt, rt)
		if !Comparable(lt, rt) {
			return bad(fmt.Sprintf("cannot compare %s and %s with '%s'", lt, rt, op), conversionHint(lt, rt))
		}
		return Bool

	case "===", "!==":
		return Bool

	case "<", ">", "<=", ">=", "LT_EQ", "GT_EQ", "<=>":
		lt, rt = c.widenLiterals(n, lt, rt)
		if (lt == Int && rt == Int) || (lt == Float && rt == Float) || (lt == String && rt == String) {
			return Bool
		}
		return bad(fmt.Sprintf("cannot order %s and %s", lt, rt), conversionHint(lt, rt))

	case "in":
		switch ct := rt.(type) {
		case *ListType:
			if !Assignable(ct.Elem, lt) && ct.Elem != Never {
				return bad(fmt.Sprintf("'%s' cannot be an element of %s", lt, rt), "")
			}
			return Bool
		case *MapType:
			if !Assignable(ct.Key, lt) && ct.Key != Never {
				return bad(fmt.Sprintf("'%s' cannot be a key of %s", lt, rt), "")
			}
			return Bool
		}
		if rt == String && lt == String {
			return Bool
		}
		if rt == JSON {
			return Bool
		}
		return bad(fmt.Sprintf("'in' needs a list, map or string on the right, got %s", rt), "")

	case "+", "-", "*", "/", "%", "**":
		lt, rt = c.widenLiterals(n, lt, rt)
		switch {
		case lt == Int && rt == Int:
			return Int
		case lt == Float && rt == Float && op != "%":
			return Float
		case op == "+" && lt == String && rt == String:
			return String
		case op == "+":
			ll, lok := lt.(*ListType)
			rl, rok := rt.(*ListType)
			if lok && rok && (Equal(ll, rl) || ll.Elem == Never || rl.Elem == Never) {
				if ll.Elem == Never {
					return rl
				}
				return ll
			}
		}
		hint := conversionHint(lt, rt)
		switch {
		case op == "+" && (lt == String || rt == String):
			hint = "no implicit conversion: use string(x) to build a string, e.g. \"n = \" + string(n)"
		case lt != rt && IsNumeric(lt) && IsNumeric(rt):
			hint = "int and float do not mix: convert explicitly with float(x) or int(x)"
		case op == "%":
			hint = "'%' is defined for int operands only"
		}
		return bad(fmt.Sprintf("operator '%s' cannot be applied to %s and %s", op, lt, rt), hint)
	}
	return bad(fmt.Sprintf("unknown operator '%s'", op), "")
}

func nonNullSide(a, b Type) Type {
	if a == Null {
		return b
	}
	return a
}

func (c *Checker) assign(n *ast.AssignExpression, sc *scope) Type {
	switch tgt := n.Target.(type) {
	case *ast.Identifier:
		sym, _ := sc.lookup(tgt.Value)
		if sym == nil {
			if c.snippet {
				// A REPL snippet may introduce a name by assigning to it; the
				// interpreter creates the binding, so declare it here too.
				vt := c.expr(n.Value, sc, nil)
				c.declare(tgt.Value, tgt.Token, vt, kVar, sc)
				return vt
			}
			hint := fmt.Sprintf("declare it first: '%s: type = value;' or '%s := value;'", tgt.Value, tgt.Value)
			if s := suggest(tgt.Value, sc.names()); s != "" {
				hint = fmt.Sprintf("did you mean '%s'? (to declare a new variable write '%s: type = value;')", s, tgt.Value)
			}
			c.addError(tgt.Token, fmt.Sprintf("assignment to undeclared variable '%s'", tgt.Value), hint)
			c.expr(n.Value, sc, nil)
			return JSON
		}
		if sym.kind != kVar && sym.kind != kParam {
			c.addError(tgt.Token, fmt.Sprintf("cannot assign to %s '%s'", kindName(sym.kind), tgt.Value), "")
			c.expr(n.Value, sc, nil)
			return JSON
		}
		if sym.isConst {
			c.addError(tgt.Token, fmt.Sprintf("cannot assign to constant '%s'", tgt.Value),
				"constants are immutable; declare a normal variable if the value must change")
			c.expr(n.Value, sc, nil)
			return JSON
		}
		vt := c.expr(n.Value, sc, sym.declared)
		ne, ok := c.fit(sym.declared, n.Value, vt)
		if !ok {
			c.addError(tokOf(n.Value), fmt.Sprintf("cannot assign %s to '%s' of type %s", vt, tgt.Value, sym.declared), conversionHint(sym.declared, vt))
		}
		n.Value = ne
		sc.clearNarrow(tgt.Value)
		c.types[tgt] = sym.declared
		return sym.declared

	case *ast.IndexExpression:
		ct := c.expr(tgt.Left, sc, nil)
		it := c.expr(tgt.Index, sc, nil)
		var elem Type = JSON
		switch x := ct.(type) {
		case *ListType:
			if it != Int {
				c.addError(tokOf(tgt.Index), fmt.Sprintf("list index must be int, got %s", it), "")
			}
			elem = x.Elem
		case *MapType:
			if !Assignable(x.Key, it) {
				c.addError(tokOf(tgt.Index), fmt.Sprintf("map key must be %s, got %s", x.Key, it), "")
			}
			elem = x.Val
		default:
			if ct != JSON {
				c.addError(tokOf(tgt.Left), fmt.Sprintf("cannot assign into an element of %s", ct), "")
			}
		}
		vt := c.expr(n.Value, sc, elem)
		ne, ok := c.fit(elem, n.Value, vt)
		if !ok {
			c.addError(tokOf(n.Value), fmt.Sprintf("cannot store %s in an element of type %s", vt, elem), conversionHint(elem, vt))
		}
		n.Value = ne
		return elem

	case *ast.DotExpression:
		ot := c.expr(tgt.Left, sc, nil)
		var ft Type = JSON
		if st, ok := ot.(*StructType); ok {
			f, found := st.LookupField(tgt.Field)
			if !found {
				if _, method := st.LookupMethod(tgt.Field); method {
					c.addError(tgt.Token, fmt.Sprintf("cannot assign to method '%s' of %s", tgt.Field, st.Name), "")
				} else {
					c.addError(tgt.Token, fmt.Sprintf("%s %s has no field '%s'", typeKind(st.IsClass), st.Name, tgt.Field), c.fieldHint(st, tgt.Field))
				}
			} else {
				if f.Visibility == "private" && c.class != f.Owner {
					owner := st.Name
					if f.Owner != nil {
						owner = f.Owner.Name
					}
					c.addError(tgt.Token, fmt.Sprintf("field '%s' of %s is private", tgt.Field, owner), "")
				}
				if f.IsConst {
					c.addError(tgt.Token, fmt.Sprintf("cannot assign to constant field '%s'", tgt.Field),
						"a const field is immutable after construction")
				}
				ft = f.Type
			}
		} else if sp, ok := ot.(*SuperType); ok {
			c.addError(tgt.Token, fmt.Sprintf("cannot assign through 'super'; use 'self.%s'", tgt.Field),
				"super is only for calling base-class methods")
			if f, found := sp.Parent.LookupField(tgt.Field); found {
				ft = f.Type
			}
		} else if ot != JSON {
			c.addError(tgt.Token, fmt.Sprintf("cannot assign to a field of %s", ot), "")
		}
		vt := c.expr(n.Value, sc, ft)
		ne, ok := c.fit(ft, n.Value, vt)
		if !ok {
			c.addError(tokOf(n.Value), fmt.Sprintf("cannot assign %s to field '%s' of type %s", vt, tgt.Field, ft), conversionHint(ft, vt))
		}
		n.Value = ne
		return ft
	}
	c.addError(n.Token, "invalid assignment target", "assignments require a variable name ('x = 1'), index ('a[0] = 1'), or field ('s.f = 1') on the left-hand side")
	c.expr(n.Value, sc, nil)
	return JSON
}

func (c *Checker) fieldHint(st *StructType, name string) string {
	var cands []string
	for _, f := range st.AllFields() {
		cands = append(cands, f.Name)
	}
	for cur := st; cur != nil; cur = cur.Parent {
		for m := range cur.Methods {
			cands = append(cands, m)
		}
	}
	if s := suggest(name, cands); s != "" {
		return fmt.Sprintf("did you mean '%s'?", s)
	}
	return ""
}

func (c *Checker) index(n *ast.IndexExpression, sc *scope) Type {
	ct := c.expr(n.Left, sc, nil)
	if ct == Invalid {
		c.expr(n.Index, sc, nil)
		return Invalid
	}
	if rg, ok := n.Index.(*ast.RangeLiteral); ok { // slice
		c.expr(rg, sc, nil)
		switch ct.(type) {
		case *ListType:
			return ct
		}
		if ct == String || ct == JSON || ct == Blob {
			return ct
		}
		c.addError(n.Token, fmt.Sprintf("cannot slice %s", ct), "")
		return JSON
	}
	switch x := ct.(type) {
	case *ListType:
		it := c.expr(n.Index, sc, Int)
		if it != Int {
			c.addError(tokOf(n.Index), fmt.Sprintf("list index must be int, got %s", it), "")
		}
		if x.Elem == Never {
			return JSON
		}
		return x.Elem
	case *MapType:
		it := c.expr(n.Index, sc, x.Key)
		ne, ok := c.fit(x.Key, n.Index, it)
		if !ok {
			c.addError(tokOf(n.Index), fmt.Sprintf("map key must be %s, got %s", x.Key, it), "")
		}
		n.Index = ne
		return x.Val
	}
	it := c.expr(n.Index, sc, nil)
	switch ct {
	case String:
		if it != Int {
			c.addError(tokOf(n.Index), fmt.Sprintf("string index must be int, got %s", it), "")
		}
		return String
	case Blob:
		return Int
	case JSON:
		return JSON
	}
	c.addError(tokOf(n.Left), fmt.Sprintf("cannot index into %s", ct), "")
	return JSON
}

func (c *Checker) dot(n *ast.DotExpression, sc *scope) Type {
	ot := c.expr(n.Left, sc, nil)
	if ot == Invalid {
		return Invalid
	}
	switch x := ot.(type) {
	case *EnumType:
		if n.Field == "tag" {
			return Int
		}
		if n.Field == "name" {
			return String
		}
		if vt, ok := x.Variants[n.Field]; ok {
			return vt
		}
		c.addError(n.Token, fmt.Sprintf("enum '%s' has no variant '%s'", x.Name, n.Field), "")
		return x
	case *InterfaceType:
		if m, ok := x.Methods[n.Field]; ok {
			return m
		}
		c.addError(n.Token, fmt.Sprintf("interface '%s' has no method '%s'", x.Name, n.Field), "")
		return JSON
	case *ModuleType:
		if t, ok := x.Members[n.Field]; ok {
			return t
		}
		if x.Open {
			return &FuncType{Dynamic: true, Ret: JSON}
		}
		var cands []string
		for m := range x.Members {
			cands = append(cands, m)
		}
		hint := ""
		if s := suggest(n.Field, cands); s != "" {
			hint = fmt.Sprintf("did you mean '%s'?", s)
		}
		c.addError(n.Token, fmt.Sprintf("module '%s' has no member '%s'", x.Name, n.Field), hint)
		return JSON
	case *StructType:
		if f, ok := x.LookupField(n.Field); ok {
			if f.Visibility == "private" && c.class != f.Owner {
				owner := x.Name
				if f.Owner != nil {
					owner = f.Owner.Name
				}
				c.addError(n.Token, fmt.Sprintf("field '%s' of %s is private", n.Field, owner), "")
			}
			return f.Type
		}
		if m, ok := x.LookupMethod(n.Field); ok {
			if m.Visibility == "private" && c.class != m.Owner {
				owner := x.Name
				if m.Owner != nil {
					owner = m.Owner.Name
				}
				c.addError(n.Token, fmt.Sprintf("method '%s' of %s is private", n.Field, owner), "")
			}
			return m.Signature
		}
		c.addError(n.Token, fmt.Sprintf("%s %s has no field or method '%s'", typeKind(x.IsClass), x.Name, n.Field), c.fieldHint(x, n.Field))
		return JSON
	case *SuperType:
		if f, ok := x.Parent.LookupField(n.Field); ok {
			if f.Visibility == "private" {
				c.addError(n.Token, fmt.Sprintf("field '%s' of %s is private", n.Field, x.Parent.Name), "")
			}
			return f.Type
		}
		if m, ok := x.Parent.LookupMethod(n.Field); ok {
			if m.Visibility == "private" {
				c.addError(n.Token, fmt.Sprintf("method '%s' of %s is private", n.Field, x.Parent.Name), "")
			}
			return m.Signature
		}
		c.addError(n.Token, fmt.Sprintf("base class %s has no field or method '%s'", x.Parent.Name, n.Field),
			c.fieldHint(x.Parent, n.Field))
		return JSON
	case *UnionType:
		if Nullable(x) {
			c.addError(n.Token, fmt.Sprintf("cannot access '.%s' on %s: the value may be null", n.Field, x),
				"check it first: 'if value != null:'")
			return JSON
		}
		c.addError(n.Token, fmt.Sprintf("cannot access '.%s' on a union type %s", n.Field, x), "")
		return JSON
	case *FuncType:
		if n.Field == "stream" {
			// fn.stream() runs the commands of a shell function live and
			// returns the output of the last command.
			return &FuncType{Params: x.Params, Ret: x.Ret, Variadic: x.Variadic}
		}
		c.addError(n.Token, fmt.Sprintf("type %s has no field '%s'", ot, n.Field), "")
		return JSON
	case *ListType:
		switch n.Field {
		case "append", "push":
			return &FuncType{Params: []Param{{Name: "elem", Type: x.Elem}}, Ret: x}
		case "pop":
			return &FuncType{Ret: x.Elem}
		case "len":
			return &FuncType{Ret: Int}
		case "sort", "reverse":
			return &FuncType{Ret: x}
		case "contains":
			return &FuncType{Params: []Param{{Name: "elem", Type: x.Elem}}, Ret: Bool}
		case "join":
			return &FuncType{Params: []Param{{Name: "sep", Type: String, Optional: true}}, Ret: String}
		case "map":
			return &FuncType{Dynamic: true, Ret: &ListType{Elem: JSON}}
		case "filter":
			return &FuncType{Dynamic: true, Ret: x}
		case "reduce":
			return &FuncType{Dynamic: true, Ret: JSON}
		case "slice":
			return &FuncType{Params: []Param{{Name: "start", Type: Int}, {Name: "end", Type: Int, Optional: true}}, Ret: x}
		}
		c.addError(n.Token, fmt.Sprintf("list has no method '%s'", n.Field), "")
		return JSON
	case *MapType:
		switch n.Field {
		case "keys":
			return &FuncType{Ret: &ListType{Elem: x.Key}}
		case "values":
			return &FuncType{Ret: &ListType{Elem: x.Val}}
		case "contains":
			return &FuncType{Params: []Param{{Name: "key", Type: x.Key}}, Ret: Bool}
		case "len":
			return &FuncType{Ret: Int}
		case "delete":
			return &FuncType{Params: []Param{{Name: "key", Type: x.Key}}, Ret: Bool}
		}
		c.addError(n.Token, fmt.Sprintf("map has no method '%s'", n.Field), "")
		return JSON
	}
	if ot == String {
		switch n.Field {
		case "split":
			return &FuncType{Params: []Param{{Name: "sep", Type: String, Optional: true}}, Ret: &ListType{Elem: String}}
		case "trim", "strip", "upper", "lower", "capitalize", "title", "reverse", "lstrip", "rstrip":
			return &FuncType{Ret: String}
		case "contains", "starts_with", "ends_with", "is_empty", "is_blank", "is_alpha", "is_digit", "is_numeric", "is_upper", "is_lower":
			return &FuncType{Params: []Param{{Name: "sub", Type: String, Optional: true}}, Ret: Bool}
		case "replace":
			return &FuncType{Params: []Param{{Name: "old", Type: String}, {Name: "new", Type: String}}, Ret: String}
		case "len", "count", "last_index_of":
			return &FuncType{Params: []Param{{Name: "sub", Type: String, Optional: true}}, Ret: Int}
		case "repeat", "pad", "pad_left", "pad_right", "center":
			return &FuncType{Params: []Param{{Name: "count", Type: Int}, {Name: "pad", Type: String, Optional: true}}, Ret: String}
		case "format":
			return &FuncType{Variadic: true, Ret: String}
		case "lines", "words":
			return &FuncType{Ret: &ListType{Elem: String}}
		}
		c.addError(n.Token, fmt.Sprintf("string has no method '%s'", n.Field), "")
		return JSON
	}
	if ot == JSON {
		return JSON
	}
	c.addError(n.Token, fmt.Sprintf("type %s has no field '%s'", ot, n.Field), "")
	return JSON
}

func (c *Checker) lambda(n *ast.LambdaExpression, sc *scope) Type {
	inner := newScope(sc)
	ft := &FuncType{}
	for _, p := range n.Params {
		var pt Type = JSON
		if p.TypeAnnotation == nil {
			c.addError(p.Token, fmt.Sprintf("lambda parameter '%s' needs a type annotation", p.Name), fmt.Sprintf("write '|%s: type| ...'", p.Name))
		} else {
			pt = c.resolve(p.TypeAnnotation)
		}
		ft.Params = append(ft.Params, Param{Name: p.Name, Type: pt})
		inner.syms[p.Name] = &symbol{kind: kParam, typ: pt, declared: pt}
	}
	saved := c.fn
	c.private++
	ft.Ret = c.expr(n.Body, inner, nil)
	c.private--
	c.fn = saved
	if ft.Ret == Null {
		ft.Ret = Void
	}
	return ft
}

func (c *Checker) stream(n *ast.StreamExpression, sc *scope) Type {
	st := c.expr(n.Source, sc, nil)
	op := n.Operator
	switch op {
	case ">>>", "?>>>":
		id, ok := n.Target.(*ast.Identifier)
		if !ok {
			c.addError(tokOf(n.Target), fmt.Sprintf("'%s' needs a variable on the right", op), "")
			return st
		}
		sym, _ := sc.lookup(id.Value)
		if sym == nil || (sym.kind != kVar && sym.kind != kParam) {
			c.addError(id.Token, fmt.Sprintf("undefined variable '%s'", id.Value),
				fmt.Sprintf("declare it first, e.g. '%s: string | null = null;'", id.Value))
			return st
		}
		var captured Type = st
		if op == "?>>>" {
			captured = ErrorType
		}
		if st == Void && op == ">>>" {
			c.addError(tokOf(n.Source), "the left side produces no value to capture", "")
		} else if !Assignable(sym.declared, captured) {
			c.addError(id.Token, fmt.Sprintf("variable '%s' has type %s and cannot hold %s", id.Value, sym.declared, captured),
				fmt.Sprintf("declare it as '%s: %s | null = null;'", id.Value, captured))
		}
		c.types[id] = sym.declared
		sc.clearNarrow(id.Value)
		return st
	case "<<":
		if n.Target != nil {
			if tt := c.expr(n.Target, sc, String); tt != String && tt != JSON {
				c.addError(tokOf(n.Target), fmt.Sprintf("stream source must be a string path, got %s", tt), "")
			}
		}
		return String
	case "|>":
		return JSON
	}
	// >> >! ?>> &>> &&>>  : write to a path/URL
	tt := c.expr(n.Target, sc, String)
	if tt != String && tt != Null && tt != JSON {
		c.addError(tokOf(n.Target), fmt.Sprintf("stream target must be a string path or URL, got %s", tt), "")
	}
	if st == Void {
		c.addError(tokOf(n.Source), "the left side produces no value to write", "")
	}
	return st
}

// ---------------------------------------------------------------------------
// Calls
// ---------------------------------------------------------------------------

func (c *Checker) call(n *ast.CallExpression, sc *scope) Type {
	if id, ok := n.Function.(*ast.Identifier); ok {
		if sym, _ := sc.lookup(id.Value); sym != nil && sym.kind == kBuiltin {
			if t, handled := c.specialCall(id.Value, n, sc); handled {
				return t
			}
		}
	}
	if dot, ok := n.Function.(*ast.DotExpression); ok {
		if id, ok := dot.Left.(*ast.Identifier); ok {
			if id.Value == "list" || id.Value == "map" {
				if t, handled := c.specialCollectionCall(id.Value, dot.Field, n, sc); handled {
					return t
				}
			}
		} else if dot2, ok := dot.Left.(*ast.DotExpression); ok {
			if id2, ok := dot2.Left.(*ast.Identifier); ok && id2.Value == "std" {
				if dot2.Field == "list" || dot2.Field == "map" {
					if t, handled := c.specialCollectionCall(dot2.Field, dot.Field, n, sc); handled {
						return t
					}
				}
			}
		}
	}
	callee := c.expr(n.Function, sc, nil)
	if callee == Invalid {
		c.evalArgsUntyped(n, sc)
		return Invalid
	}

	switch f := callee.(type) {
	case *FuncType:
		if f.Dynamic {
			c.evalArgsUntyped(n, sc)
			return f.Ret
		}
		return c.bindCall(n, f, sc, true)
	case *Overloads:
		return c.overloadCall(n, f, sc)
	case *StructType:
		ctor := &FuncType{Ret: f}
		for _, fld := range f.AllFields() {
			ctor.Params = append(ctor.Params, Param{Name: fld.Name, Type: fld.Type, Optional: fld.HasDefault})
		}
		return c.bindCall(n, ctor, sc, true)
	}
	if callee == JSON {
		c.evalArgsUntyped(n, sc)
		return JSON
	}
	c.addError(tokOf(n.Function), fmt.Sprintf("a value of type %s is not callable", callee), "")
	c.evalArgsUntyped(n, sc)
	return JSON
}

func (c *Checker) evalArgsUntyped(n *ast.CallExpression, sc *scope) {
	for _, a := range n.Arguments {
		if t := c.expr(a, sc, nil); t == Void {
			c.addError(tokOf(a), "argument has no value (void)", "")
		}
	}
	for _, k := range sortedKeys(n.KwArgs) {
		c.expr(n.KwArgs[k], sc, nil)
	}
}

func calleeName(n *ast.CallExpression) string {
	switch f := n.Function.(type) {
	case *ast.Identifier:
		return f.Value
	case *ast.DotExpression:
		if id, ok := f.Left.(*ast.Identifier); ok {
			return id.Value + "." + f.Field
		}
		return f.Field
	}
	return "function"
}

func (c *Checker) bindCall(n *ast.CallExpression, f *FuncType, sc *scope, report bool) Type {
	name := calleeName(n)
	assigned := make([]bool, len(f.Params))

	for i, a := range n.Arguments {
		pi := i
		var pt Type
		switch {
		case i < len(f.Params):
			pt = f.Params[i].Type
		case f.Variadic && len(f.Params) > 0:
			pi = len(f.Params) - 1
			pt = f.Params[pi].Type
		default:
			c.expr(a, sc, nil)
			if report {
				c.addError(tokOf(a), fmt.Sprintf("too many arguments to %s: expected at most %d, got %d", name, len(f.Params), len(n.Arguments)+len(n.KwArgs)), "")
			}
			return f.Ret
		}
		at := c.expr(a, sc, pt)
		if at == Void {
			c.addError(tokOf(a), "argument has no value (void)", "")
		}
		ne, ok := c.fit(pt, a, at)
		if !ok && report {
			pname := f.Params[pi].Name
			if pname != "" {
				pname = " '" + pname + "'"
			}
			c.addError(tokOf(a), fmt.Sprintf("argument %d%s of %s has type %s, expected %s", i+1, pname, name, at, pt), conversionHint(pt, at))
		}
		n.Arguments[i] = ne
		assigned[pi] = true
	}
	for _, k := range sortedKeys(n.KwArgs) {
		idx := -1
		for i, p := range f.Params {
			if p.Name == k {
				idx = i
			}
		}
		a := n.KwArgs[k]
		if idx < 0 {
			c.expr(a, sc, nil)
			var cands []string
			for _, p := range f.Params {
				cands = append(cands, p.Name)
			}
			hint := ""
			if s := suggest(k, cands); s != "" {
				hint = fmt.Sprintf("did you mean '%s'?", s)
			}
			c.addError(tokOf(a), fmt.Sprintf("%s has no parameter named '%s'", name, k), hint)
			continue
		}
		if assigned[idx] {
			c.addError(tokOf(a), fmt.Sprintf("parameter '%s' of %s is given twice", k, name), "")
		}
		assigned[idx] = true
		pt := f.Params[idx].Type
		at := c.expr(a, sc, pt)
		ne, ok := c.fit(pt, a, at)
		if !ok {
			c.addError(tokOf(a), fmt.Sprintf("argument '%s' of %s has type %s, expected %s", k, name, at, pt), conversionHint(pt, at))
		}
		n.KwArgs[k] = ne
	}
	if report {
		for i, p := range f.Params {
			if !assigned[i] && !p.Optional && !(f.Variadic && i == len(f.Params)-1) {
				c.addError(tokOf(n.Function), fmt.Sprintf("missing argument '%s' in call to %s", p.Name, name), "")
				break
			}
		}
	}
	return f.Ret
}

// overloadCall picks the first signature whose parameter types accept the arguments.
func (c *Checker) overloadCall(n *ast.CallExpression, o *Overloads, sc *scope) Type {
	types := make([]Type, len(n.Arguments))
	for i, a := range n.Arguments {
		types[i] = c.expr(a, sc, nil)
	}
	matches := func(sig *FuncType, literalWiden bool) bool {
		if len(n.Arguments) > len(sig.Params) && !sig.Variadic {
			return false
		}
		required := 0
		for _, p := range sig.Params {
			if !p.Optional {
				required++
			}
		}
		if len(n.Arguments) < required {
			return false
		}
		for i, a := range n.Arguments {
			var pt Type
			if i < len(sig.Params) {
				pt = sig.Params[i].Type
			} else {
				pt = sig.Params[len(sig.Params)-1].Type
			}
			if Assignable(pt, types[i]) {
				continue
			}
			if literalWiden && pt == Float && types[i] == Int && isIntLiteral(a) {
				continue
			}
			return false
		}
		return true
	}
	for _, widen := range []bool{false, true} {
		for _, sig := range o.Sigs {
			if matches(sig, widen) {
				for i, a := range n.Arguments {
					var pt Type
					if i < len(sig.Params) {
						pt = sig.Params[i].Type
					} else {
						pt = sig.Params[len(sig.Params)-1].Type
					}
					if ne, ok := c.fit(pt, a, types[i]); ok {
						n.Arguments[i] = ne
					}
				}
				for _, k := range sortedKeys(n.KwArgs) {
					c.expr(n.KwArgs[k], sc, nil)
				}
				return sig.Ret
			}
		}
	}
	var got []string
	for _, t := range types {
		got = append(got, t.String())
	}
	c.addError(tokOf(n.Function), fmt.Sprintf("no matching overload of %s for (%s)", calleeName(n), joinStr(got)),
		"available: "+o.String())
	return o.Sigs[0].Ret
}

func joinStr(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}

// specialCall type-checks the generic built-ins (len, keys, values, push, pop, range, list).
func (c *Checker) specialCall(name string, n *ast.CallExpression, sc *scope) (Type, bool) {
	argc := len(n.Arguments)
	need := func(k int) bool {
		if argc != k || len(n.KwArgs) > 0 {
			c.addError(tokOf(n.Function), fmt.Sprintf("%s() takes %d argument(s), got %d", name, k, argc), "")
			c.evalArgsUntyped(n, sc)
			return false
		}
		return true
	}
	switch name {
	case "len":
		if !need(1) {
			return Int, true
		}
		t := c.expr(n.Arguments[0], sc, nil)
		switch t.(type) {
		case *ListType, *MapType:
			return Int, true
		}
		if t != String && t != Blob && t != JSON {
			c.addError(tokOf(n.Arguments[0]), fmt.Sprintf("len() needs a list, map, string or blob, got %s", t), "")
		}
		return Int, true

	case "keys", "values":
		if !need(1) {
			return &ListType{Elem: JSON}, true
		}
		t := c.expr(n.Arguments[0], sc, nil)
		if m, ok := t.(*MapType); ok {
			if name == "keys" {
				return &ListType{Elem: m.Key}, true
			}
			return &ListType{Elem: m.Val}, true
		}
		if t != JSON {
			c.addError(tokOf(n.Arguments[0]), fmt.Sprintf("%s() needs a map, got %s", name, t), "")
		}
		return &ListType{Elem: JSON}, true

	case "push":
		if !need(2) {
			return &ListType{Elem: JSON}, true
		}
		lt := c.expr(n.Arguments[0], sc, nil)
		l, ok := lt.(*ListType)
		if !ok {
			if lt != JSON {
				c.addError(tokOf(n.Arguments[0]), fmt.Sprintf("push() needs a list as first argument, got %s", lt), "")
			}
			c.expr(n.Arguments[1], sc, nil)
			return &ListType{Elem: JSON}, true
		}
		et := c.expr(n.Arguments[1], sc, l.Elem)
		ne, fits := c.fit(l.Elem, n.Arguments[1], et)
		if !fits && l.Elem != Never {
			c.addError(tokOf(n.Arguments[1]), fmt.Sprintf("cannot push %s onto %s", et, lt), conversionHint(l.Elem, et))
		}
		n.Arguments[1] = ne
		if l.Elem == Never {
			return &ListType{Elem: et}, true
		}
		return lt, true

	case "append":
		if !need(2) {
			return &ListType{Elem: JSON}, true
		}
		lt := c.expr(n.Arguments[0], sc, nil)
		l, ok := lt.(*ListType)
		if !ok {
			if lt != JSON {
				c.addError(tokOf(n.Arguments[0]), fmt.Sprintf("append() needs a list as first argument, got %s", lt), "")
			}
			c.expr(n.Arguments[1], sc, nil)
			return &ListType{Elem: JSON}, true
		}
		et := c.expr(n.Arguments[1], sc, l.Elem)
		ne, fits := c.fit(l.Elem, n.Arguments[1], et)
		if !fits && l.Elem != Never {
			c.addError(tokOf(n.Arguments[1]), fmt.Sprintf("cannot append %s to %s", et, lt), conversionHint(l.Elem, et))
		}
		n.Arguments[1] = ne
		if l.Elem == Never {
			return &ListType{Elem: et}, true
		}
		return lt, true

	case "pop":
		if !need(1) {
			return JSON, true
		}
		lt := c.expr(n.Arguments[0], sc, nil)
		if l, ok := lt.(*ListType); ok {
			if l.Elem == Never {
				return JSON, true
			}
			// pop() on an empty list returns null, so the result is nullable.
			return NewUnion(l.Elem, Null), true
		}
		if lt != JSON {
			c.addError(tokOf(n.Arguments[0]), fmt.Sprintf("pop() needs a list, got %s", lt), "")
		}
		return JSON, true

	case "range":
		if argc < 1 || argc > 3 || len(n.KwArgs) > 0 {
			c.addError(tokOf(n.Function), "range() takes 1 to 3 int arguments", "")
			c.evalArgsUntyped(n, sc)
			return &ListType{Elem: Int}, true
		}
		for _, a := range n.Arguments {
			if t := c.expr(a, sc, Int); t != Int && t != JSON {
				c.addError(tokOf(a), fmt.Sprintf("range() arguments must be int, got %s", t), "")
			}
		}
		return &ListType{Elem: Int}, true

	case "list":
		if !need(1) {
			return &ListType{Elem: JSON}, true
		}
		t := c.expr(n.Arguments[0], sc, nil)
		switch x := t.(type) {
		case *ListType:
			return x, true
		case *MapType:
			return &ListType{Elem: x.Key}, true
		}
		if t == String {
			return &ListType{Elem: String}, true
		}
		return &ListType{Elem: JSON}, true
	}
	return nil, false
}

func (c *Checker) specialCollectionCall(mod, method string, n *ast.CallExpression, sc *scope) (Type, bool) {
	if mod == "list" {
		switch method {
		case "sort", "reverse", "unique":
			if len(n.Arguments) >= 1 {
				t := c.expr(n.Arguments[0], sc, nil)
				if len(n.Arguments) > 1 {
					c.expr(n.Arguments[1], sc, nil)
				}
				if lt, ok := t.(*ListType); ok {
					return lt, true
				}
				return &ListType{Elem: JSON}, true
			}
		case "filter":
			if len(n.Arguments) >= 2 {
				t := c.expr(n.Arguments[0], sc, nil)
				c.expr(n.Arguments[1], sc, nil)
				if lt, ok := t.(*ListType); ok {
					return lt, true
				}
				return &ListType{Elem: JSON}, true
			}
		case "map":
			if len(n.Arguments) >= 2 {
				c.expr(n.Arguments[0], sc, nil)
				fnType := c.expr(n.Arguments[1], sc, nil)
				var retElem Type = JSON
				if ft, ok := fnType.(*FuncType); ok && ft.Ret != nil {
					retElem = ft.Ret
				}
				return &ListType{Elem: retElem}, true
			}
		case "reduce":
			if len(n.Arguments) >= 2 {
				t := c.expr(n.Arguments[0], sc, nil)
				fnType := c.expr(n.Arguments[1], sc, nil)
				if len(n.Arguments) >= 3 {
					initType := c.expr(n.Arguments[2], sc, nil)
					return initType, true
				}
				if ft, ok := fnType.(*FuncType); ok && ft.Ret != nil {
					return ft.Ret, true
				}
				if lt, ok := t.(*ListType); ok {
					return lt.Elem, true
				}
				return JSON, true
			}
		case "keys":
			if len(n.Arguments) >= 1 {
				c.expr(n.Arguments[0], sc, nil)
				return &ListType{Elem: Int}, true
			}
		case "values":
			if len(n.Arguments) >= 1 {
				t := c.expr(n.Arguments[0], sc, nil)
				if lt, ok := t.(*ListType); ok {
					return lt, true
				}
				return &ListType{Elem: JSON}, true
			}
		case "push":
			if len(n.Arguments) >= 2 {
				t := c.expr(n.Arguments[0], sc, nil)
				c.expr(n.Arguments[1], sc, nil)
				if lt, ok := t.(*ListType); ok {
					return lt, true
				}
				return &ListType{Elem: JSON}, true
			}
		case "pop":
			if len(n.Arguments) >= 1 {
				t := c.expr(n.Arguments[0], sc, nil)
				if lt, ok := t.(*ListType); ok {
					return NewUnion(lt.Elem, Null), true
				}
				return JSON, true
			}
		case "slice", "concat":
			if len(n.Arguments) >= 1 {
				t := c.expr(n.Arguments[0], sc, nil)
				for i := 1; i < len(n.Arguments); i++ {
					c.expr(n.Arguments[i], sc, nil)
				}
				if lt, ok := t.(*ListType); ok {
					return lt, true
				}
				return &ListType{Elem: JSON}, true
			}
		}
	} else if mod == "map" {
		switch method {
		case "keys":
			if len(n.Arguments) >= 1 {
				t := c.expr(n.Arguments[0], sc, nil)
				if mt, ok := t.(*MapType); ok {
					return &ListType{Elem: mt.Key}, true
				}
				return &ListType{Elem: String}, true
			}
		case "values":
			if len(n.Arguments) >= 1 {
				t := c.expr(n.Arguments[0], sc, nil)
				if mt, ok := t.(*MapType); ok {
					return &ListType{Elem: mt.Val}, true
				}
				return &ListType{Elem: JSON}, true
			}
		case "sort", "reverse", "filter", "set", "clone", "merge":
			if len(n.Arguments) >= 1 {
				t := c.expr(n.Arguments[0], sc, nil)
				for i := 1; i < len(n.Arguments); i++ {
					c.expr(n.Arguments[i], sc, nil)
				}
				if mt, ok := t.(*MapType); ok {
					return mt, true
				}
				return &MapType{Key: String, Val: JSON}, true
			}
		case "map":
			if len(n.Arguments) >= 2 {
				t := c.expr(n.Arguments[0], sc, nil)
				c.expr(n.Arguments[1], sc, nil)
				if mt, ok := t.(*MapType); ok {
					return mt, true
				}
				return &MapType{Key: String, Val: JSON}, true
			}
		case "reduce":
			if len(n.Arguments) >= 2 {
				t := c.expr(n.Arguments[0], sc, nil)
				fnType := c.expr(n.Arguments[1], sc, nil)
				if len(n.Arguments) >= 3 {
					initType := c.expr(n.Arguments[2], sc, nil)
					return initType, true
				}
				if ft, ok := fnType.(*FuncType); ok && ft.Ret != nil {
					return ft.Ret, true
				}
				if mt, ok := t.(*MapType); ok {
					return mt.Val, true
				}
				return JSON, true
			}
		case "get":
			if len(n.Arguments) >= 2 {
				t := c.expr(n.Arguments[0], sc, nil)
				c.expr(n.Arguments[1], sc, nil)
				if len(n.Arguments) >= 3 {
					defType := c.expr(n.Arguments[2], sc, nil)
					if mt, ok := t.(*MapType); ok {
						return NewUnion(mt.Val, defType), true
					}
					return defType, true
				}
				if mt, ok := t.(*MapType); ok {
					return NewUnion(mt.Val, Null), true
				}
				return JSON, true
			}
		}
	}
	return nil, false
}

var _ = sort.Strings
