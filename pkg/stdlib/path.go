// Package stdlib provides the Path standard library module for the Alca programming language.
// It supports cross-platform path manipulation, joins, splits, normalization, extensions, and resolution.
package stdlib

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// RegisterPathBuiltins registers the "std.path" module and its functions into the environment.
func RegisterPathBuiltins(env *interpreter.Environment) {
	pathModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		pathModule.Set(name, bf)
	}

	// join(parts...) -> string
	register("join", func(args ...interpreter.Value) interpreter.Value {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = interpreter.CoerceToString(a)
		}
		return &interpreter.StringValue{Val: filepath.Join(parts...)}
	})

	// abs(path) -> string
	register("abs", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "path.abs requires a path argument")
		}
		p := interpreter.CoerceToString(args[0])
		a, err := filepath.Abs(p)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: a}
	})

	// base(path) -> string
	register("base", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: "."}
		}
		p := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: filepath.Base(p)}
	})

	// dir(path) -> string
	register("dir", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: "."}
		}
		p := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: filepath.Dir(p)}
	})

	// ext(path) -> string (e.g. ".al")
	register("ext", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		p := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: filepath.Ext(p)}
	})

	// stem(path) -> string (filename without extension, e.g. "script.test.al" -> "script.test")
	register("stem", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		p := interpreter.CoerceToString(args[0])
		base := filepath.Base(p)
		ext := filepath.Ext(base)
		if ext != "" {
			return &interpreter.StringValue{Val: base[:len(base)-len(ext)]}
		}
		return &interpreter.StringValue{Val: base}
	})

	// clean(path) -> string
	register("clean", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: "."}
		}
		p := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: filepath.Clean(p)}
	})

	// is_abs(path) -> bool
	register("is_abs", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.FALSE
		}
		p := interpreter.CoerceToString(args[0])
		if filepath.IsAbs(p) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// rel(base, target) -> string
	register("rel", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "path.rel requires (base, target) arguments")
		}
		base := interpreter.CoerceToString(args[0])
		target := interpreter.CoerceToString(args[1])
		r, err := filepath.Rel(base, target)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: r}
	})

	// split(path) -> list [dir, file]
	register("split", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewList(&interpreter.StringValue{Val: "."}, &interpreter.StringValue{Val: ""})
		}
		p := interpreter.CoerceToString(args[0])
		dir, file := filepath.Split(p)
		return interpreter.NewList(&interpreter.StringValue{Val: dir}, &interpreter.StringValue{Val: file})
	})

	// expand(path) -> string (expands ~ to home dir and environment variables)
	register("expand", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		p := interpreter.CoerceToString(args[0])
		if strings.HasPrefix(p, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				if p == "~" {
					p = home
				} else if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
					p = filepath.Join(home, p[2:])
				}
			}
		}
		p = os.ExpandEnv(p)
		return &interpreter.StringValue{Val: p}
	})

	// sep() -> string
	register("sep", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: string(os.PathSeparator)}
	})

	// to_slash(path) -> string (converts '\' to '/')
	register("to_slash", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		p := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: filepath.ToSlash(p)}
	})

	// from_slash(path) -> string (converts '/' to system separator)
	register("from_slash", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		p := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: filepath.FromSlash(p)}
	})

	// matches(pattern, name) -> bool
	register("matches", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.FALSE
		}
		pattern := interpreter.CoerceToString(args[0])
		name := interpreter.CoerceToString(args[1])
		matched, err := filepath.Match(pattern, name)
		if err != nil || !matched {
			return interpreter.FALSE
		}
		return interpreter.TRUE
	})

	// which(binary_name) -> string or null
	register("which", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NULL
		}
		bin := interpreter.CoerceToString(args[0])
		p, err := Which(bin)
		if err != nil {
			return interpreter.NULL
		}
		return &interpreter.StringValue{Val: p}
	})

	// list_sep() -> string (';' on Windows, ':' on Unix)
	register("list_sep", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.StringValue{Val: string(os.PathListSeparator)}
	})

	// split_list(pathListStr) -> list of strings
	register("split_list", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewList()
		}
		str := interpreter.CoerceToString(args[0])
		parts := filepath.SplitList(str)
		vals := make([]interpreter.Value, len(parts))
		for i, p := range parts {
			vals[i] = &interpreter.StringValue{Val: p}
		}
		return interpreter.NewList(vals...)
	})

	// join_list(dirs...) -> string
	register("join_list", func(args ...interpreter.Value) interpreter.Value {
		var parts []string
		if len(args) == 1 {
			if lv, ok := args[0].(*interpreter.ListValue); ok {
				for _, elem := range lv.Elements {
					parts = append(parts, interpreter.CoerceToString(elem))
				}
				return &interpreter.StringValue{Val: strings.Join(parts, string(os.PathListSeparator))}
			}
		}
		for _, a := range args {
			parts = append(parts, interpreter.CoerceToString(a))
		}
		return &interpreter.StringValue{Val: strings.Join(parts, string(os.PathListSeparator))}
	})

	env.Set("path", pathModule)
	env.Set("std.path", pathModule)
}
