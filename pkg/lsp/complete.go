package lsp

import (
	"sort"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/token"
	"github.com/alca-lang/alca/pkg/typecheck"
)

// Completion item kinds as defined by the Language Server Protocol.
const (
	kindFunction = 3
	kindField    = 5
	kindVariable = 6
	kindClass    = 7
	kindModule   = 9
	kindKeyword  = 14
	kindStruct   = 22
)

// completionItem builds one LSP completion entry.
func completionItem(label string, kind int, detail, doc string) map[string]interface{} {
	return map[string]interface{}{
		"label":         label,
		"kind":          kind,
		"detail":        detail,
		"documentation": doc,
	}
}

// completionsFor returns the completion list for a position in a document.
// Inside a member access (receiver.) only the members of the receiver are
// offered; everywhere else the full symbol table is returned.
func (s *Server) completionsFor(uri string, line, char int) []map[string]interface{} {
	text := s.docs[uri]
	prog, checker, _ := analyze(uri, text)

	if receiver := dotReceiver(text, line, char); receiver != "" {
		if items := receiverItems(receiver, prog, checker); len(items) > 0 {
			return items
		}
	}
	return generalCompletions(prog, checker)
}

// generalCompletions lists keywords, primitive types, standard modules,
// built-in functions and the symbols declared in the document.
func generalCompletions(prog *ast.Program, checker *typecheck.Checker) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, 256)

	for _, kw := range token.KeywordNames() {
		items = append(items, completionItem(kw, kindKeyword, "Alca keyword", "Keyword: `"+kw+"`"))
	}
	for _, t := range token.TypeKeywordNames() {
		items = append(items, completionItem(t, kindClass, "Alca primitive type", "Type: `"+t+"`"))
	}
	for _, name := range typecheck.StdModuleNames() {
		items = append(items, completionItem("std."+name, kindModule, "Standard library module", "Standard module: `std."+name+"`"))
	}
	for name, t := range typecheck.GlobalBuiltinTypes() {
		items = append(items, completionItem(name, kindFunction, t.String(), "Builtin: `"+name+"`"))
	}

	if prog != nil {
		items = append(items, documentSymbols(prog, checker)...)
	}
	return items
}

// documentSymbols collects structs, fields, functions and variables declared in
// the document being edited.
func documentSymbols(prog *ast.Program, checker *typecheck.Checker) []map[string]interface{} {
	var items []map[string]interface{}

	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch x := n.(type) {
		case *ast.StructDeclaration:
			items = append(items, completionItem(x.Name, kindStruct, "struct", "Struct declared in this file."))
			for _, f := range x.Fields {
				detail := x.Name + "." + f.Name
				if f.TypeAnnotation != nil {
					detail += ": " + f.TypeAnnotation.String()
				}
				doc := "Field of `" + x.Name + "`."
				if f.DefaultValue != nil {
					doc += " Default: `" + f.DefaultValue.String() + "`."
				}
				items = append(items, completionItem(f.Name, kindField, detail, doc))
			}
		case *ast.ClassDeclaration:
			detail := "class"
			if x.Parent != "" {
				detail = "class extends " + x.Parent
			}
			items = append(items, completionItem(x.Name, kindClass, detail, "Class declared in this file."))
			for _, f := range x.Fields {
				fieldDetail := x.Name + "." + f.Name
				if f.TypeAnnotation != nil {
					fieldDetail += ": " + f.TypeAnnotation.String()
				}
				items = append(items, completionItem(f.Name, kindField, fieldDetail, "Field of `"+x.Name+"`."))
			}
			for _, m := range x.Methods {
				if m == nil || m.Func == nil {
					continue
				}
				items = append(items, completionItem(m.Func.Name, kindFunction, funcSignatureText(m.Func), "Method of `"+x.Name+"`."))
			}
		case *ast.FuncDeclaration:
			items = append(items, completionItem(x.Name, kindFunction, funcSignatureText(x), "Function declared in this file."))
		case *ast.VarDeclaration:
			if x.Name == nil {
				break
			}
			detail := x.Name.Value
			if x.TypeAnnotation != nil {
				detail += ": " + x.TypeAnnotation.String()
			} else if checker != nil && x.Value != nil {
				if t := checker.TypeOf(x.Value); t != nil {
					detail += ": " + t.String()
				}
			}
			items = append(items, completionItem(x.Name.Value, kindVariable, detail, "Variable declared in this file."))
		}
		for _, ch := range childrenOf(n) {
			walk(ch)
		}
	}
	if prog != nil {
		for _, st := range prog.Statements {
			walk(st)
		}
	}
	return items
}

// dotReceiver returns the expression written directly before a trailing '.'
// at the cursor position, or "" when the cursor is not in a member access.
func dotReceiver(text string, line, char int) string {
	if line < 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if line >= len(lines) {
		return ""
	}
	l := lines[line]
	if char > len(l) {
		char = len(l)
	}
	prefix := strings.TrimRight(l[:char], " \t")
	if !strings.HasSuffix(prefix, ".") {
		return ""
	}
	prefix = prefix[:len(prefix)-1]
	i := len(prefix)
	for i > 0 {
		c := prefix[i-1]
		if c == '_' || c == '.' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			i--
			continue
		}
		break
	}
	return prefix[i:]
}

// receiverItems lists the members of a '.' receiver: the std namespace, a
// standard module, or a struct-typed variable declared in the document.
func receiverItems(receiver string, prog *ast.Program, checker *typecheck.Checker) []map[string]interface{} {
	if receiver == "" {
		return nil
	}
	if receiver == "std" {
		var items []map[string]interface{}
		for _, name := range typecheck.StdModuleNames() {
			items = append(items, completionItem(name, kindModule, "std."+name, "Standard module: `std."+name+"`"))
		}
		return items
	}
	if mod := strings.TrimPrefix(receiver, "std."); mod != receiver {
		return moduleMemberItems(mod)
	}
	if typecheck.StdModuleTypes(receiver) != nil {
		return moduleMemberItems(receiver)
	}
	if fields := structFieldsOf(receiver, prog, checker); len(fields) > 0 {
		return fields
	}
	return nil
}

// moduleMemberItems lists the members of a standard module.
func moduleMemberItems(module string) []map[string]interface{} {
	members := typecheck.StdModuleTypes(module)
	if members == nil {
		return nil
	}
	canon := typecheck.CanonicalModuleName(module)
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)

	items := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		items = append(items, completionItem(name, kindFunction, members[name].String(), "Member of `std."+canon+"`."))
	}
	return items
}

// structFieldsOf collects the fields of a struct-typed variable declared in
// the document. The struct type is taken from the checker when available and
// otherwise reconstructed from the struct declaration in the same document.
func structFieldsOf(name string, prog *ast.Program, checker *typecheck.Checker) []map[string]interface{} {
	if checker != nil {
		if st := structTypeOfDeclaredVar(name, prog, checker); st != nil {
			fields := st.AllFields()
			items := make([]map[string]interface{}, 0, len(fields)+len(st.Methods))
			for _, f := range fields {
				items = append(items, completionItem(f.Name, kindField, st.Name+"."+f.Name+": "+f.Type.String(), "Field of `"+st.Name+"`."))
			}
			seen := map[string]bool{}
			for cur := st; cur != nil; cur = cur.Parent {
				for method, m := range cur.Methods {
					if seen[method] {
						continue
					}
					seen[method] = true
					items = append(items, completionItem(method, kindFunction, st.Name+"."+method+" "+m.Signature.String(), "Method of `"+st.Name+"`."))
				}
			}
			return items
		}
	}
	// Fall back to the struct declaration text in the document.
	var structName string
	walkNodes(prog, func(n ast.Node) {
		if structName != "" {
			return
		}
		vd, ok := n.(*ast.VarDeclaration)
		if !ok || vd.Name == nil || vd.Name.Value != name || vd.TypeAnnotation == nil {
			return
		}
		structName = strings.TrimSpace(vd.TypeAnnotation.String())
	})
	if structName == "" {
		return nil
	}
	var items []map[string]interface{}
	walkNodes(prog, func(n ast.Node) {
		switch decl := n.(type) {
		case *ast.StructDeclaration:
			if decl.Name != structName {
				return
			}
			for _, f := range decl.Fields {
				detail := decl.Name + "." + f.Name
				if f.TypeAnnotation != nil {
					detail += ": " + f.TypeAnnotation.String()
				}
				items = append(items, completionItem(f.Name, kindField, detail, "Field of `"+decl.Name+"`."))
			}
		case *ast.ClassDeclaration:
			if decl.Name != structName {
				return
			}
			for _, f := range decl.Fields {
				detail := decl.Name + "." + f.Name
				if f.TypeAnnotation != nil {
					detail += ": " + f.TypeAnnotation.String()
				}
				items = append(items, completionItem(f.Name, kindField, detail, "Field of `"+decl.Name+"`."))
			}
			for _, m := range decl.Methods {
				if m != nil && m.Func != nil {
					items = append(items, completionItem(m.Func.Name, kindFunction, funcSignatureText(m.Func), "Method of `"+decl.Name+"`."))
				}
			}
		}
	})
	return items
}

// structTypeOfDeclaredVar resolves the struct type of a variable declared in
// the document, via the checker's recorded expression types.
func structTypeOfDeclaredVar(name string, prog *ast.Program, checker *typecheck.Checker) *typecheck.StructType {
	var found *typecheck.StructType
	walkNodes(prog, func(n ast.Node) {
		if found != nil {
			return
		}
		vd, ok := n.(*ast.VarDeclaration)
		if !ok || vd.Name == nil || vd.Name.Value != name || vd.Value == nil {
			return
		}
		if st, ok := checker.TypeOf(vd.Value).(*typecheck.StructType); ok {
			found = st
		}
	})
	return found
}

// walkNodes visits every AST node of a program in source order.
func walkNodes(prog *ast.Program, visit func(ast.Node)) {
	if prog == nil {
		return
	}
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		if n == nil {
			return
		}
		visit(n)
		for _, ch := range childrenOf(n) {
			walk(ch)
		}
	}
	for _, st := range prog.Statements {
		walk(st)
	}
}
