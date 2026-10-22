package vm

import (
	"reflect"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
)

// walkAST calls visit for every pointer value reachable from the AST. The
// standard library compiler lowering needs to answer questions about the whole
// tree, and reflection keeps that walk complete as the AST grows.
func walkAST(value reflect.Value, visit func(reflect.Value)) {
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Interface:
		if !value.IsNil() {
			walkAST(value.Elem(), visit)
		}
	case reflect.Ptr:
		if value.IsNil() {
			return
		}
		if value.CanInterface() {
			visit(value)
		}
		walkAST(value.Elem(), visit)
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			walkAST(value.Field(i), visit)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			walkAST(value.Index(i), visit)
		}
	case reflect.Map:
		iter := value.MapRange()
		for iter.Next() {
			walkAST(iter.Key(), visit)
			walkAST(iter.Value(), visit)
		}
	}
}

// mathAliasEscapes reports whether a std.math module alias is used anywhere
// other than as the receiver of a direct member call, or whether a function
// declaration reuses the alias name.
//
// The compiler lowers math calls to OpMathBuiltin, which bypasses the module
// object and therefore cannot observe a later mutation such as
//
//	x := math
//	x.sqrt = 7
//
// or a module passed to a function that mutates it. Treating any such use as an
// escape keeps the intrinsic only where it is provably equivalent to a normal
// call; all other programs keep the regular VM call path.
func mathAliasEscapes(prog *ast.Program, alias string) bool {
	return moduleAliasEscapes(prog, alias)
}

// moduleAliasEscapes reports whether a stdlib module alias is used anywhere
// other than as the root receiver of a direct member call, or whether a
// function declaration reuses the alias name. Nested module paths such as
// formats.json.encode are treated as one direct call on the root alias.
func moduleAliasEscapes(prog *ast.Program, alias string) bool {
	directCalls := make(map[*ast.Identifier]bool)
	conflict := false
	walkAST(reflect.ValueOf(prog), func(value reflect.Value) {
		switch node := value.Interface().(type) {
		case *ast.CallExpression:
			dot, ok := node.Function.(*ast.DotExpression)
			if !ok {
				return
			}
			if receiver := moduleCallRoot(dot); receiver != nil && receiver.Value == alias {
				directCalls[receiver] = true
			}
		case *ast.FuncDeclaration:
			// A function-shaped binding with the same name can replace the
			// module binding depending on the scope; do not optimize across it.
			if node.Name == alias {
				conflict = true
			}
		}
	})
	if conflict {
		return true
	}

	escaped := false
	walkAST(reflect.ValueOf(prog), func(value reflect.Value) {
		identifier, ok := value.Interface().(*ast.Identifier)
		if ok && identifier.Value == alias && !directCalls[identifier] {
			escaped = true
		}
	})
	return escaped
}

// moduleCallRoot returns the identifier at the root of a dotted member chain.
func moduleCallRoot(dot *ast.DotExpression) *ast.Identifier {
	var expr ast.Expression = dot
	for {
		switch n := expr.(type) {
		case *ast.Identifier:
			return n
		case *ast.DotExpression:
			expr = n.Left
		default:
			return nil
		}
	}
}

// moduleCallPath returns the member path of a dotted call below its root
// identifier. For formats.json.encode it returns "json.encode".
func moduleCallPath(dot *ast.DotExpression) (root *ast.Identifier, path string) {
	var fields []string
	var expr ast.Expression = dot
	for {
		switch n := expr.(type) {
		case *ast.Identifier:
			if len(fields) == 0 {
				return nil, ""
			}
			for i, j := 0, len(fields)-1; i < j; i, j = i+1, j-1 {
				fields[i], fields[j] = fields[j], fields[i]
			}
			return n, strings.Join(fields, ".")
		case *ast.DotExpression:
			fields = append(fields, n.Field)
			expr = n.Left
		default:
			return nil, ""
		}
	}
}
