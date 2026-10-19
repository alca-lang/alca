// Package sandbox implements the runtime capability policy used by the Alca
// CLI and standalone executables.
//
// Programs are denied file-system and network access by default. The CLI turns
// selected capabilities on with --allow-read, --allow-write, --allow-net and
// --allow-all. Library users and the test suite keep the permissive default by
// calling ResetPolicy.
package sandbox

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

// Capability grants access either everywhere (All) or only under the listed
// roots/hosts. The zero value denies everything.
type Capability struct {
	All   bool
	Roots []string
}

// Policy is the active capability set.
type Policy struct {
	Read  Capability
	Write Capability
	Net   Capability
}

// AllowAll returns a policy that grants every capability.
func AllowAll() Policy {
	return Policy{
		Read:  Capability{All: true},
		Write: Capability{All: true},
		Net:   Capability{All: true},
	}
}

// DenyAll returns the default policy: no file-system or network access.
func DenyAll() Policy { return Policy{} }

var current atomic.Pointer[Policy]

func init() {
	p := AllowAll()
	current.Store(&p)
}

// SetPolicy installs p process-wide. Builtins consult it at call time, so both
// the VM and the tree-walking interpreter see the same permissions.
func SetPolicy(p Policy) {
	p = p.clone()
	current.Store(&p)
}

// CurrentPolicy returns a copy of the active policy.
func CurrentPolicy() Policy { return current.Load().clone() }

// ResetPolicy restores the permissive library default.
func ResetPolicy() { SetPolicy(AllowAll()) }

func (p Policy) clone() Policy {
	p.Read.Roots = append([]string(nil), p.Read.Roots...)
	p.Write.Roots = append([]string(nil), p.Write.Roots...)
	p.Net.Roots = append([]string(nil), p.Net.Roots...)
	return p
}

// DeniedError is returned when an operation exceeds the active policy. It
// unwraps to os.ErrPermission, so ordinary permission diagnostics work too.
type DeniedError struct {
	Operation string
	Resource  string
	Flag      string
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("permission denied: %s access to %q is not allowed (pass %s)", e.Operation, e.Resource, e.Flag)
}

// Unwrap makes errors.Is(err, os.ErrPermission) true.
func (e *DeniedError) Unwrap() error { return os.ErrPermission }

// RequireRead checks read access to path against the active policy.
func RequireRead(path string) error {
	if CurrentPolicy().Read.AllowsPath(path) {
		return nil
	}
	return &DeniedError{Operation: "read", Resource: path, Flag: "--allow-read[=<path>]"}
}

// RequireWrite checks write access to path against the active policy.
func RequireWrite(path string) error {
	if CurrentPolicy().Write.AllowsPath(path) {
		return nil
	}
	return &DeniedError{Operation: "write", Resource: path, Flag: "--allow-write[=<path>]"}
}

// RequireNet checks network access to hostport ("host", "host:port" or ":port").
func RequireNet(hostport string) error {
	if CurrentPolicy().Net.AllowsHost(hostport) {
		return nil
	}
	return &DeniedError{Operation: "network", Resource: hostport, Flag: "--allow-net[=<host[:port]>]"}
}

// RequireURL checks network access for an HTTP(S) or WebSocket URL.
func RequireURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		// Let the real client produce the malformed-URL error.
		return nil
	}
	host := u.Hostname()
	if port := u.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	return RequireNet(host)
}

// AllowsPath reports whether path is inside an allowed root.
func (c Capability) AllowsPath(path string) bool {
	if c.All {
		return true
	}
	if len(c.Roots) == 0 {
		return false
	}
	target := canonicalPath(path)
	for _, root := range c.Roots {
		if pathWithin(canonicalPath(root), target) {
			return true
		}
	}
	return false
}

// AllowsHost reports whether hostport matches an allowed host/port rule.
func (c Capability) AllowsHost(hostport string) bool {
	if c.All {
		return true
	}
	host, port := splitHostPort(hostport)
	for _, rule := range c.Roots {
		ruleHost, rulePort := splitHostPort(stripScheme(rule))
		if ruleHost == "*" && rulePort == "" {
			return true
		}
		if ruleHost != "" && strings.HasPrefix(ruleHost, "*.") {
			suffix := strings.ToLower(ruleHost[1:])
			if strings.HasSuffix(strings.ToLower(host), suffix) && !strings.EqualFold(host, ruleHost[2:]) {
				if rulePort == "" || rulePort == port {
					return true
				}
			}
			continue
		}
		if strings.EqualFold(host, ruleHost) && (rulePort == "" || rulePort == port) {
			return true
		}
	}
	return false
}

func stripScheme(s string) string {
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

func splitHostPort(s string) (host, port string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if strings.HasPrefix(s, ":") {
		return "", strings.TrimPrefix(s, ":")
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		return h, p
	}
	return s, ""
}

// canonicalPath returns a cleaned absolute path, resolving symlinks in the
// longest existing prefix so a symlink cannot be used to escape a root.
func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}

	// The file may not exist yet (write access). Resolve the deepest existing
	// ancestor and append the remaining components.
	var suffix []string
	dir := abs
	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		suffix = append(suffix, filepath.Base(dir))
		dir = parent
	}
}

// pathWithin reports whether target is root itself or lives beneath it.
func pathWithin(root, target string) bool {
	if runtime.GOOS == "windows" {
		root = strings.ToLower(root)
		target = strings.ToLower(target)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	return true
}

// ParseArgs consumes leading --allow-* flags and returns the remaining
// arguments (entry file and/or script arguments). Unknown options are left for
// the caller, so scripts can still receive their own flags after the file name.
func ParseArgs(args []string) (Policy, []string, error) {
	policy := DenyAll()
	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if arg == "--allow-all" {
			policy = AllowAll()
			continue
		}
		if !strings.HasPrefix(arg, "--allow-") {
			break
		}
		if err := policy.applyFlag(arg); err != nil {
			return DenyAll(), nil, err
		}
	}
	return policy, args[i:], nil
}

func (p *Policy) applyFlag(arg string) error {
	for _, item := range []struct {
		name string
		cap  *Capability
	}{
		{"--allow-read", &p.Read},
		{"--allow-write", &p.Write},
		{"--allow-net", &p.Net},
	} {
		if arg == item.name {
			item.cap.All = true
			return nil
		}
		prefix := item.name + "="
		if !strings.HasPrefix(arg, prefix) {
			continue
		}
		value := strings.TrimPrefix(arg, prefix)
		if value == "" {
			return fmt.Errorf("%s requires a non-empty value", item.name)
		}
		for _, spec := range strings.Split(value, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				return fmt.Errorf("%s contains an empty value", item.name)
			}
			if item.name != "--allow-net" {
				abs, err := filepath.Abs(spec)
				if err != nil {
					return fmt.Errorf("%s: invalid path %q: %w", item.name, spec, err)
				}
				spec = filepath.Clean(abs)
			}
			item.cap.Roots = append(item.cap.Roots, spec)
		}
		return nil
	}
	return fmt.Errorf("unknown option %q", arg)
}

// FlagHint is the help text for the capability flags.
const FlagHint = "--allow-read[=<path>] --allow-write[=<path>] --allow-net[=<host[:port]>] --allow-all"
