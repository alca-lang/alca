// Package typecheck implements Alca's strict static type checker.
//
// Alca is statically and strongly typed: every variable has exactly one type that
// is known at compile time, there are no implicit conversions, no truthiness, and
// 'null' is only valid for types that explicitly include it (T | null).
// The dynamic type 'json' is the only escape hatch; values of type json can be
// indexed/called dynamically but must be converted (int(), string(), ...) before
// they can be used as a concrete type.
package typecheck

import (
	"sort"
	"strings"
)

// Type is a static Alca type.
type Type interface {
	String() string
}

// Basic is a primitive (or opaque built-in) type. Instances are singletons.
type Basic struct{ Name string }

func (b *Basic) String() string { return b.Name }

// Singleton primitive types.
var (
	Int    = &Basic{"int"}
	Float  = &Basic{"float"}
	String = &Basic{"string"}
	Bool   = &Basic{"bool"}
	Null   = &Basic{"null"}
	Void   = &Basic{"void"}
	Blob   = &Basic{"blob"}
	Secret = &Basic{"secret"}
	// JSON is the dynamic type: result of untyped library calls and decoded data.
	JSON = &Basic{"json"}
	// Any is only usable as a *parameter* type of built-ins (accepts every value).
	Any = &Basic{"any"}
	// Never is the type of expressions that never produce a value (err(...)).
	Never = &Basic{"never"}
	// Invalid is the type of an expression that already produced a diagnostic;
	// it is compatible with everything so one mistake yields one error.
	Invalid = &Basic{"<invalid>"}
)

// ListType is list[T].
type ListType struct{ Elem Type }

func (l *ListType) String() string { return "list[" + l.Elem.String() + "]" }

// MapType is map[K, V].
type MapType struct{ Key, Val Type }

func (m *MapType) String() string { return "map[" + m.Key.String() + ", " + m.Val.String() + "]" }

// Param is a function parameter.
type Param struct {
	Name     string
	Type     Type
	Optional bool
}

// FuncType is a callable signature.
type FuncType struct {
	Params   []Param
	Ret      Type
	Variadic bool // last parameter accepts zero or more values of its type
	Dynamic  bool // 'func' with unknown signature: args unchecked, returns json
}

func (f *FuncType) String() string {
	if f.Dynamic {
		return "func"
	}
	var ps []string
	for _, p := range f.Params {
		s := p.Type.String()
		if p.Optional {
			s += "?"
		}
		ps = append(ps, s)
	}
	return "func(" + strings.Join(ps, ", ") + ") -> " + f.Ret.String()
}

// Overloads is a set of alternative signatures (used by polymorphic built-ins).
type Overloads struct{ Sigs []*FuncType }

func (o *Overloads) String() string {
	var s []string
	for _, f := range o.Sigs {
		s = append(s, f.String())
	}
	return strings.Join(s, " | ")
}

// Field is a struct field.
type Field struct {
	Name       string
	Type       Type
	HasDefault bool
}

// StructType is a nominal record type.
type StructType struct {
	Name   string
	Fields []Field
}

func (s *StructType) String() string { return s.Name }

// Field looks up a field by name.
func (s *StructType) Field(name string) (Field, bool) {
	for _, f := range s.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// ErrorType is the built-in type of err(...) values: fields msg, code, time.
var ErrorType = &StructType{Name: "error", Fields: []Field{
	{Name: "msg", Type: String}, {Name: "message", Type: String},
	{Name: "code", Type: Int}, {Name: "time", Type: String},
}}

// UnionType is A | B | ... (flattened, de-duplicated, at least two members).
type UnionType struct{ Types []Type }

func (u *UnionType) String() string {
	var s []string
	for _, t := range u.Types {
		s = append(s, t.String())
	}
	return strings.Join(s, " | ")
}

// ModuleType is a standard-library module namespace.
type ModuleType struct {
	Name    string
	Members map[string]Type
	// Open modules (user modules that could not be analysed) accept any member
	// as a dynamic function returning json.
	Open bool
}

func (m *ModuleType) String() string { return "module " + m.Name }

// NewUnion builds a normalized union (flatten, dedupe, collapse single member).
func NewUnion(ts ...Type) Type {
	var flat []Type
	var add func(t Type)
	add = func(t Type) {
		if u, ok := t.(*UnionType); ok {
			for _, m := range u.Types {
				add(m)
			}
			return
		}
		for _, e := range flat {
			if Equal(e, t) {
				return
			}
		}
		flat = append(flat, t)
	}
	for _, t := range ts {
		add(t)
	}
	if len(flat) == 1 {
		return flat[0]
	}
	return &UnionType{Types: flat}
}

// Equal reports structural type equality.
func Equal(a, b Type) bool {
	if a == b {
		return true
	}
	switch x := a.(type) {
	case *ListType:
		y, ok := b.(*ListType)
		return ok && Equal(x.Elem, y.Elem)
	case *MapType:
		y, ok := b.(*MapType)
		return ok && Equal(x.Key, y.Key) && Equal(x.Val, y.Val)
	case *StructType:
		y, ok := b.(*StructType)
		return ok && x.Name == y.Name
	case *FuncType:
		y, ok := b.(*FuncType)
		if !ok || x.Dynamic != y.Dynamic || len(x.Params) != len(y.Params) || !Equal(x.Ret, y.Ret) {
			return false
		}
		for i := range x.Params {
			if !Equal(x.Params[i].Type, y.Params[i].Type) {
				return false
			}
		}
		return true
	case *UnionType:
		y, ok := b.(*UnionType)
		if !ok || len(x.Types) != len(y.Types) {
			return false
		}
		for _, m := range x.Types {
			found := false
			for _, n := range y.Types {
				if Equal(m, n) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	return false
}

// Assignable reports whether a value of type src may be stored in a slot of type dst.
func Assignable(dst, src Type) bool {
	if dst == Invalid || src == Invalid || Equal(dst, src) {
		return true
	}
	// An empty list/map literal ([] / {}) fits any list/map type.
	if sl, ok := src.(*ListType); ok && sl.Elem == Never {
		if _, isList := dst.(*ListType); isList {
			return true
		}
	}
	if sm, ok := src.(*MapType); ok && sm.Key == Never {
		if _, isMap := dst.(*MapType); isMap {
			return true
		}
	}
	if dst == Any || src == Never {
		return true
	}
	if dst == JSON { // anything may be widened to the dynamic type
		return true
	}
	if su, ok := src.(*UnionType); ok {
		for _, m := range su.Types {
			if !Assignable(dst, m) {
				return false
			}
		}
		return true
	}
	if du, ok := dst.(*UnionType); ok {
		for _, m := range du.Types {
			if Assignable(m, src) {
				return true
			}
		}
		return false
	}
	if f, ok := dst.(*FuncType); ok && f.Dynamic {
		_, isF := src.(*FuncType)
		return isF
	}
	return false
}

// Nullable reports whether t admits null.
func Nullable(t Type) bool {
	if t == Null || t == JSON {
		return true
	}
	if u, ok := t.(*UnionType); ok {
		for _, m := range u.Types {
			if m == Null {
				return true
			}
		}
	}
	return false
}

// StripNull removes null from a union (used for null-check narrowing).
func StripNull(t Type) Type {
	u, ok := t.(*UnionType)
	if !ok {
		return t
	}
	var rest []Type
	for _, m := range u.Types {
		if m != Null {
			rest = append(rest, m)
		}
	}
	if len(rest) == 0 {
		return Null
	}
	return NewUnion(rest...)
}

// IsNumeric reports int or float.
func IsNumeric(t Type) bool { return t == Int || t == Float }

// ParseType parses a type expression such as "list[int]", "map[string, int]" or "int | null".
func ParseType(src string, structs map[string]*StructType) (Type, bool) {
	p := &typeParser{s: src, structs: structs}
	t := p.union()
	p.ws()
	if t == nil || p.i != len(p.s) {
		return nil, false
	}
	return t, true
}

type typeParser struct {
	s       string
	i       int
	structs map[string]*StructType
}

func (p *typeParser) ws() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

func (p *typeParser) union() Type {
	var members []Type
	for {
		t := p.single()
		if t == nil {
			return nil
		}
		members = append(members, t)
		p.ws()
		if p.i < len(p.s) && p.s[p.i] == '|' {
			p.i++
			continue
		}
		break
	}
	return NewUnion(members...)
}

func (p *typeParser) single() Type {
	p.ws()
	start := p.i
	for p.i < len(p.s) && (isIdent(p.s[p.i])) {
		p.i++
	}
	name := p.s[start:p.i]
	if name == "" {
		return nil
	}
	p.ws()
	switch name {
	case "list":
		if p.i < len(p.s) && p.s[p.i] == '[' {
			p.i++
			e := p.union()
			p.ws()
			if e == nil || p.i >= len(p.s) || p.s[p.i] != ']' {
				return nil
			}
			p.i++
			return &ListType{Elem: e}
		}
		return &ListType{Elem: JSON}
	case "map":
		if p.i < len(p.s) && p.s[p.i] == '[' {
			p.i++
			k := p.union()
			p.ws()
			if k == nil || p.i >= len(p.s) || p.s[p.i] != ',' {
				return nil
			}
			p.i++
			v := p.union()
			p.ws()
			if v == nil || p.i >= len(p.s) || p.s[p.i] != ']' {
				return nil
			}
			p.i++
			return &MapType{Key: k, Val: v}
		}
		return &MapType{Key: String, Val: JSON}
	case "int":
		return Int
	case "float":
		return Float
	case "string":
		return String
	case "bool":
		return Bool
	case "null":
		return Null
	case "void":
		return Void
	case "blob":
		return Blob
	case "secret":
		return Secret
	case "json":
		return JSON
	case "any":
		return Any
	case "error":
		return ErrorType
	case "func":
		return &FuncType{Dynamic: true, Ret: JSON}
	}
	if st, ok := p.structs[name]; ok {
		return st
	}
	return nil
}

func isIdent(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// suggest returns the closest candidate (edit distance <= 2) or "".
func suggest(name string, candidates []string) string {
	best, bestD := "", 3
	sort.Strings(candidates)
	for _, c := range candidates {
		if d := levenshtein(name, c); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
