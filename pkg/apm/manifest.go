// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Package apm implements the Alca package manager: the alca.toml manifest
// format, semantic version constraints, the local package registry, compiled
// package artifacts and the client for remote registry nodes.
package apm

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DefaultEntry is the manifest entry file used when `entry` is omitted.
const DefaultEntry = "src/main.al"

// Package section of alca.toml ([package]).
type Package struct {
	Name        string
	Version     string
	Description string
	Entry       string
	Alca        string
	License     string
	Authors     []string
	Repository  string
}

// Build section of alca.toml ([build]).
type Build struct {
	// Compile enables building a compiled bytecode artifact for the library.
	// Defaults to true.
	Compile bool
	// Optimization is the VM optimization level (0..3). Defaults to 2.
	Optimization int
}

// Manifest is the parsed alca.toml of a library.
type Manifest struct {
	Package      Package
	Dependencies map[string]string // dependency name -> version constraint
	DepOrder     []string          // dependency names in file order (deterministic output)
	Build        Build
	// Extra preserves unknown sections as raw string keys so future manifest
	// extensions remain readable by older toolchains instead of failing.
	Extra map[string]map[string]string
}

var (
	nameRe        = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	importPathRe  = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)*$`)
	versionSemver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

// ValidName reports whether s is a valid package name.
func ValidName(s string) bool { return nameRe.MatchString(s) }

// ValidImportPath reports whether s is a valid module import path
// (dot-separated valid package-name segments).
func ValidImportPath(s string) bool { return importPathRe.MatchString(s) }

// ParseManifest parses and validates an alca.toml document.
func ParseManifest(data []byte) (*Manifest, error) {
	m := &Manifest{
		Dependencies: map[string]string{},
		Build:        Build{Compile: true, Optimization: 2},
		Extra:        map[string]map[string]string{},
	}
	section := ""
	depSeen := map[string]bool{}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") || strings.HasPrefix(line, "[[") {
				return nil, fmt.Errorf("alca.toml:%d: invalid section header %q", i+1, line)
			}
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section == "" {
				return nil, fmt.Errorf("alca.toml:%d: empty section header", i+1)
			}
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("alca.toml:%d: expected key = value, got %q", i+1, line)
		}
		key := strings.TrimSpace(line[:eq])
		valText := strings.TrimSpace(line[eq+1:])
		if strings.HasPrefix(valText, "[") && !strings.HasSuffix(valText, "]") {
			for i+1 < len(lines) && !strings.HasSuffix(valText, "]") {
				i++
				next := strings.TrimSpace(stripComment(lines[i]))
				if next != "" {
					valText += " " + next
				}
			}
		}
		if section == "" {
			return nil, fmt.Errorf("alca.toml:%d: key %q is outside any section", i+1, key)
		}
		val, err := parseTomlValue(valText)
		if err != nil {
			return nil, fmt.Errorf("alca.toml:%d: %s.%s: %w", i+1, section, key, err)
		}
		switch section {
		case "package":
			if err := m.setPackageField(key, val, i+1); err != nil {
				return nil, err
			}
		case "dependencies":
			s, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("alca.toml:%d: dependency %q must be a version constraint string", i+1, key)
			}
			if !ValidName(key) {
				return nil, fmt.Errorf("alca.toml:%d: invalid dependency name %q", i+1, key)
			}
			if !depSeen[key] {
				m.DepOrder = append(m.DepOrder, key)
				depSeen[key] = true
			}
			m.Dependencies[key] = s
		case "build":
			if err := m.setBuildField(key, val, i+1); err != nil {
				return nil, err
			}
		default:
			// Unknown section: keep string values for forward compatibility.
			if m.Extra[section] == nil {
				m.Extra[section] = map[string]string{}
			}
			switch v := val.(type) {
			case string:
				m.Extra[section][key] = v
			case int64:
				m.Extra[section][key] = strconv.FormatInt(v, 10)
			case bool:
				m.Extra[section][key] = strconv.FormatBool(v)
			case []string:
				m.Extra[section][key] = strings.Join(v, ",")
			case []any:
				var parts []string
				for _, item := range v {
					parts = append(parts, fmt.Sprint(item))
				}
				m.Extra[section][key] = strings.Join(parts, ",")
			default:
				m.Extra[section][key] = fmt.Sprint(val)
			}
		}
	}
	if m.Package.Entry == "" {
		m.Package.Entry = DefaultEntry
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manifest) setPackageField(key string, val any, line int) error {
	str := func() (string, error) {
		s, ok := val.(string)
		if !ok {
			return "", fmt.Errorf("alca.toml:%d: package.%s must be a string", line, key)
		}
		return s, nil
	}
	var err error
	switch key {
	case "name":
		m.Package.Name, err = str()
	case "version":
		m.Package.Version, err = str()
	case "description":
		m.Package.Description, err = str()
	case "entry":
		m.Package.Entry, err = str()
	case "alca":
		m.Package.Alca, err = str()
	case "license":
		m.Package.License, err = str()
	case "repository":
		m.Package.Repository, err = str()
	case "authors":
		if arr, ok := val.([]string); ok {
			m.Package.Authors = arr
		} else if s, ok := val.(string); ok {
			m.Package.Authors = []string{s}
		} else {
			return fmt.Errorf("alca.toml:%d: package.authors must be a string array", line)
		}
	default:
		// Unknown package key: accept for forward compatibility.
	}
	return err
}

func (m *Manifest) setBuildField(key string, val any, line int) error {
	switch key {
	case "compile":
		b, ok := val.(bool)
		if !ok {
			return fmt.Errorf("alca.toml:%d: build.compile must be a boolean", line)
		}
		m.Build.Compile = b
	case "optimization":
		n, ok := val.(int)
		if !ok {
			return fmt.Errorf("alca.toml:%d: build.optimization must be an integer", line)
		}
		m.Build.Optimization = n
	default:
		// Unknown build key: accept for forward compatibility.
	}
	return nil
}

// Validate checks the manifest invariants that every consumer relies on.
func (m *Manifest) Validate() error {
	if !ValidName(m.Package.Name) {
		return fmt.Errorf("invalid package name %q (use lowercase letters, digits, '-' or '_')", m.Package.Name)
	}
	if !versionSemver.MatchString(m.Package.Version) {
		return fmt.Errorf("invalid package version %q (expected MAJOR.MINOR.PATCH)", m.Package.Version)
	}
	if _, err := ParseVersion(m.Package.Version); err != nil {
		return fmt.Errorf("invalid package version: %w", err)
	}
	if len(m.Package.Description) > 512 {
		return fmt.Errorf("description is too long (max 512 characters)")
	}
	if m.Package.Alca != "" {
		if _, err := ParseConstraint(m.Package.Alca); err != nil {
			return fmt.Errorf("invalid alca version requirement: %w", err)
		}
	}
	if m.Build.Optimization < 0 || m.Build.Optimization > 3 {
		return fmt.Errorf("build.optimization must be between 0 and 3, got %d", m.Build.Optimization)
	}
	if err := ValidateEntryPath(m.Package.Entry); err != nil {
		return err
	}
	for _, dep := range m.DepOrder {
		if !ValidName(dep) {
			return fmt.Errorf("invalid dependency name %q", dep)
		}
		if dep == m.Package.Name {
			return fmt.Errorf("package %q cannot depend on itself", dep)
		}
		if _, err := ParseConstraint(m.Dependencies[dep]); err != nil {
			return fmt.Errorf("dependency %s: %w", dep, err)
		}
	}
	return nil
}

// ValidateEntryPath checks that an entry path is relative, inside the package
// and an .al file.
func ValidateEntryPath(entry string) error {
	if entry == "" {
		return fmt.Errorf("package entry is empty")
	}
	if strings.Contains(entry, "\\") {
		entry = strings.ReplaceAll(entry, "\\", "/")
	}
	if strings.HasPrefix(entry, "/") || strings.Contains(entry, ":") {
		return fmt.Errorf("package entry %q must be a relative path inside the package", entry)
	}
	for _, part := range strings.Split(entry, "/") {
		if part == ".." {
			return fmt.Errorf("package entry %q must not escape the package directory", entry)
		}
	}
	if !strings.HasSuffix(entry, ".al") {
		return fmt.Errorf("package entry %q must be an .al file", entry)
	}
	return nil
}

// Marshal renders the manifest back to canonical alca.toml text.
func (m *Manifest) Marshal() []byte {
	var b strings.Builder
	b.WriteString("[package]\n")
	writeTomlString(&b, "name", m.Package.Name)
	writeTomlString(&b, "version", m.Package.Version)
	if m.Package.Description != "" {
		writeTomlString(&b, "description", m.Package.Description)
	}
	entry := m.Package.Entry
	if entry == "" {
		entry = DefaultEntry
	}
	writeTomlString(&b, "entry", entry)
	if m.Package.Alca != "" {
		writeTomlString(&b, "alca", m.Package.Alca)
	}
	if m.Package.License != "" {
		writeTomlString(&b, "license", m.Package.License)
	}
	if len(m.Package.Authors) > 0 {
		b.WriteString("authors = [")
		for i, a := range m.Package.Authors {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(strconv.Quote(a))
		}
		b.WriteString("]\n")
	}
	if m.Package.Repository != "" {
		writeTomlString(&b, "repository", m.Package.Repository)
	}
	b.WriteString("\n[dependencies]\n")
	deps := append([]string(nil), m.DepOrder...)
	for dep := range m.Dependencies {
		found := false
		for _, d := range deps {
			if d == dep {
				found = true
				break
			}
		}
		if !found {
			deps = append(deps, dep)
		}
	}
	sort.Strings(deps)
	for _, dep := range deps {
		writeTomlString(&b, dep, m.Dependencies[dep])
	}
	b.WriteString("\n[build]\n")
	b.WriteString("compile = " + strconv.FormatBool(m.Build.Compile) + "\n")
	b.WriteString("optimization = " + strconv.Itoa(m.Build.Optimization) + "\n")
	return []byte(b.String())
}

func writeTomlString(b *strings.Builder, key, value string) {
	b.WriteString(key + " = " + strconv.Quote(value) + "\n")
}

// stripComment removes a trailing # comment that is outside a quoted string.
func stripComment(line string) string {
	inStr := byte(0)
	escaped := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case escaped:
			escaped = false
		case inStr == '"' && ch == '\\':
			escaped = true
		case inStr != 0:
			if ch == inStr {
				inStr = 0
			}
		case ch == '"' || ch == '\'':
			inStr = ch
		case ch == '#':
			return line[:i]
		}
	}
	return line
}

// parseTomlValue parses the supported TOML value forms: basic strings, literal
// strings, integers, booleans, string arrays and inline tables of strings.
func parseTomlValue(s string) (any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("missing value")
	}
	switch s[0] {
	case '"':
		v, rest, err := parseBasicString(s)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(rest) != "" {
			return nil, fmt.Errorf("unexpected trailing text after string: %q", rest)
		}
		return v, nil
	case '\'':
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return nil, fmt.Errorf("unterminated literal string")
		}
		v := s[1 : 1+end]
		if strings.TrimSpace(s[2+end:]) != "" {
			return nil, fmt.Errorf("unexpected trailing text after string")
		}
		return v, nil
	case '[':
		return parseStringArray(s)
	case '{':
		return parseInlineTable(s)
	}
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	return nil, fmt.Errorf("unsupported value %q", s)
}

func parseBasicString(s string) (string, string, error) {
	if len(s) < 2 || s[0] != '"' {
		return "", "", fmt.Errorf("expected a quoted string")
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		ch := s[i]
		switch ch {
		case '"':
			return b.String(), s[i+1:], nil
		case '\\':
			if i+1 >= len(s) {
				return "", "", fmt.Errorf("unterminated escape sequence")
			}
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'u':
				if i+4 >= len(s) {
					return "", "", fmt.Errorf("invalid unicode escape")
				}
				n, err := strconv.ParseUint(s[i+1:i+5], 16, 32)
				if err != nil {
					return "", "", fmt.Errorf("invalid unicode escape")
				}
				b.WriteRune(rune(n))
				i += 4
			default:
				return "", "", fmt.Errorf("unsupported escape \\%c", s[i])
			}
		default:
			b.WriteByte(ch)
		}
		i++
	}
	return "", "", fmt.Errorf("unterminated string")
}

func parseStringArray(s string) ([]string, error) {
	if !strings.HasSuffix(strings.TrimSpace(s), "]") {
		return nil, fmt.Errorf("unterminated array")
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return []string{}, nil
	}
	var out []string
	for _, part := range splitTopLevel(inner) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, _, err := parseBasicString(part)
		if err != nil {
			return nil, fmt.Errorf("array values must be strings: %w", err)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseInlineTable(s string) (map[string]string, error) {
	if !strings.HasSuffix(strings.TrimSpace(s), "}") {
		return nil, fmt.Errorf("unterminated inline table")
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	out := map[string]string{}
	if inner == "" {
		return out, nil
	}
	for _, part := range splitTopLevel(inner) {
		eq := strings.IndexByte(part, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("invalid inline table entry %q", part)
		}
		key := strings.TrimSpace(part[:eq])
		valText := strings.TrimSpace(part[eq+1:])
		val, _, err := parseBasicString(valText)
		if err != nil {
			return nil, fmt.Errorf("inline table %s: %w", key, err)
		}
		out[key] = val
	}
	return out, nil
}

// splitTopLevel splits on commas that are not inside quotes, brackets or braces.
func splitTopLevel(s string) []string {
	var out []string
	depth := 0
	inStr := byte(0)
	escaped := false
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case escaped:
			escaped = false
		case inStr == '"' && ch == '\\':
			escaped = true
		case inStr != 0:
			if ch == inStr {
				inStr = 0
			}
		case ch == '"' || ch == '\'':
			inStr = ch
		case ch == '[' || ch == '{':
			depth++
		case ch == ']' || ch == '}':
			depth--
		case ch == ',' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
