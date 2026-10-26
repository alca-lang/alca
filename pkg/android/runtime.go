// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/token"
)

// TranspileAlcaToJS converts Alca source code into JavaScript for execution in WebView.
func TranspileAlcaToJS(source string) (string, error) {
	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return "", fmt.Errorf("parsing Alca: %s", strings.Join(p.Errors(), "; "))
	}
	return TranspileProgram(prog)
}

// TranspileProgram converts an Alca AST program into a self-contained JavaScript bundle.
func TranspileProgram(prog *ast.Program) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(`// Alca Android Runtime Bridge
(function(window) {
    "use strict";

    if (!window.Alca) {
        window.Alca = {
            modules: {},
            listeners: [],
            invoke: function(name) {
                var args = Array.prototype.slice.call(arguments, 1);
                if (typeof window.Alca.modules[name] === "function") {
                    return window.Alca.modules[name].apply(null, args);
                }
                console.warn("[Alca] Function not found in runtime: " + name);
                return null;
            },
            vibrate: function(ms) {
                try {
                    if (window.navigator && window.navigator.vibrate) {
                        window.navigator.vibrate(ms || 30);
                    }
                } catch (e) {}
            },
            toast: function(message) {
                var existing = document.getElementById("__alca_toast");
                if (existing) existing.remove();
                var toast = document.createElement("div");
                toast.id = "__alca_toast";
                toast.innerText = message;
                toast.style.position = "fixed";
                toast.style.bottom = "30px";
                toast.style.left = "50%";
                toast.style.transform = "translateX(-50%)";
                toast.style.backgroundColor = "rgba(15, 23, 42, 0.95)";
                toast.style.color = "#f8fafc";
                toast.style.padding = "10px 20px";
                toast.style.borderRadius = "20px";
                toast.style.fontSize = "13px";
                toast.style.fontWeight = "600";
                toast.style.boxShadow = "0 10px 25px rgba(0,0,0,0.5)";
                toast.style.zIndex = "999999";
                toast.style.border = "1px solid rgba(255,255,255,0.1)";
                toast.style.transition = "opacity 0.3s ease";
                document.body.appendChild(toast);
                setTimeout(function() {
                    toast.style.opacity = "0";
                    setTimeout(function() { toast.remove(); }, 300);
                }, 2000);
            },
            getDeviceInfo: function() {
                return {
                    userAgent: navigator.userAgent,
                    screenWidth: window.innerWidth,
                    screenHeight: window.innerHeight,
                    devicePixelRatio: window.devicePixelRatio || 1,
                    isOnline: navigator.onLine,
                    language: navigator.language || "ru",
                    platform: navigator.platform || "Android"
                };
            },
            share: function(data) {
                if (navigator.share) {
                    return navigator.share(data || {});
                }
                return Promise.reject("Web Share API not supported");
            },
            clipboard: {
                writeText: function(text) {
                    if (navigator.clipboard && navigator.clipboard.writeText) {
                        return navigator.clipboard.writeText(text);
                    }
                    return Promise.reject("Clipboard API not supported");
                },
                readText: function() {
                    if (navigator.clipboard && navigator.clipboard.readText) {
                        return navigator.clipboard.readText();
                    }
                    return Promise.reject("Clipboard API not supported");
                }
            },
            geolocation: {
                getCurrentPosition: function(success, error, opts) {
                    if (navigator.geolocation) {
                        navigator.geolocation.getCurrentPosition(success, error, opts);
                    } else if (error) {
                        error({ code: 0, message: "Geolocation not supported" });
                    }
                },
                watchPosition: function(success, error, opts) {
                    if (navigator.geolocation) {
                        return navigator.geolocation.watchPosition(success, error, opts);
                    }
                    return null;
                }
            },
            storage: {
                get: function(key) {
                    try { return localStorage.getItem(key); } catch (e) { return null; }
                },
                set: function(key, val) {
                    try { localStorage.setItem(key, String(val)); } catch (e) {}
                },
                remove: function(key) {
                    try { localStorage.removeItem(key); } catch (e) {}
                },
                clear: function() {
                    try { localStorage.clear(); } catch (e) {}
                }
            }
        };
    }

    function println() {
        var msg = Array.prototype.slice.call(arguments).join(" ");
        console.log("[Alca]", msg);
    }

    function print() {
        var msg = Array.prototype.slice.call(arguments).join(" ");
        console.log("[Alca]", msg);
    }

    function string(v) {
        if (v === null || v === undefined) return "";
        return String(v);
    }

    function int(v) {
        var n = parseInt(v, 10);
        return isNaN(n) ? 0 : n;
    }

    function float(v) {
        var n = parseFloat(v);
        return isNaN(n) ? 0.0 : n;
    }

    function len(v) {
        if (v === null || v === undefined) return 0;
        if (typeof v === "string" || Array.isArray(v)) return v.length;
        if (typeof v === "object") return Object.keys(v).length;
        return 0;
    }

`)

	for _, stmt := range prog.Statements {
		jsStmt, err := transpileStatement(stmt)
		if err != nil {
			return "", err
		}
		if jsStmt != "" {
			buf.WriteString("    ")
			buf.WriteString(jsStmt)
			buf.WriteString("\n")
		}
	}

	buf.WriteString("})(typeof window !== 'undefined' ? window : this);\n")
	return buf.String(), nil
}

func transpileStatement(stmt ast.Statement) (string, error) {
	if stmt == nil {
		return "", nil
	}
	switch s := stmt.(type) {
	case *ast.VarDeclaration:
		val := "null"
		if s.Value != nil {
			v, err := transpileExpression(s.Value)
			if err != nil {
				return "", err
			}
			val = v
		}
		varName := "v"
		if s.Name != nil {
			varName = s.Name.Value
		}
		keyword := "let"
		if s.IsConst {
			keyword = "const"
		}
		return fmt.Sprintf("%s %s = %s;", keyword, varName, val), nil

	case *ast.FuncDeclaration:
		var params []string
		for _, p := range s.Params {
			if p.Name != "" {
				params = append(params, p.Name)
			}
		}
		body, err := transpileBlock(s.Body)
		if err != nil {
			return "", err
		}

		funcName := s.Name
		var res strings.Builder
		res.WriteString(fmt.Sprintf("function %s(%s) {\n%s\n    }", funcName, strings.Join(params, ", "), body))
		if s.IsPublic {
			res.WriteString(fmt.Sprintf("\n    window.Alca.modules[%q] = %s;", funcName, funcName))
		}
		return res.String(), nil

	case *ast.ReturnStatement:
		if s.Value != nil {
			val, err := transpileExpression(s.Value)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("return %s;", val), nil
		}
		return "return;", nil

	case *ast.ExpressionStatement:
		val, err := transpileExpression(s.Expression)
		if err != nil {
			return "", err
		}
		return val + ";", nil

	case *ast.IfStatement:
		cond, err := transpileExpression(s.Condition)
		if err != nil {
			return "", err
		}
		conseq, err := transpileBlock(s.Consequence)
		if err != nil {
			return "", err
		}

		var res strings.Builder
		res.WriteString(fmt.Sprintf("if (%s) {\n%s\n    }", cond, conseq))

		for _, alt := range s.Alternatives {
			if alt != nil && alt.Condition != nil {
				altCond, err := transpileExpression(alt.Condition)
				if err != nil {
					return "", err
				}
				altBody, err := transpileBlock(alt.Body)
				if err != nil {
					return "", err
				}
				res.WriteString(fmt.Sprintf(" else if (%s) {\n%s\n    }", altCond, altBody))
			}
		}

		if s.ElseBlock != nil {
			alt, err := transpileBlock(s.ElseBlock)
			if err != nil {
				return "", err
			}
			res.WriteString(fmt.Sprintf(" else {\n%s\n    }", alt))
		}
		return res.String(), nil

	case *ast.BlockStatement:
		return transpileBlock(s)

	case *ast.ForStatement:
		body, err := transpileBlock(s.Body)
		if err != nil {
			return "", err
		}
		varName := s.Variable
		if varName == "" {
			varName = "_i"
		}
		if r, ok := s.Iterable.(*ast.RangeLiteral); ok {
			start, err := transpileExpression(r.Start)
			if err != nil {
				return "", err
			}
			end, err := transpileExpression(r.End)
			if err != nil {
				return "", err
			}
			cmp := "<"
			if r.Inclusive {
				cmp = "<="
			}
			return fmt.Sprintf("for (let %s = %s; %s %s %s; %s++) {\n%s\n    }", varName, start, varName, cmp, end, varName, body), nil
		}
		iter, err := transpileExpression(s.Iterable)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("for (let %s of %s) {\n%s\n    }", varName, iter, body), nil

	case *ast.ForEachStatement:
		body, err := transpileBlock(s.Body)
		if err != nil {
			return "", err
		}
		varName := s.Variable
		if varName == "" {
			varName = "_item"
		}
		iter, err := transpileExpression(s.Iterable)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("for (let %s of %s) {\n%s\n    }", varName, iter, body), nil

	case *ast.WhileStatement:
		cond, err := transpileExpression(s.Condition)
		if err != nil {
			return "", err
		}
		body, err := transpileBlock(s.Body)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("while (%s) {\n%s\n    }", cond, body), nil

	case *ast.LoopStatement:
		body, err := transpileBlock(s.Body)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("while (true) {\n%s\n    }", body), nil

	case *ast.BreakStatement:
		return "break;", nil

	case *ast.ContinueStatement:
		return "continue;", nil

	case *ast.AssertStatement:
		cond, err := transpileExpression(s.Expression)
		if err != nil {
			return "", err
		}
		line := nodeLine(s)
		return fmt.Sprintf("if (!(%s)) { throw new Error(\"assertion failed at line %d: %s\"); }", cond, line, strings.ReplaceAll(s.Expression.String(), "\"", "\\\"")), nil

	case *ast.StructDeclaration:
		var fieldNames []string
		var inits []string
		for _, f := range s.Fields {
			if f == nil {
				continue
			}
			fieldNames = append(fieldNames, f.Name)
			defVal := "null"
			if f.DefaultValue != nil {
				dv, err := transpileExpression(f.DefaultValue)
				if err != nil {
					return "", err
				}
				defVal = dv
			}
			inits = append(inits, fmt.Sprintf("        this.%s = (%s !== undefined) ? %s : %s;", f.Name, f.Name, f.Name, defVal))
		}
		paramStr := strings.Join(fieldNames, ", ")
		initStr := strings.Join(inits, "\n")
		code := fmt.Sprintf(`function %s(%s) {
        if (!(this instanceof %s)) return new %s(%s);
%s
    }`, s.Name, paramStr, s.Name, s.Name, paramStr, initStr)
		return code, nil

	case *ast.ClassDeclaration:
		var fieldNames []string
		var inits []string
		for _, f := range s.Fields {
			if f == nil {
				continue
			}
			fieldNames = append(fieldNames, f.Name)
			defVal := "null"
			if f.DefaultValue != nil {
				dv, err := transpileExpression(f.DefaultValue)
				if err != nil {
					return "", err
				}
				defVal = dv
			}
			inits = append(inits, fmt.Sprintf("            this.%s = (%s !== undefined) ? %s : %s;", f.Name, f.Name, f.Name, defVal))
		}

		var methods []string
		for _, m := range s.Methods {
			if m == nil || m.Func == nil {
				continue
			}
			var mParams []string
			for _, p := range m.Func.Params {
				mParams = append(mParams, p.Name)
			}
			mBody, err := transpileBlock(m.Func.Body)
			if err != nil {
				return "", err
			}
			methods = append(methods, fmt.Sprintf("        %s(%s) {\n%s\n        }", m.Func.Name, strings.Join(mParams, ", "), mBody))
		}

		extendsClause := ""
		superCall := ""
		if s.Parent != "" {
			extendsClause = " extends " + s.Parent
			superCall = "            super(...arguments);\n"
		}

		paramStr := strings.Join(fieldNames, ", ")
		initStr := strings.Join(inits, "\n")
		methodStr := strings.Join(methods, "\n\n")

		className := s.Name
		internalClass := className + "_Class"
		code := fmt.Sprintf(`class %s%s {
        constructor(%s) {
%s%s
        }

%s
    }
    function %s(%s) {
        return new %s(%s);
    }
    %s.prototype = %s.prototype;`, internalClass, extendsClause, paramStr, superCall, initStr, methodStr, className, paramStr, internalClass, paramStr, className, internalClass)
		return code, nil

	case *ast.EnumDeclaration:
		var variants []string
		for idx, v := range s.Variants {
			tagVal := fmt.Sprintf("%d", idx)
			if v.Value != nil {
				tv, err := transpileExpression(v.Value)
				if err != nil {
					return "", err
				}
				tagVal = tv
			}
			variants = append(variants, fmt.Sprintf("        %s: Object.freeze({ name: %q, tag: %s })", v.Name, v.Name, tagVal))
		}
		code := fmt.Sprintf("const %s = Object.freeze({\n%s\n    });", s.Name, strings.Join(variants, ",\n"))
		return code, nil

	case *ast.MatchStatement:
		subj, err := transpileExpression(s.Subject)
		if err != nil {
			return "", err
		}
		var cases []string
		for _, c := range s.Cases {
			if c == nil {
				continue
			}
			patt, err := transpileExpression(c.Pattern)
			if err != nil {
				return "", err
			}
			cBody, err := transpileBlock(c.Body)
			if err != nil {
				return "", err
			}
			cases = append(cases, fmt.Sprintf("        case %s: {\n%s\n            break;\n        }", patt, cBody))
		}
		defaultBlock := ""
		if s.Default != nil {
			dBody, err := transpileBlock(s.Default)
			if err != nil {
				return "", err
			}
			defaultBlock = fmt.Sprintf("\n        default: {\n%s\n            break;\n        }", dBody)
		}
		return fmt.Sprintf("switch (%s) {\n%s%s\n    }", subj, strings.Join(cases, "\n"), defaultBlock), nil

	case *ast.UseStatement:
		// use std.<mod>; statements are handled by the runtime / builtins in JS target
		return fmt.Sprintf("// use %s", s.ModulePath), nil

	default:
		line := nodeLine(stmt)
		return "", fmt.Errorf("transpiler error at line %d: unsupported statement %T (%q)", line, stmt, stmt.TokenLiteral())
	}
}

func transpileBlock(block *ast.BlockStatement) (string, error) {
	if block == nil {
		return "", nil
	}
	var lines []string
	for _, s := range block.Statements {
		js, err := transpileStatement(s)
		if err != nil {
			return "", err
		}
		if js != "" {
			lines = append(lines, "        "+js)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func transpileExpression(expr ast.Expression) (string, error) {
	if expr == nil {
		return "null", nil
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		if e.Value == "self" {
			return "this", nil
		}
		if e.Value == "nil" || e.Value == "null" {
			return "null", nil
		}
		return e.Value, nil

	case *ast.IntegerLiteral:
		return fmt.Sprintf("%d", e.Value), nil

	case *ast.FloatLiteral:
		return fmt.Sprintf("%g", e.Value), nil

	case *ast.StringLiteral:
		return fmt.Sprintf("%q", e.Value), nil

	case *ast.BooleanLiteral:
		if e.Value {
			return "true", nil
		}
		return "false", nil

	case *ast.NullLiteral:
		return "null", nil

	case *ast.ListLiteral:
		var items []string
		for _, el := range e.Elements {
			s, err := transpileExpression(el)
			if err != nil {
				return "", err
			}
			items = append(items, s)
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", ")), nil

	case *ast.MapLiteral:
		var pairs []string
		for _, p := range e.Pairs {
			k, err := transpileExpression(p.Key)
			if err != nil {
				return "", err
			}
			v, err := transpileExpression(p.Value)
			if err != nil {
				return "", err
			}
			pairs = append(pairs, fmt.Sprintf("[%s]: %s", k, v))
		}
		return fmt.Sprintf("({ %s })", strings.Join(pairs, ", ")), nil

	case *ast.IndexExpression:
		left, err := transpileExpression(e.Left)
		if err != nil {
			return "", err
		}
		idx, err := transpileExpression(e.Index)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s[%s])", left, idx), nil

	case *ast.DotExpression:
		left, err := transpileExpression(e.Left)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s", left, e.Field), nil

	case *ast.InfixExpression:
		left, err := transpileExpression(e.Left)
		if err != nil {
			return "", err
		}
		right, err := transpileExpression(e.Right)
		if err != nil {
			return "", err
		}

		op := e.Operator
		switch op {
		case "and":
			return fmt.Sprintf("(%s && %s)", left, right), nil
		case "or":
			return fmt.Sprintf("(%s || %s)", left, right), nil
		case "==":
			return fmt.Sprintf("(%s === %s)", left, right), nil
		case "!=":
			return fmt.Sprintf("(%s !== %s)", left, right), nil
		case "===":
			return fmt.Sprintf("(%s === %s)", left, right), nil
		case "!==":
			return fmt.Sprintf("(%s !== %s)", left, right), nil
		case "/":
			// Alca integer division truncated toward zero
			return fmt.Sprintf("Math.trunc((%s) / (%s))", left, right), nil
		case "**":
			return fmt.Sprintf("Math.pow(%s, %s)", left, right), nil
		case "%":
			return fmt.Sprintf("((%s) %% (%s))", left, right), nil
		case "+", "-", "*", "<", "<=", ">", ">=", "&", "|", "^", "<<", ">>":
			return fmt.Sprintf("(%s %s %s)", left, op, right), nil
		default:
			line := nodeLine(e)
			return "", fmt.Errorf("transpiler error at line %d: unsupported binary operator %q", line, op)
		}

	case *ast.PrefixExpression:
		right, err := transpileExpression(e.Right)
		if err != nil {
			return "", err
		}
		op := e.Operator
		switch op {
		case "not", "!":
			return fmt.Sprintf("(!(%s))", right), nil
		case "-", "+", "~":
			return fmt.Sprintf("(%s(%s))", op, right), nil
		default:
			line := nodeLine(e)
			return "", fmt.Errorf("transpiler error at line %d: unsupported prefix operator %q", line, op)
		}

	case *ast.CallExpression:
		fn, err := transpileExpression(e.Function)
		if err != nil {
			return "", err
		}
		var args []string
		for _, arg := range e.Arguments {
			a, err := transpileExpression(arg)
			if err != nil {
				return "", err
			}
			args = append(args, a)
		}
		return fmt.Sprintf("%s(%s)", fn, strings.Join(args, ", ")), nil

	case *ast.AssignExpression:
		left, err := transpileExpression(e.Target)
		if err != nil {
			return "", err
		}
		right, err := transpileExpression(e.Value)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s = %s", left, right), nil

	case *ast.LambdaExpression:
		var params []string
		for _, p := range e.Params {
			params = append(params, p.Name)
		}
		bodyExpr, err := transpileExpression(e.Body)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("((%s) => %s)", strings.Join(params, ", "), bodyExpr), nil

	case *ast.RangeLiteral:
		start, err := transpileExpression(e.Start)
		if err != nil {
			return "", err
		}
		end, err := transpileExpression(e.End)
		if err != nil {
			return "", err
		}
		cmp := "i < end"
		if e.Inclusive {
			cmp = "i <= end"
		}
		return fmt.Sprintf("((start, end) => { const arr = []; for (let i = start; %s; i++) arr.push(i); return arr; })(%s, %s)", cmp, start, end), nil

	case *ast.StructInstantiation:
		var args []string
		for _, arg := range e.Args {
			a, err := transpileExpression(arg)
			if err != nil {
				return "", err
			}
			args = append(args, a)
		}
		return fmt.Sprintf("%s(%s)", e.Name, strings.Join(args, ", ")), nil

	default:
		line := nodeLine(expr)
		return "", fmt.Errorf("transpiler error at line %d: unsupported expression %T (%q)", line, expr, expr.TokenLiteral())
	}
}

func nodeLine(node ast.Node) int {
	if node == nil {
		return 0
	}
	// Try reflection for Token.Line first (works on any AST node holding a Token)
	val := reflect.ValueOf(node)
	if val.Kind() == reflect.Pointer && !val.IsNil() {
		val = val.Elem()
	}
	if val.Kind() == reflect.Struct {
		field := val.FieldByName("Token")
		if field.IsValid() && field.CanInterface() {
			if tok, ok := field.Interface().(token.Token); ok && tok.Line > 0 {
				return tok.Line
			}
		}
	}
	return 0
}
