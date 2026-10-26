// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package stdlib provides the Regex standard library module for the Alca programming language.
// It supports fast pattern matching, searches, capture groups, splits, and replacements using Go's RE2 engine.
package stdlib

import (
	"regexp"
	"sync"

	"github.com/alca-lang/alca/pkg/interpreter"
)

type regexLRUEntry struct {
	pattern string
	re      *regexp.Regexp
	prev    *regexLRUEntry
	next    *regexLRUEntry
}

type regexLRUCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*regexLRUEntry
	head     *regexLRUEntry
	tail     *regexLRUEntry
}

func newRegexLRUCache(capacity int) *regexLRUCache {
	c := &regexLRUCache{
		capacity: capacity,
		entries:  make(map[string]*regexLRUEntry, capacity),
	}
	c.head = &regexLRUEntry{}
	c.tail = &regexLRUEntry{}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

func (c *regexLRUCache) get(pattern string) (*regexp.Regexp, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.entries[pattern]
	if !ok {
		return nil, false
	}
	c.removeNode(node)
	c.addToHead(node)
	return node.re, true
}

func (c *regexLRUCache) put(pattern string, re *regexp.Regexp) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.entries[pattern]; ok {
		node.re = re
		c.removeNode(node)
		c.addToHead(node)
		return
	}

	if len(c.entries) >= c.capacity {
		oldest := c.tail.prev
		if oldest != c.head {
			c.removeNode(oldest)
			delete(c.entries, oldest.pattern)
		}
	}

	node := &regexLRUEntry{pattern: pattern, re: re}
	c.entries[pattern] = node
	c.addToHead(node)
}

func (c *regexLRUCache) removeNode(node *regexLRUEntry) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (c *regexLRUCache) addToHead(node *regexLRUEntry) {
	node.next = c.head.next
	node.prev = c.head
	c.head.next.prev = node
	c.head.next = node
}

var regexCache = newRegexLRUCache(512)

func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	if re, ok := regexCache.get(pattern); ok {
		return re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCache.put(pattern, re)
	return re, nil
}

// RegisterRegexBuiltins registers the "std.regex" and "re" modules into the interpreter environment.
func RegisterRegexBuiltins(env *interpreter.Environment) {
	reModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		reModule.Set(name, bf)
	}

	// match(pattern, text) -> bool
	register("match", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.FALSE
		}
		pattern := interpreter.CoerceToString(args[0])
		text := interpreter.CoerceToString(args[1])
		re, err := getCompiledRegex(pattern)
		if err != nil {
			return interpreter.NewError(400, "invalid regex pattern: "+err.Error())
		}
		if re.MatchString(text) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// find(pattern, text) -> string or null
	register("find", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NULL
		}
		pattern := interpreter.CoerceToString(args[0])
		text := interpreter.CoerceToString(args[1])
		re, err := getCompiledRegex(pattern)
		if err != nil {
			return interpreter.NewError(400, "invalid regex pattern: "+err.Error())
		}
		loc := re.FindString(text)
		if loc == "" && !re.MatchString(text) {
			return interpreter.NULL
		}
		return &interpreter.StringValue{Val: loc}
	})

	// find_all(pattern, text, limit?) -> list of strings
	register("find_all", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewList()
		}
		pattern := interpreter.CoerceToString(args[0])
		text := interpreter.CoerceToString(args[1])
		limit := -1
		if len(args) >= 3 {
			if iv, ok := args[2].(*interpreter.IntegerValue); ok {
				limit = int(iv.Val)
			}
		}
		re, err := getCompiledRegex(pattern)
		if err != nil {
			return interpreter.NewError(400, "invalid regex pattern: "+err.Error())
		}
		matches := re.FindAllString(text, limit)
		vals := make([]interpreter.Value, len(matches))
		for i, m := range matches {
			vals[i] = &interpreter.StringValue{Val: m}
		}
		return interpreter.NewList(vals...)
	})

	// find_groups(pattern, text) -> list of strings or null
	register("find_groups", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NULL
		}
		pattern := interpreter.CoerceToString(args[0])
		text := interpreter.CoerceToString(args[1])
		re, err := getCompiledRegex(pattern)
		if err != nil {
			return interpreter.NewError(400, "invalid regex pattern: "+err.Error())
		}
		submatches := re.FindStringSubmatch(text)
		if len(submatches) == 0 {
			return interpreter.NULL
		}
		vals := make([]interpreter.Value, len(submatches))
		for i, s := range submatches {
			vals[i] = &interpreter.StringValue{Val: s}
		}
		return interpreter.NewList(vals...)
	})

	// find_all_groups(pattern, text, limit?) -> list of lists of strings
	register("find_all_groups", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewList()
		}
		pattern := interpreter.CoerceToString(args[0])
		text := interpreter.CoerceToString(args[1])
		limit := -1
		if len(args) >= 3 {
			if iv, ok := args[2].(*interpreter.IntegerValue); ok {
				limit = int(iv.Val)
			}
		}
		re, err := getCompiledRegex(pattern)
		if err != nil {
			return interpreter.NewError(400, "invalid regex pattern: "+err.Error())
		}
		allMatches := re.FindAllStringSubmatch(text, limit)
		outerVals := make([]interpreter.Value, len(allMatches))
		for i, group := range allMatches {
			innerVals := make([]interpreter.Value, len(group))
			for j, s := range group {
				innerVals[j] = &interpreter.StringValue{Val: s}
			}
			outerVals[i] = interpreter.NewList(innerVals...)
		}
		return interpreter.NewList(outerVals...)
	})

	// replace(pattern, replacement, text) -> string
	register("replace", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "re.replace requires (pattern, replacement, text)")
		}
		pattern := interpreter.CoerceToString(args[0])
		repl := interpreter.CoerceToString(args[1])
		text := interpreter.CoerceToString(args[2])
		re, err := getCompiledRegex(pattern)
		if err != nil {
			return interpreter.NewError(400, "invalid regex pattern: "+err.Error())
		}
		return &interpreter.StringValue{Val: re.ReplaceAllString(text, repl)}
	})

	// split(pattern, text, limit?) -> list of strings
	register("split", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewList()
		}
		pattern := interpreter.CoerceToString(args[0])
		text := interpreter.CoerceToString(args[1])
		limit := -1
		if len(args) >= 3 {
			if iv, ok := args[2].(*interpreter.IntegerValue); ok {
				limit = int(iv.Val)
			}
		}
		re, err := getCompiledRegex(pattern)
		if err != nil {
			return interpreter.NewError(400, "invalid regex pattern: "+err.Error())
		}
		parts := re.Split(text, limit)
		vals := make([]interpreter.Value, len(parts))
		for i, p := range parts {
			vals[i] = &interpreter.StringValue{Val: p}
		}
		return interpreter.NewList(vals...)
	})

	// quote(str) -> escaped string
	register("quote", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		str := interpreter.CoerceToString(args[0])
		return &interpreter.StringValue{Val: regexp.QuoteMeta(str)}
	})

	env.Set("re", reModule)
	env.Set("std.regex", reModule)
	env.Set("std.re", reModule)
}
