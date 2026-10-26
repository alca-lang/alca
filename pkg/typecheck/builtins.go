// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package typecheck

import "strings"

// splitTop splits s on sep, ignoring separators nested inside [] or ().
func splitTop(s string, sep byte) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// parseSig parses a signature DSL string: "(int, string?)->bool;(float)->bool".
// A bare type (no parentheses) denotes a constant of that type.
func parseSig(sig string) Type {
	sig = strings.TrimSpace(sig)
	if sig == "" {
		return &FuncType{Dynamic: true, Ret: JSON}
	}
	if sig[0] != '(' {
		t, ok := ParseType(sig, nil)
		if !ok {
			return JSON
		}
		return t
	}
	var sigs []*FuncType
	for _, alt := range splitTop(sig, ';') {
		alt = strings.TrimSpace(alt)
		close := matchParen(alt)
		if close < 0 {
			continue
		}
		paramSrc := alt[1:close]
		rest := strings.TrimSpace(alt[close+1:])
		rest = strings.TrimPrefix(rest, "->")
		ret, ok := ParseType(strings.TrimSpace(rest), nil)
		if !ok {
			ret = JSON
		}
		f := &FuncType{Ret: ret}
		if strings.TrimSpace(paramSrc) != "" {
			for _, ps := range splitTop(paramSrc, ',') {
				ps = strings.TrimSpace(ps)
				p := Param{}
				if strings.HasPrefix(ps, "...") {
					f.Variadic = true
					ps = strings.TrimPrefix(ps, "...")
				}
				if strings.HasSuffix(ps, "...") {
					f.Variadic = true
					ps = strings.TrimSuffix(ps, "...")
				}
				if strings.HasSuffix(ps, "?") {
					p.Optional = true
					ps = strings.TrimSuffix(ps, "?")
				}
				pt, ok := ParseType(ps, nil)
				if !ok {
					pt = Any
				}
				p.Type = pt
				f.Params = append(f.Params, p)
			}
		}
		sigs = append(sigs, f)
	}
	if len(sigs) == 1 {
		return sigs[0]
	}
	return &Overloads{Sigs: sigs}
}

func matchParen(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

var moduleCache = map[string]*ModuleType{}

var httpRouterStaticType = &StructType{
	Name: "HTTPRouter",
	Methods: map[string]*Method{
		"get":     {Name: "get", Signature: &FuncType{Params: []Param{{Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"post":    {Name: "post", Signature: &FuncType{Params: []Param{{Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"put":     {Name: "put", Signature: &FuncType{Params: []Param{{Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"patch":   {Name: "patch", Signature: &FuncType{Params: []Param{{Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"delete":  {Name: "delete", Signature: &FuncType{Params: []Param{{Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"head":    {Name: "head", Signature: &FuncType{Params: []Param{{Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"options": {Name: "options", Signature: &FuncType{Params: []Param{{Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"route":   {Name: "route", Signature: &FuncType{Params: []Param{{Type: String}, {Type: String}, {Type: Any}, {Type: &MapType{Key: String, Val: JSON}, Optional: true}}, Ret: Bool}},
		"use":     {Name: "use", Signature: &FuncType{Params: []Param{{Type: Any}}, Ret: Bool}},
		"serve":   {Name: "serve", Signature: &FuncType{Params: []Param{{Type: Int}}, Ret: httpServerStaticType}},
		"openapi": {Name: "openapi", Signature: &FuncType{Params: []Param{{Type: String, Optional: true}}, Ret: JSON}},
	},
}

var httpServerStaticType = &StructType{
	Name: "HTTPServer",
	Fields: []Field{
		{Name: "port", Type: Int, Visibility: "public"},
	},
	Methods: map[string]*Method{
		"close": {Name: "close", Signature: &FuncType{Ret: Bool}},
	},
}

var fileHandleStaticType = &StructType{
	Name: "FileHandle",
	Fields: []Field{
		{Name: "path", Type: String, Visibility: "public"},
		{Name: "mode", Type: String, Visibility: "public"},
		{Name: "closed", Type: Bool, Visibility: "public"},
	},
	Methods: map[string]*Method{
		"read":        {Name: "read", Signature: &FuncType{Params: []Param{{Type: Int, Optional: true}}, Ret: String}},
		"read_bytes":  {Name: "read_bytes", Signature: &FuncType{Params: []Param{{Type: Int, Optional: true}}, Ret: Blob}},
		"read_line":   {Name: "read_line", Signature: &FuncType{Ret: String}},
		"lines":       {Name: "lines", Signature: &FuncType{Ret: &ListType{Elem: String}}},
		"write":       {Name: "write", Signature: &FuncType{Params: []Param{{Type: String}}, Ret: Int}},
		"write_bytes": {Name: "write_bytes", Signature: &FuncType{Params: []Param{{Type: Blob}}, Ret: Int}},
		"seek":        {Name: "seek", Signature: &FuncType{Params: []Param{{Type: Int}, {Type: Int, Optional: true}}, Ret: Int}},
		"tell":        {Name: "tell", Signature: &FuncType{Ret: Int}},
		"flush":       {Name: "flush", Signature: &FuncType{Ret: Bool}},
		"close":       {Name: "close", Signature: &FuncType{Ret: Bool}},
		"lock":        {Name: "lock", Signature: &FuncType{Params: []Param{{Type: Bool, Optional: true}}, Ret: Bool}},
		"unlock":      {Name: "unlock", Signature: &FuncType{Ret: Bool}},
	},
}

// moduleByName returns the typed std module (canonical or alias name).
func moduleByName(name string) (*ModuleType, bool) {
	if canon, ok := moduleAliases[name]; ok {
		name = canon
	}
	if m, ok := moduleCache[name]; ok {
		return m, true
	}
	members, ok := moduleMembers[name]
	if !ok {
		return nil, false
	}
	m := &ModuleType{Name: name, Members: make(map[string]Type, len(members))}
	for member, sig := range members {
		m.Members[member] = parseSig(sig)
	}
	if name == "http" {
		m.Members["router"] = &FuncType{Ret: httpRouterStaticType}
	}
	if name == "http" {
		m.Members["http_serve"] = &FuncType{Params: []Param{{Type: Int}, {Type: Any}}, Ret: httpServerStaticType}
	}
	if name == "io" {
		m.Members["open"] = &FuncType{Params: []Param{{Type: String}, {Type: String, Optional: true}}, Ret: fileHandleStaticType}
	}
	moduleCache[name] = m
	return m, true
}

func stdNamespace() *ModuleType {
	ns := &ModuleType{Name: "std", Members: map[string]Type{}}
	for name := range moduleMembers {
		if m, ok := moduleByName(name); ok {
			ns.Members[name] = m
		}
	}
	for alias := range moduleAliases {
		if m, ok := moduleByName(alias); ok {
			ns.Members[alias] = m
		}
	}
	return ns
}

// globalBuiltins returns the type of every always-available built-in function
// that has a fixed (non-generic) signature.
func globalBuiltins() map[string]Type {
	g := map[string]Type{}
	for name, sig := range globalSigs {
		g[name] = parseSig(sig)
	}
	return g
}
