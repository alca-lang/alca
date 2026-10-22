package apm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alca-lang/alca/pkg/version"
)

// Version is a parsed semantic version (semver 2.0.0, without build metadata
// in comparisons).
type Version struct {
	Major int
	Minor int
	Patch int
	Pre   string // prerelease identifiers, without the leading '-'
	Build string // build metadata, without the leading '+'
}

// ParseVersion parses "1.2.3", "1.2" or "1" (missing parts default to zero),
// with optional -prerelease and +build suffixes.
func ParseVersion(s string) (Version, error) {
	orig := strings.TrimSpace(s)
	if orig == "" {
		return Version{}, fmt.Errorf("empty version")
	}
	core := orig
	if i := strings.IndexByte(core, '+'); i >= 0 {
		core, _ = core[:i], core[i+1:]
	}
	build := ""
	if i := strings.IndexByte(orig, '+'); i >= 0 {
		build = orig[i+1:]
	}
	pre := ""
	hasPre := false
	if i := strings.IndexByte(core, '-'); i >= 0 {
		hasPre = true
		core, pre = core[:i], core[i+1:]
	}
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, fmt.Errorf("invalid version %q", s)
	}
	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || strings.HasPrefix(p, "+") || strings.HasPrefix(p, "-") {
			return Version{}, fmt.Errorf("invalid version %q", s)
		}
		nums[i] = n
	}
	if hasPre && !validPreRelease(pre) {
		return Version{}, fmt.Errorf("invalid prerelease in version %q", s)
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre, Build: build}, nil
}

func validPreRelease(pre string) bool {
	for _, id := range strings.Split(pre, ".") {
		if id == "" {
			return false
		}
		for _, r := range id {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
				return false
			}
		}
	}
	return true
}

// String renders the canonical version string.
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare returns -1, 0 or 1 comparing v to o by semver precedence rules.
func (v Version) Compare(o Version) int {
	if c := compareInt(v.Major, o.Major); c != 0 {
		return c
	}
	if c := compareInt(v.Minor, o.Minor); c != 0 {
		return c
	}
	if c := compareInt(v.Patch, o.Patch); c != 0 {
		return c
	}
	switch {
	case v.Pre == "" && o.Pre == "":
		return 0
	case v.Pre == "":
		return 1 // release > prerelease
	case o.Pre == "":
		return -1
	}
	return comparePre(v.Pre, o.Pre)
}

func comparePre(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aerr := strconv.Atoi(as[i])
		bn, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if c := compareInt(an, bn); c != 0 {
				return c
			}
		case aerr == nil:
			return -1 // numeric identifiers sort lower than alphanumeric
		case berr == nil:
			return 1
		default:
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
		}
	}
	return compareInt(len(as), len(bs))
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// Less reports whether v sorts before o.
func (v Version) Less(o Version) bool { return v.Compare(o) < 0 }

// Comparator is one constraint atom, for example ">=1.2.0".
type Comparator struct {
	Op      string // "=", ">", ">=", "<", "<=", "^", "~", "*"
	Version Version
	Any     bool
}

// Constraint is a conjunction of comparators ("^1.0.0 <2.0.0"). The zero
// constraint matches any version.
type Constraint struct {
	Raw   string
	Atoms []Comparator
}

// ParseConstraint parses a dependency constraint. Supported forms:
//
//	"" or "*"        any version
//	1.2.3 or =1.2.3  exact
//	^1.2.3           compatible: >=1.2.3 <2.0.0 (^0.x rules applied)
//	~1.2.3           patch-level: >=1.2.3 <1.3.0
//	>=1.2.3 etc.     comparison
//	">=1.0.0 <2.0.0" conjunction (spaces or commas)
func ParseConstraint(s string) (Constraint, error) {
	raw := strings.TrimSpace(s)
	c := Constraint{Raw: raw}
	if raw == "" || raw == "*" {
		return c, nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	for _, f := range fields {
		atom, err := parseComparator(f)
		if err != nil {
			return Constraint{}, err
		}
		c.Atoms = append(c.Atoms, atom)
	}
	if len(c.Atoms) == 0 {
		return Constraint{}, fmt.Errorf("invalid version constraint %q", s)
	}
	return c, nil
}

func parseComparator(f string) (Comparator, error) {
	op := "="
	rest := f
	for _, cand := range []string{">=", "<=", "^", "~", ">", "<", "="} {
		if strings.HasPrefix(f, cand) {
			op = cand
			rest = strings.TrimSpace(f[len(cand):])
			break
		}
	}
	if rest == "" || rest == "*" {
		if op == "=" || rest == "*" {
			return Comparator{Op: "*", Any: true}, nil
		}
		return Comparator{}, fmt.Errorf("invalid version in constraint %q", f)
	}
	v, err := ParseVersion(rest)
	if err != nil {
		return Comparator{}, fmt.Errorf("invalid version in constraint %q: %w", f, err)
	}
	return Comparator{Op: op, Version: v}, nil
}

// String returns the raw constraint text.
func (c Constraint) String() string { return c.Raw }

// Satisfies reports whether v matches every comparator.
func (c Constraint) Satisfies(v Version) bool {
	for _, a := range c.Atoms {
		if !a.satisfies(v) {
			return false
		}
	}
	return true
}

func (a Comparator) satisfies(v Version) bool {
	if a.Any {
		return true
	}
	cmp := v.Compare(a.Version)
	switch a.Op {
	case "=":
		return cmp == 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case "^":
		if cmp < 0 {
			return false
		}
		upper := caretUpper(a.Version)
		return v.Less(upper)
	case "~":
		if cmp < 0 {
			return false
		}
		upper := a.Version
		upper.Patch = 0
		upper.Pre, upper.Build = "", ""
		upper.Minor++
		return v.Less(upper)
	default:
		return false
	}
}

// caretUpper returns the exclusive upper bound for ^v: the next major version,
// except that 0.x.y stays within the same minor and 0.0.z within the same patch.
func caretUpper(v Version) Version {
	switch {
	case v.Major > 0:
		return Version{Major: v.Major + 1}
	case v.Minor > 0:
		return Version{Minor: v.Minor + 1}
	default:
		return Version{Patch: v.Patch + 1}
	}
}

// AlcaCompatible reports whether the running toolchain satisfies the manifest's
// `alca = ">=x.y.z"` requirement. An empty requirement matches everything.
func AlcaCompatible(requirement string) (bool, error) {
	if strings.TrimSpace(requirement) == "" {
		return true, nil
	}
	c, err := ParseConstraint(requirement)
	if err != nil {
		return false, err
	}
	current, err := ParseVersion(version.Version)
	if err != nil {
		return false, fmt.Errorf("cannot parse toolchain version: %w", err)
	}
	return c.Satisfies(current), nil
}
