package vm

import "strings"

// pureStdlibModules lists the canonical stdlib modules whose members are
// deterministic value transformations with no file, network, process, database,
// GUI, clock, or environment dependency. Direct calls to these modules can be
// fused into OpStdlibCall: the VM and JIT engines can then invoke the registered
// function without executing the generic module lookup and field-access opcodes.
//
// Host-facing members still run through the JIT runtime-helper tier, but are not
// fused as pure calls because their result depends on external state.
var pureStdlibModules = map[string]bool{
	"math":     true,
	"string":   true,
	"str":      true,
	"re":       true,
	"regex":    true,
	"compress": true,
	"json":     true,
}

// mixedPureStdlibMembers lists deterministic value members of modules that also
// expose host, clock, entropy, or file-backed operations. A fused call is emitted
// only for listed members; everything else uses the generic host-call helper.
//
// The formats container is listed by nested member path because formats.csv and
// formats.ini also expose file-backed read/write members, and ai is listed per
// member because ai.matrix seeds random weights.
var mixedPureStdlibMembers = map[string]map[string]bool{
	"formats": {
		"json.encode":    true,
		"json.parse":     true,
		"json.prettify":  true,
		"json.query":     true,
		"json.stringify": true,
		"json.validate":  true,
		"csv.parse":      true,
		"csv.stringify":  true,
		"ini.parse":      true,
		"ini.stringify":  true,
	},
	"csv": {
		"parse":     true,
		"stringify": true,
	},
	"ini": {
		"parse":     true,
		"stringify": true,
	},
	"ai": {
		"add":           true,
		"dense_forward": true,
		"dot":           true,
		"matmul":        true,
		"mse":           true,
		"relu":          true,
		"sigmoid":       true,
		"softmax":       true,
		"tanh":          true,
		"transpose":     true,
	},
	"path": {
		"join":       true,
		"base":       true,
		"dir":        true,
		"ext":        true,
		"stem":       true,
		"clean":      true,
		"is_abs":     true,
		"rel":        true,
		"split":      true,
		"sep":        true,
		"to_slash":   true,
		"from_slash": true,
		"matches":    true,
		"list_sep":   true,
		"split_list": true,
		"join_list":  true,
	},
	"crypto": {
		"sha256":              true,
		"sha512":              true,
		"sha1":                true,
		"md5":                 true,
		"constant_time_equal": true,
		"base64_encode":       true,
		"base64_decode":       true,
		"hex_encode":          true,
		"hex_decode":          true,
		"hmac_sha256":         true,
		"hmac_sha512":         true,
		"aes_decrypt":         true,
		"derive_key":          true,
	},
	"urllib2": {
		"parse":     true,
		"split":     true,
		"join":      true,
		"quote":     true,
		"unquote":   true,
		"urlencode": true,
		"parse_qs":  true,
		"parse_qsl": true,
	},
	"time": {
		"format":             true,
		"parse":              true,
		"parse_duration":     true,
		"diff":               true,
		"unix_ms_to_seconds": true,
		"unix_seconds_to_ms": true,
	},
}

// StdlibBindingName returns the environment binding a stdlib use statement
// installs when no explicit alias is given.
func StdlibBindingName(modulePath string) string {
	modulePath = strings.TrimSpace(modulePath)
	modulePath = strings.TrimPrefix(modulePath, "std.")
	if i := strings.LastIndexByte(modulePath, '.'); i >= 0 {
		modulePath = modulePath[i+1:]
	}
	return modulePath
}

// IsPureStdlibModule reports whether every member of modulePath is pure. Both
// "std.math" and "math" spellings are accepted.
func IsPureStdlibModule(modulePath string) bool {
	return pureStdlibModules[StdlibBindingName(modulePath)]
}

// SupportsPureStdlibMembers reports whether modulePath has at least one member
// that can use the fused pure-call path.
func SupportsPureStdlibMembers(modulePath string) bool {
	name := StdlibBindingName(modulePath)
	return pureStdlibModules[name] || len(mixedPureStdlibMembers[name]) > 0
}

// IsPureStdlibMember reports whether one module member can be fused. memberPath
// may contain nested module components, matching the compiler's lowering path.
func IsPureStdlibMember(modulePath, memberPath string) bool {
	name := StdlibBindingName(modulePath)
	if pureStdlibModules[name] {
		return true
	}
	return mixedPureStdlibMembers[name][memberPath]
}
