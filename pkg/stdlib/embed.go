// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/alca-lang/alca/pkg/interpreter"
)

var embeddedResources = struct {
	sync.RWMutex
	files map[string][]byte
}{files: map[string][]byte{}}

// SetEmbeddedResources replaces the resource set exposed to standalone programs.
func SetEmbeddedResources(files map[string][]byte) {
	copyFiles := make(map[string][]byte, len(files))
	for name, data := range files {
		copyFiles[normalizeEmbeddedPath(name)] = append([]byte(nil), data...)
	}
	embeddedResources.Lock()
	embeddedResources.files = copyFiles
	embeddedResources.Unlock()
}

func normalizeEmbeddedPath(name string) string {
	return path.Clean(strings.TrimPrefix(strings.ReplaceAll(name, "\\", "/"), "./"))
}

// ReadEmbeddedResource returns an embedded resource by its project-relative path.
func ReadEmbeddedResource(name string) ([]byte, error) {
	name = normalizeEmbeddedPath(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
		return nil, fmt.Errorf("invalid embedded resource path %q", name)
	}
	embeddedResources.RLock()
	defer embeddedResources.RUnlock()
	data, ok := embeddedResources.files[name]
	if !ok {
		return nil, fmt.Errorf("embedded resource %q was not included at build time", name)
	}
	return append([]byte(nil), data...), nil
}

// RegisterEmbedBuiltins registers read-only access to resources packaged by `alca build --embed`.
func RegisterEmbedBuiltins(env *interpreter.Environment) {
	module := interpreter.NewMap()
	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		module.Set(name, builtin)
		env.Set("embed_"+name, builtin)
	}

	register("read", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "embed.read requires a resource path")
		}
		data, err := ReadEmbeddedResource(interpreter.CoerceToString(args[0]))
		if err != nil {
			return interpreter.NewError(404, err.Error())
		}
		return &interpreter.StringValue{Val: string(data)}
	})
	register("exists", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "embed.exists requires a resource path")
		}
		_, err := ReadEmbeddedResource(interpreter.CoerceToString(args[0]))
		if err != nil {
			return interpreter.FALSE
		}
		return interpreter.TRUE
	})
	register("list", func(args ...interpreter.Value) interpreter.Value {
		prefix := ""
		if len(args) > 1 {
			return interpreter.NewError(400, "embed.list accepts at most one directory path")
		}
		if len(args) == 1 {
			prefix = normalizeEmbeddedPath(interpreter.CoerceToString(args[0]))
			if prefix == "." {
				prefix = ""
			}
			if prefix == ".." || strings.HasPrefix(prefix, "../") || strings.HasPrefix(prefix, "/") {
				return interpreter.NewError(400, "embed.list path must be inside the embedded resources")
			}
			if prefix != "" {
				prefix += "/"
			}
		}
		embeddedResources.RLock()
		names := make([]string, 0, len(embeddedResources.files))
		for name := range embeddedResources.files {
			if strings.HasPrefix(name, prefix) {
				names = append(names, name)
			}
		}
		embeddedResources.RUnlock()
		sort.Strings(names)
		items := make([]interpreter.Value, len(names))
		for i, name := range names {
			items[i] = &interpreter.StringValue{Val: name}
		}
		return interpreter.NewList(items...)
	})

	env.Set("embed", module)
	env.Set("std.embed", module)
}
