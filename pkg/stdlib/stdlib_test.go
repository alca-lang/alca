package stdlib_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestTimeModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterTimeBuiltins(env)

	val, ok := env.Get("time")
	if !ok {
		t.Fatalf("expected 'time' module to be registered")
	}
	m, ok := val.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("expected 'time' to be *MapValue, got %T", val)
	}

	// now
	nowFn, _ := m.Get("now")
	nowVal := nowFn.(*interpreter.BuiltinFunction).Fn()
	sec := nowVal.(*interpreter.IntegerValue).Val
	if sec <= 0 {
		t.Errorf("expected positive timestamp, got %d", sec)
	}

	// now_ms
	nowMsFn, _ := m.Get("now_ms")
	nowMsVal := nowMsFn.(*interpreter.BuiltinFunction).Fn()
	ms := nowMsVal.(*interpreter.IntegerValue).Val
	if ms < sec*1000 {
		t.Errorf("expected ms >= sec*1000, got %d vs %d", ms, sec)
	}

	// now_str
	nowStrFn, _ := m.Get("now_str")
	sVal := nowStrFn.(*interpreter.BuiltinFunction).Fn()
	s := sVal.(*interpreter.StringValue).Val
	if len(s) < 10 || !strings.Contains(s, "-") {
		t.Errorf("expected date string, got %q", s)
	}

	// format and parse
	fmtFn, _ := m.Get("format")
	formatted := fmtFn.(*interpreter.BuiltinFunction).Fn(nowVal, &interpreter.StringValue{Val: "YYYY-MM-DD"})
	dateStr := formatted.(*interpreter.StringValue).Val
	if len(dateStr) != 10 {
		t.Errorf("expected YYYY-MM-DD date, got %q", dateStr)
	}

	parseFn, _ := m.Get("parse")
	parsed := parseFn.(*interpreter.BuiltinFunction).Fn(formatted, &interpreter.StringValue{Val: "YYYY-MM-DD"})
	if _, ok := parsed.(*interpreter.IntegerValue); !ok {
		t.Errorf("expected integer timestamp from parse, got %v", parsed)
	}

	// date
	dateFn, _ := m.Get("date")
	dMap := dateFn.(*interpreter.BuiltinFunction).Fn(nowVal).(*interpreter.MapValue)
	yearVal, _ := dMap.Get("year")
	if yearVal.(*interpreter.IntegerValue).Val < 2024 {
		t.Errorf("unexpected year %v", yearVal)
	}

	// diff
	diffFn, _ := m.Get("diff")
	diffVal := diffFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 100}, &interpreter.IntegerValue{Val: 40})
	if diffVal.(*interpreter.FloatValue).Val != 60.0 {
		t.Errorf("expected diff 60, got %v", diffVal)
	}
}

func TestPathModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterPathBuiltins(env)

	val, ok := env.Get("path")
	if !ok {
		t.Fatalf("expected 'path' module to be registered")
	}
	m := val.(*interpreter.MapValue)

	// join
	joinFn, _ := m.Get("join")
	joined := joinFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "folder"},
		&interpreter.StringValue{Val: "sub"},
		&interpreter.StringValue{Val: "file.al"},
	).(*interpreter.StringValue).Val
	if !strings.Contains(joined, "folder") || !strings.Contains(joined, "file.al") {
		t.Errorf("unexpected join result: %q", joined)
	}

	// base, dir, ext, stem
	baseFn, _ := m.Get("base")
	base := baseFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: joined}).(*interpreter.StringValue).Val
	if base != "file.al" {
		t.Errorf("expected base 'file.al', got %q", base)
	}

	dirFn, _ := m.Get("dir")
	dir := dirFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: joined}).(*interpreter.StringValue).Val
	if !strings.Contains(dir, "folder") {
		t.Errorf("expected dir containing 'folder', got %q", dir)
	}

	extFn, _ := m.Get("ext")
	ext := extFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: joined}).(*interpreter.StringValue).Val
	if ext != ".al" {
		t.Errorf("expected ext '.al', got %q", ext)
	}

	stemFn, _ := m.Get("stem")
	stem := stemFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: joined}).(*interpreter.StringValue).Val
	if stem != "file" {
		t.Errorf("expected stem 'file', got %q", stem)
	}

	// split
	splitFn, _ := m.Get("split")
	splitList := splitFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: joined}).(*interpreter.ListValue)
	if len(splitList.Elements) != 2 {
		t.Errorf("expected 2 elements in split, got %d", len(splitList.Elements))
	}

	// matches
	matchFn, _ := m.Get("matches")
	isMatch := matchFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "*.al"},
		&interpreter.StringValue{Val: "app.al"},
	)
	if isMatch != interpreter.TRUE {
		t.Errorf("expected *.al to match app.al")
	}
}

func TestCryptoModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterCryptoBuiltins(env)

	val, ok := env.Get("crypto")
	if !ok {
		t.Fatalf("expected 'crypto' module to be registered")
	}
	m := val.(*interpreter.MapValue)

	// sha256
	sha256Fn, _ := m.Get("sha256")
	hashVal := sha256Fn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "hello alca"}).(*interpreter.StringValue).Val
	if len(hashVal) != 64 {
		t.Errorf("expected 64 hex chars for sha256, got %d (%q)", len(hashVal), hashVal)
	}

	// md5
	md5Fn, _ := m.Get("md5")
	md5Val := md5Fn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "hello alca"}).(*interpreter.StringValue).Val
	if len(md5Val) != 32 {
		t.Errorf("expected 32 hex chars for md5, got %d (%q)", len(md5Val), md5Val)
	}

	// base64 encode/decode
	b64EncFn, _ := m.Get("base64_encode")
	b64DecFn, _ := m.Get("base64_decode")
	enc := b64EncFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "secret token"}).(*interpreter.StringValue).Val
	dec := b64DecFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: enc}).(*interpreter.StringValue).Val
	if dec != "secret token" {
		t.Errorf("base64 roundtrip failed: got %q, expected 'secret token'", dec)
	}

	// uuid
	uuidFn, _ := m.Get("uuid")
	u := uuidFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.StringValue).Val
	if len(u) != 36 || strings.Count(u, "-") != 4 {
		t.Errorf("expected valid UUID format, got %q", u)
	}

	// random_string
	randStrFn, _ := m.Get("random_string")
	rnd := randStrFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 24}).(*interpreter.StringValue).Val
	if len(rnd) != 24 {
		t.Errorf("expected length 24, got %d (%q)", len(rnd), rnd)
	}

	// random_int
	randIntFn, _ := m.Get("random_int")
	ri := randIntFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 10}, &interpreter.IntegerValue{Val: 20}).(*interpreter.IntegerValue).Val
	if ri < 10 || ri > 20 {
		t.Errorf("expected random int in [10, 20], got %d", ri)
	}
}

func TestPathManagement(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterEnvBuiltins(env)
	stdlib.RegisterPathBuiltins(env)

	envVal, _ := env.Get("env")
	envModule := envVal.(*interpreter.MapValue)

	pathVal, _ := env.Get("path")
	pathModule := pathVal.(*interpreter.MapValue)

	// 1. get_path
	getPathFn, _ := envModule.Get("get_path")
	pathsList := getPathFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.ListValue)
	initialCount := len(pathsList.Elements)
	if initialCount == 0 {
		t.Errorf("expected non-empty PATH in test environment")
	}

	// 2. add_path
	testDir := "C:\\alca_dummy_test_dir_12345"
	if stdlib.OSName() != "windows" {
		testDir = "/tmp/alca_dummy_test_dir_12345"
	}

	addPathFn, _ := envModule.Get("add_path")
	addRes := addPathFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: testDir})
	if addRes != interpreter.TRUE {
		t.Errorf("expected add_path to return true, got %v", addRes)
	}

	// 3. has_path
	hasPathFn, _ := envModule.Get("has_path")
	hasRes := hasPathFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: testDir})
	if hasRes != interpreter.TRUE {
		t.Errorf("expected has_path to return true after adding")
	}

	// 4. prepend_path
	prependPathFn, _ := envModule.Get("prepend_path")
	prependRes := prependPathFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: testDir})
	if prependRes != interpreter.TRUE {
		t.Errorf("expected prepend_path to return true, got %v", prependRes)
	}

	updatedPaths := getPathFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.ListValue)
	if len(updatedPaths.Elements) == 0 || updatedPaths.Elements[0].(*interpreter.StringValue).Val != testDir {
		t.Errorf("expected prepended dir to be first in PATH, got %v", updatedPaths.Elements[0])
	}

	// 5. remove_path
	removePathFn, _ := envModule.Get("remove_path")
	removeRes := removePathFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: testDir})
	if removeRes != interpreter.TRUE {
		t.Errorf("expected remove_path to return true, got %v", removeRes)
	}

	hasAfterRemove := hasPathFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: testDir})
	if hasAfterRemove != interpreter.FALSE {
		t.Errorf("expected has_path to return false after removing")
	}

	// 6. which (in env and path)
	whichEnvFn, _ := envModule.Get("which")
	whichPathFn, _ := pathModule.Get("which")

	targetBin := "go"
	whichRes := whichEnvFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: targetBin})
	if whichRes == interpreter.NULL {
		t.Errorf("expected to find binary %q in PATH", targetBin)
	}
	whichPathRes := whichPathFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: targetBin})
	if whichPathRes == interpreter.NULL || whichPathRes.(*interpreter.StringValue).Val != whichRes.(*interpreter.StringValue).Val {
		t.Errorf("path.which should match env.which: %v vs %v", whichPathRes, whichRes)
	}

	// 7. list_sep, split_list, join_list
	listSepFn, _ := pathModule.Get("list_sep")
	sep := listSepFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.StringValue).Val
	if sep != ";" && sep != ":" {
		t.Errorf("expected list separator to be ';' or ':', got %q", sep)
	}

	splitListFn, _ := pathModule.Get("split_list")
	joinListFn, _ := pathModule.Get("join_list")

	sampleJoined := "foo" + sep + "bar" + sep + "baz"
	splitRes := splitListFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: sampleJoined}).(*interpreter.ListValue)
	if len(splitRes.Elements) != 3 {
		t.Errorf("expected 3 split items, got %d", len(splitRes.Elements))
	}

	joinedRes := joinListFn.(*interpreter.BuiltinFunction).Fn(splitRes).(*interpreter.StringValue).Val
	if joinedRes != sampleJoined {
		t.Errorf("expected joined %q, got %q", sampleJoined, joinedRes)
	}

	// 8. get_user_path
	getUserPathFn, _ := envModule.Get("get_user_path")
	userPaths := getUserPathFn.(*interpreter.BuiltinFunction).Fn()
	if userPaths.Type() != interpreter.LIST_VAL {
		t.Errorf("expected get_user_path to return LIST, got %s", userPaths.Type())
	}
}

func TestRegexModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterRegexBuiltins(env)

	val, _ := env.Get("re")
	m := val.(*interpreter.MapValue)

	// match
	matchFn, _ := m.Get("match")
	isMatch := matchFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: `^\d{3}-\d{2}$`},
		&interpreter.StringValue{Val: "123-45"},
	)
	if isMatch != interpreter.TRUE {
		t.Errorf("expected match to return true")
	}

	// find
	findFn, _ := m.Get("find")
	found := findFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: `\b[A-Z]+\b`},
		&interpreter.StringValue{Val: "hello ALCA world"},
	).(*interpreter.StringValue).Val
	if found != "ALCA" {
		t.Errorf("expected 'ALCA', got %q", found)
	}

	// find_groups
	findGroupsFn, _ := m.Get("find_groups")
	groups := findGroupsFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: `(\w+)@(\w+\.\w+)`},
		&interpreter.StringValue{Val: "contact admin@alca.dev for info"},
	).(*interpreter.ListValue)
	if len(groups.Elements) != 3 {
		t.Fatalf("expected 3 elements in groups, got %d", len(groups.Elements))
	}
	if groups.Elements[1].(*interpreter.StringValue).Val != "admin" {
		t.Errorf("expected group 1 to be 'admin', got %v", groups.Elements[1])
	}
	if groups.Elements[2].(*interpreter.StringValue).Val != "alca.dev" {
		t.Errorf("expected group 2 to be 'alca.dev', got %v", groups.Elements[2])
	}

	// replace
	replaceFn, _ := m.Get("replace")
	rep := replaceFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: `\s+`},
		&interpreter.StringValue{Val: "_"},
		&interpreter.StringValue{Val: "alca   lang   dev"},
	).(*interpreter.StringValue).Val
	if rep != "alca_lang_dev" {
		t.Errorf("expected 'alca_lang_dev', got %q", rep)
	}

	// split
	splitFn, _ := m.Get("split")
	sp := splitFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: `[,;]\s*`},
		&interpreter.StringValue{Val: "apple, banana; orange,grape"},
	).(*interpreter.ListValue)
	if len(sp.Elements) != 4 {
		t.Errorf("expected 4 split items, got %d", len(sp.Elements))
	}
}

func TestArchiveModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterArchiveBuiltins(env)

	val, _ := env.Get("archive")
	m := val.(*interpreter.MapValue)

	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	_ = os.MkdirAll(srcDir, 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello zip"), 0644)
	_ = os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("hello tar"), 0644)

	// ZIP Create & Extract
	zipFile := filepath.Join(tempDir, "test.zip")
	zipDest := filepath.Join(tempDir, "zip_out")

	zipCreateFn, _ := m.Get("zip_create")
	zipExtFn, _ := m.Get("zip_extract")
	listFn, _ := m.Get("list")

	res := zipCreateFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: srcDir},
		&interpreter.StringValue{Val: zipFile},
	)
	if res != interpreter.TRUE {
		t.Fatalf("zip_create failed: %v", res)
	}

	listRes := listFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: zipFile}).(*interpreter.ListValue)
	if len(listRes.Elements) < 2 {
		t.Errorf("expected at least 2 entries in zip list, got %d", len(listRes.Elements))
	}

	extRes := zipExtFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: zipFile},
		&interpreter.StringValue{Val: zipDest},
	)
	if extRes != interpreter.TRUE {
		t.Fatalf("zip_extract failed: %v", extRes)
	}

	readBack, err := os.ReadFile(filepath.Join(zipDest, "file1.txt"))
	if err != nil || string(readBack) != "hello zip" {
		t.Errorf("extracted content mismatch: %v (%s)", err, string(readBack))
	}

	// TAR.GZ Create & Extract
	tarFile := filepath.Join(tempDir, "test.tar.gz")
	tarDest := filepath.Join(tempDir, "tar_out")

	tarCreateFn, _ := m.Get("tar_create")
	tarExtFn, _ := m.Get("tar_extract")

	tRes := tarCreateFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: srcDir},
		&interpreter.StringValue{Val: tarFile},
	)
	if tRes != interpreter.TRUE {
		t.Fatalf("tar_create failed: %v", tRes)
	}

	tExtRes := tarExtFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: tarFile},
		&interpreter.StringValue{Val: tarDest},
	)
	if tExtRes != interpreter.TRUE {
		t.Fatalf("tar_extract failed: %v", tExtRes)
	}

	readTar, err := os.ReadFile(filepath.Join(tarDest, "file2.txt"))
	if err != nil || string(readTar) != "hello tar" {
		t.Errorf("tar extracted content mismatch: %v (%s)", err, string(readTar))
	}
}

func TestFormatsModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterFormatsBuiltins(env)

	// 1. CSV
	csvVal, _ := env.Get("csv")
	csvModule := csvVal.(*interpreter.MapValue)

	csvParseFn, _ := csvModule.Get("parse")
	csvStringifyFn, _ := csvModule.Get("stringify")

	csvRaw := "id,name,role\n1,Alice,Dev\n2,Bob,Ops\n"
	csvParsed := csvParseFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: csvRaw}).(*interpreter.ListValue)
	if len(csvParsed.Elements) != 2 {
		t.Fatalf("expected 2 parsed rows, got %d", len(csvParsed.Elements))
	}
	row0 := csvParsed.Elements[0].(*interpreter.MapValue)
	nameVal, _ := row0.Get("name")
	if nameVal.(*interpreter.StringValue).Val != "Alice" {
		t.Errorf("expected 'Alice', got %v", nameVal)
	}

	csvGen := csvStringifyFn.(*interpreter.BuiltinFunction).Fn(csvParsed).(*interpreter.StringValue).Val
	if !strings.Contains(csvGen, "Alice") || !strings.Contains(csvGen, "role") {
		t.Errorf("expected serialized csv with Alice and role, got %s", csvGen)
	}

	// 2. INI
	iniVal, _ := env.Get("ini")
	iniModule := iniVal.(*interpreter.MapValue)

	iniParseFn, _ := iniModule.Get("parse")
	iniRaw := "[server]\nhost = 0.0.0.0\nport = 8080\n[database]\nname = alca_db\n"
	iniParsed := iniParseFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: iniRaw}).(*interpreter.MapValue)

	srvSec, ok := iniParsed.Get("server")
	if !ok {
		t.Fatalf("expected 'server' section in parsed ini")
	}
	portVal, _ := srvSec.(*interpreter.MapValue).Get("port")
	if portVal.(*interpreter.StringValue).Val != "8080" {
		t.Errorf("expected port '8080', got %v", portVal)
	}

	// 3. JSON Query
	jsonQueryFn, _ := env.Get("json_query")
	doc := `{"cluster":{"nodes":[{"name":"node-1","ip":"10.0.0.1"},{"name":"node-2","ip":"10.0.0.2"}]}}`
	node2Ip := jsonQueryFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: doc},
		&interpreter.StringValue{Val: "cluster.nodes[1].ip"},
	)
	if node2Ip == interpreter.NULL || node2Ip.(*interpreter.StringValue).Val != "10.0.0.2" {
		t.Errorf("expected '10.0.0.2', got %v", node2Ip)
	}
}

func TestSysModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterSysBuiltins(env)

	val, _ := env.Get("sys")
	m := val.(*interpreter.MapValue)

	// disk_usage
	diskFn, _ := m.Get("disk_usage")
	diskInfo := diskFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.MapValue)
	totalGb, ok := diskInfo.Get("total_gb")
	if !ok || totalGb.(*interpreter.FloatValue).Val <= 0 {
		t.Errorf("expected positive total_gb for disk usage")
	}

	// ram_usage
	ramFn, _ := m.Get("ram_usage")
	ramInfo := ramFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.MapValue)
	totalMb, ok := ramInfo.Get("total_mb")
	if !ok || totalMb.(*interpreter.FloatValue).Val <= 0 {
		t.Errorf("expected positive total_mb for RAM usage")
	}

	// cpu_count
	cpuFn, _ := m.Get("cpu_count")
	cpus := cpuFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.IntegerValue).Val
	if cpus <= 0 {
		t.Errorf("expected positive cpu_count, got %d", cpus)
	}

	// network_interfaces
	netFn, _ := m.Get("network_interfaces")
	ifaces := netFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.ListValue)
	if len(ifaces.Elements) == 0 {
		t.Errorf("expected at least 1 network interface")
	}
}

func TestCLIModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterCLIBuiltins(env)

	val, _ := env.Get("cli")
	m := val.(*interpreter.MapValue)

	// color
	colorFn, _ := m.Get("color")
	col := colorFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "green"},
		&interpreter.StringValue{Val: "Success"},
	).(*interpreter.StringValue).Val
	if !strings.Contains(col, "\x1b[32m") || !strings.Contains(col, "Success") {
		t.Errorf("expected ANSI green formatted text, got %q", col)
	}

	// progress_bar
	progFn, _ := m.Get("progress_bar")
	bar := progFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 50},
		&interpreter.IntegerValue{Val: 100},
	).(*interpreter.StringValue).Val
	if !strings.Contains(bar, "50.0%") {
		t.Errorf("expected progress bar to contain 50.0%%, got %s", bar)
	}

	// parse
	parseFn, _ := m.Get("parse")
	stdlib.SetScriptArgs([]string{"--port=9000", "-v", "deploy"})
	spec := interpreter.NewMap()
	spec.Set("port", &interpreter.IntegerValue{Val: 8080})
	spec.Set("verbose", interpreter.FALSE)

	parsed := parseFn.(*interpreter.BuiltinFunction).Fn(spec).(*interpreter.MapValue)
	pVal, _ := parsed.Get("port")
	if pVal.(*interpreter.IntegerValue).Val != 9000 {
		t.Errorf("expected parsed port to be 9000, got %v", pVal)
	}
	vVal, _ := parsed.Get("verbose")
	if vVal != interpreter.TRUE {
		t.Errorf("expected parsed verbose to be true, got %v", vVal)
	}
}

func TestLogModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterLogBuiltins(env)

	val, _ := env.Get("log")
	m := val.(*interpreter.MapValue)

	setLvlFn, _ := m.Get("set_level")
	res := setLvlFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "debug"})
	if res != interpreter.TRUE {
		t.Errorf("expected set_level to return true")
	}

	setFmtFn, _ := m.Get("set_format")
	resFmt := setFmtFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "json"})
	if resFmt != interpreter.TRUE {
		t.Errorf("expected set_format to return true")
	}

	infoFn, _ := m.Get("info")
	fields := interpreter.NewMap()
	fields.Set("port", &interpreter.IntegerValue{Val: 8080})
	infoFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "Server started"}, fields)
}

func TestMathModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterMathBuiltins(env)

	val, _ := env.Get("math")
	m := val.(*interpreter.MapValue)

	// PI
	piVal, _ := m.Get("PI")
	if math.Abs(piVal.(*interpreter.FloatValue).Val-math.Pi) > 1e-9 {
		t.Errorf("PI mismatch: %v", piVal)
	}

	// sqrt
	sqrtFn, _ := m.Get("sqrt")
	sq := sqrtFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 144}).(*interpreter.FloatValue).Val
	if sq != 12.0 {
		t.Errorf("expected sqrt(144) == 12, got %f", sq)
	}

	// pow
	powFn, _ := m.Get("pow")
	pw := powFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 2},
		&interpreter.IntegerValue{Val: 8},
	).(*interpreter.FloatValue).Val
	if pw != 256.0 {
		t.Errorf("expected pow(2, 8) == 256, got %f", pw)
	}

	// round, floor, ceil
	roundFn, _ := m.Get("round")
	floorFn, _ := m.Get("floor")
	ceilFn, _ := m.Get("ceil")

	r := roundFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 3.7}).(*interpreter.IntegerValue).Val
	fl := floorFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 3.7}).(*interpreter.IntegerValue).Val
	cl := ceilFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 3.2}).(*interpreter.IntegerValue).Val
	if r != 4 || fl != 3 || cl != 4 {
		t.Errorf("round/floor/ceil mismatch: %d, %d, %d", r, fl, cl)
	}

	// clamp
	clampFn, _ := m.Get("clamp")
	clamped := clampFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 150},
		&interpreter.IntegerValue{Val: 0},
		&interpreter.IntegerValue{Val: 100},
	).(*interpreter.IntegerValue).Val
	if clamped != 100 {
		t.Errorf("expected clamped value 100, got %d", clamped)
	}
}

func TestDBModule(t *testing.T) {
	if !stdlib.DBDriversLinked {
		t.Skip("built with -tags nodb: SQL drivers are not linked")
	}
	env := interpreter.NewEnvironment()
	stdlib.RegisterDBBuiltins(env)

	val, ok := env.Get("db")
	if !ok {
		t.Fatalf("expected 'db' module to be registered")
	}
	dbMod := val.(*interpreter.MapValue)

	sqliteFn, ok := dbMod.Get("sqlite")
	if !ok {
		t.Fatalf("expected db.sqlite to be defined")
	}

	// Open in-memory SQLite database
	connVal := sqliteFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: ":memory:"})
	conn, ok := connVal.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("expected DB connection map, got %v", connVal)
	}

	// ping
	pingFn, _ := conn.Get("ping")
	isPing := pingFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.BoolValue).Val
	if !isPing {
		t.Fatalf("expected ping to succeed")
	}

	// exec DDL
	execFn, _ := conn.Get("exec")
	ddlRes := execFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "CREATE TABLE accounts (id INTEGER PRIMARY KEY, username TEXT, balance REAL);"},
	)
	if ddlRes.Type() == interpreter.ERROR_VAL {
		t.Fatalf("DDL error: %v", ddlRes.Inspect())
	}

	// exec INSERT
	insertRes := execFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "INSERT INTO accounts (username, balance) VALUES (?, ?);"},
		&interpreter.StringValue{Val: "alice"},
		&interpreter.FloatValue{Val: 250.75},
	)
	if insertRes.Type() == interpreter.ERROR_VAL {
		t.Fatalf("INSERT error: %v", insertRes.Inspect())
	}
	resMap := insertRes.(*interpreter.MapValue)
	ra, _ := resMap.Get("rows_affected")
	if ra.(*interpreter.IntegerValue).Val != 1 {
		t.Errorf("expected 1 row affected, got %v", ra)
	}

	// query
	queryFn, _ := conn.Get("query")
	qRes := queryFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "SELECT id, username, balance FROM accounts WHERE username = ?;"},
		&interpreter.StringValue{Val: "alice"},
	)
	rowsList, ok := qRes.(*interpreter.ListValue)
	if !ok || len(rowsList.Elements) != 1 {
		t.Fatalf("expected 1 row returned, got %v", qRes)
	}
	rowMap := rowsList.Elements[0].(*interpreter.MapValue)
	nameVal, _ := rowMap.Get("username")
	if nameVal.(*interpreter.StringValue).Val != "alice" {
		t.Errorf("expected username 'alice', got %v", nameVal)
	}
	balVal, _ := rowMap.Get("balance")
	if balVal.(*interpreter.FloatValue).Val != 250.75 {
		t.Errorf("expected balance 250.75, got %v", balVal)
	}

	// Transaction commit
	beginFn, _ := conn.Get("begin")
	txVal := beginFn.(*interpreter.BuiltinFunction).Fn()
	txMap := txVal.(*interpreter.MapValue)

	txExec, _ := txMap.Get("exec")
	txExec.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "INSERT INTO accounts (username, balance) VALUES (?, ?);"},
		&interpreter.StringValue{Val: "bob"},
		&interpreter.FloatValue{Val: 500.00},
	)
	txCommit, _ := txMap.Get("commit")
	txCommit.(*interpreter.BuiltinFunction).Fn()

	// query_row
	queryRowFn, _ := conn.Get("query_row")
	row := queryRowFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "SELECT username, balance FROM accounts WHERE username = ?;"},
		&interpreter.StringValue{Val: "bob"},
	).(*interpreter.MapValue)
	bobName, _ := row.Get("username")
	if bobName.(*interpreter.StringValue).Val != "bob" {
		t.Errorf("expected 'bob', got %v", bobName)
	}

	// close
	closeFn, _ := conn.Get("close")
	closeFn.(*interpreter.BuiltinFunction).Fn()
}

func TestMetricsModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterMetricsBuiltins(env)

	val, ok := env.Get("metrics")
	if !ok {
		t.Fatalf("expected 'metrics' module to be registered")
	}
	m := val.(*interpreter.MapValue)

	// counter
	counterFn, _ := m.Get("counter")
	cVal := counterFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "test_requests_total"},
		&interpreter.StringValue{Val: "Total requests"},
	).(*interpreter.MapValue)
	cInc, _ := cVal.Get("inc")
	cInc.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 5})
	cGet, _ := cVal.Get("get")
	cV := cGet.(*interpreter.BuiltinFunction).Fn().(*interpreter.FloatValue).Val
	if cV != 5.0 {
		t.Errorf("expected counter 5.0, got %f", cV)
	}

	// gauge
	gaugeFn, _ := m.Get("gauge")
	gVal := gaugeFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "test_active_sessions"},
		&interpreter.StringValue{Val: "Active sessions"},
	).(*interpreter.MapValue)
	gSet, _ := gVal.Get("set")
	gSet.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 20})
	gDec, _ := gVal.Get("dec")
	gDec.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 3})
	gGet, _ := gVal.Get("get")
	gV := gGet.(*interpreter.BuiltinFunction).Fn().(*interpreter.FloatValue).Val
	if gV != 17.0 {
		t.Errorf("expected gauge 17.0, got %f", gV)
	}

	// export
	exportFn, _ := m.Get("export")
	text := exportFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.StringValue).Val
	if !strings.Contains(text, "test_requests_total 5") || !strings.Contains(text, "test_active_sessions 17") {
		t.Errorf("expected exported metrics text, got:\n%s", text)
	}
}

func TestSecurityAndPermissionLogging(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterLogBuiltins(env)
	stdlib.RegisterSecurityBuiltins(env)

	// 1. Check IsPermissionError detection
	if !stdlib.IsPermissionError(os.ErrPermission) {
		t.Errorf("expected os.ErrPermission to be recognized as permission error")
	}
	if !stdlib.IsPermissionError(fmt.Errorf("open C:\\secret: Access is denied")) {
		t.Errorf("expected 'Access is denied' string to be recognized as permission error")
	}

	// 2. Direct log permission denied
	stdlib.LogPermissionDenied("read_protected", "/etc/shadow", fmt.Errorf("permission denied"))
	errs := stdlib.GetRecentAccessErrors()
	if len(errs) == 0 {
		t.Fatalf("expected recent access error event to be recorded")
	}
	last := errs[len(errs)-1]
	if last.Operation != "read_protected" || last.Resource != "/etc/shadow" {
		t.Errorf("unexpected logged event: %+v", last)
	}

	// 3. log.access_denied builtin
	logModVal, ok := env.Get("std.log")
	if !ok {
		t.Fatalf("expected std.log to be registered")
	}
	logMod := logModVal.(*interpreter.MapValue)
	accFn, ok := logMod.Get("access_denied")
	if !ok {
		t.Fatalf("expected log.access_denied to be registered")
	}
	accFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "delete_database"},
		&interpreter.StringValue{Val: "/var/lib/data"},
		&interpreter.StringValue{Val: "unauthorized role"},
	)

	// 4. log.get_access_errors builtin
	getErrFn, ok := logMod.Get("get_access_errors")
	if !ok {
		t.Fatalf("expected log.get_access_errors to be registered")
	}
	recordedList := getErrFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.ListValue)
	if len(recordedList.Elements) < 2 {
		t.Errorf("expected at least 2 recorded access errors, got %d", len(recordedList.Elements))
	}

	// 5. log.audit builtin
	auditFn, ok := logMod.Get("audit")
	if !ok {
		t.Fatalf("expected log.audit to be registered")
	}
	auditRes := auditFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "AUTH"},
		&interpreter.StringValue{Val: "LOGIN"},
		&interpreter.StringValue{Val: "admin"},
		&interpreter.StringValue{Val: "SUCCESS"},
	)
	if auditRes != interpreter.TRUE {
		t.Errorf("expected audit function to return true")
	}
}

func TestAdvancedIOAndFileHandles(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterIOBuiltins(env)

	ioModVal, _ := env.Get("std.io")
	ioMod := ioModVal.(*interpreter.MapValue)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "alca_test_io_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %s", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// 1. io.open with 'w+' mode
	openFn, ok := ioMod.Get("open")
	if !ok {
		t.Fatalf("expected io.open to be registered")
	}

	handleVal := openFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: tmpPath},
		&interpreter.StringValue{Val: "w+"},
	)
	if handleVal.Type() == interpreter.ERROR_VAL {
		t.Fatalf("failed to open handle: %s", handleVal.(*interpreter.ErrorValue).Message)
	}
	handle := handleVal.(*interpreter.MapValue)

	// 2. Write to handle
	writeFn, _ := handle.Get("write")
	writeFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "Hello Alca Stream!\nSecond Line"})

	// 3. Seek to beginning
	seekFn, _ := handle.Get("seek")
	posVal := seekFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 0},
		&interpreter.IntegerValue{Val: 0},
	).(*interpreter.IntegerValue)
	if posVal.Val != 0 {
		t.Errorf("expected seek pos 0, got %d", posVal.Val)
	}

	// 4. Tell current offset
	tellFn, _ := handle.Get("tell")
	tellVal := tellFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.IntegerValue)
	if tellVal.Val != 0 {
		t.Errorf("expected tell pos 0, got %d", tellVal.Val)
	}

	// 5. Read line
	readLineFn, _ := handle.Get("read_line")
	line := readLineFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.StringValue).Val
	if line != "Hello Alca Stream!" {
		t.Errorf("expected 'Hello Alca Stream!', got %q", line)
	}

	// 6. Lock and Unlock handle
	lockFn, _ := handle.Get("lock")
	unlockFn, _ := handle.Get("unlock")
	_ = lockFn.(*interpreter.BuiltinFunction).Fn()
	_ = unlockFn.(*interpreter.BuiltinFunction).Fn()

	// 7. Close handle
	closeFn, _ := handle.Get("close")
	closeRes := closeFn.(*interpreter.BuiltinFunction).Fn()
	if closeRes != interpreter.TRUE {
		t.Errorf("expected handle.close to return true")
	}

	// 8. check_access
	checkAccessFn, _ := ioMod.Get("check_access")
	canRead := checkAccessFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: tmpPath},
		&interpreter.StringValue{Val: "r"},
	)
	if canRead != interpreter.TRUE {
		t.Errorf("expected check_access to return true for readable temp file")
	}

	// 9. permissions
	permFn, _ := ioMod.Get("permissions")
	permVal := permFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: tmpPath}).(*interpreter.MapValue)
	if !permVal.Has("octal") || !permVal.Has("mode") {
		t.Errorf("expected permissions map to have octal and mode fields")
	}

	// 10. is_permission_error
	isPermErrFn, _ := ioMod.Get("is_permission_error")
	errVal := interpreter.NewError(403, "access denied")
	if isPermErrFn.(*interpreter.BuiltinFunction).Fn(errVal) != interpreter.TRUE {
		t.Errorf("expected is_permission_error to return true for 403 error")
	}
}

func TestSystemTracking(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterSysBuiltins(env)

	sysModVal, _ := env.Get("std.sys")
	sysMod := sysModVal.(*interpreter.MapValue)

	// 1. uptime
	uptimeFn, ok := sysMod.Get("uptime")
	if !ok {
		t.Fatalf("expected sys.uptime to be registered")
	}
	uptime := uptimeFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.FloatValue).Val
	if uptime < 0 {
		t.Errorf("expected non-negative uptime, got %f", uptime)
	}

	// 2. whoami
	whoamiFn, ok := sysMod.Get("whoami")
	if !ok {
		t.Fatalf("expected sys.whoami to be registered")
	}
	whoami := whoamiFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.MapValue)
	if !whoami.Has("user") || !whoami.Has("hostname") || !whoami.Has("is_admin") {
		t.Errorf("expected whoami map to have user, hostname, is_admin fields")
	}

	// 3. process_list
	procListFn, ok := sysMod.Get("process_list")
	if !ok {
		t.Fatalf("expected sys.process_list to be registered")
	}
	procs := procListFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.ListValue)
	if len(procs.Elements) == 0 {
		t.Errorf("expected non-empty process list")
	}

	// 4. disk_partitions
	partsFn, ok := sysMod.Get("disk_partitions")
	if !ok {
		t.Fatalf("expected sys.disk_partitions to be registered")
	}
	parts := partsFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.ListValue)
	if len(parts.Elements) == 0 {
		t.Errorf("expected non-empty disk partitions list")
	}

	// 5. load_avg
	loadFn, ok := sysMod.Get("load_avg")
	if !ok {
		t.Fatalf("expected sys.load_avg to be registered")
	}
	loads := loadFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.MapValue)
	if !loads.Has("1m") || !loads.Has("5m") || !loads.Has("15m") {
		t.Errorf("expected load_avg map to have 1m, 5m, 15m")
	}
}

func TestFFIModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterFFIBuiltins(env)

	ffiVal, ok := env.Get("std.ffi")
	if !ok {
		t.Fatalf("expected std.ffi to be registered")
	}
	ffiMod := ffiVal.(*interpreter.MapValue)

	// 1. size_of
	sizeOfFn, _ := ffiMod.Get("size_of")
	intSize := sizeOfFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "int32"}).(*interpreter.IntegerValue).Val
	ptrSize := sizeOfFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "ptr"}).(*interpreter.IntegerValue).Val
	if intSize != 4 || ptrSize != 8 {
		t.Errorf("size_of mismatch: int32=%d, ptr=%d", intSize, ptrSize)
	}

	// 2. alloc
	allocFn, _ := ffiMod.Get("alloc")
	bufVal := allocFn.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 32}).(*interpreter.BlobValue)
	if len(bufVal.Data) != 32 {
		t.Errorf("expected alloc 32 bytes, got %d", len(bufVal.Data))
	}

	// 3. String pointers
	cStrFn, _ := ffiMod.Get("c_str")
	ptrToStrFn, _ := ffiMod.Get("ptr_to_str")
	ptr := cStrFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "Alca Native FFI"})
	recovered := ptrToStrFn.(*interpreter.BuiltinFunction).Fn(ptr).(*interpreter.StringValue).Val
	if recovered != "Alca Native FFI" {
		t.Errorf("expected 'Alca Native FFI', got %q", recovered)
	}

	// 4. On Windows: test loading kernel32.dll and calling GetCurrentProcessId
	if runtime.GOOS == "windows" {
		loadFn, _ := ffiMod.Get("load")
		libVal := loadFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "kernel32.dll"})
		if libVal.Type() == interpreter.ERROR_VAL {
			t.Fatalf("failed to load kernel32.dll: %s", libVal.(*interpreter.ErrorValue).Message)
		}
		lib := libVal.(*interpreter.MapValue)

		callFn, _ := lib.Get("call")
		pidVal := callFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "GetCurrentProcessId"})
		if pidVal.Type() != interpreter.INTEGER_VAL {
			t.Fatalf("expected integer PID from GetCurrentProcessId, got %v", pidVal)
		}
		pid := pidVal.(*interpreter.IntegerValue).Val
		expectedPid := int64(os.Getpid())
		if pid != expectedPid {
			t.Errorf("GetCurrentProcessId mismatch: expected %d, got %d", expectedPid, pid)
		}

		// Also test callable func
		funcMaker, _ := lib.Get("func")
		getTickFn := funcMaker.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "GetTickCount64"})
		tickVal := getTickFn.(*interpreter.BuiltinFunction).Fn().(*interpreter.IntegerValue).Val
		if tickVal <= 0 {
			t.Errorf("expected positive tick count, got %d", tickVal)
		}

		closeFn, _ := lib.Get("close")
		closeFn.(*interpreter.BuiltinFunction).Fn()
	}
}

func TestAIModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterAIBuiltins(env)

	aiVal, ok := env.Get("std.ai")
	if !ok {
		t.Fatalf("expected std.ai to be registered")
	}
	aiMod := aiVal.(*interpreter.MapValue)

	// 1. matrix creation
	matrixFn, _ := aiMod.Get("matrix")
	mVal := matrixFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 2},
		&interpreter.IntegerValue{Val: 3},
		&interpreter.FloatValue{Val: 1.5},
	).(*interpreter.ListValue)
	if len(mVal.Elements) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(mVal.Elements))
	}
	r0 := mVal.Elements[0].(*interpreter.ListValue)
	if len(r0.Elements) != 3 || r0.Elements[0].(*interpreter.FloatValue).Val != 1.5 {
		t.Errorf("unexpected matrix contents: %+v", r0)
	}

	// 2. matmul: [ [1, 2], [3, 4] ] x [ [2, 0], [1, 2] ] = [ [4, 4], [10, 8] ]
	matmulFn, _ := aiMod.Get("matmul")
	matA := interpreter.NewList(
		interpreter.NewList(&interpreter.FloatValue{Val: 1}, &interpreter.FloatValue{Val: 2}),
		interpreter.NewList(&interpreter.FloatValue{Val: 3}, &interpreter.FloatValue{Val: 4}),
	)
	matB := interpreter.NewList(
		interpreter.NewList(&interpreter.FloatValue{Val: 2}, &interpreter.FloatValue{Val: 0}),
		interpreter.NewList(&interpreter.FloatValue{Val: 1}, &interpreter.FloatValue{Val: 2}),
	)
	resMat := matmulFn.(*interpreter.BuiltinFunction).Fn(matA, matB).(*interpreter.ListValue)
	row0 := resMat.Elements[0].(*interpreter.ListValue)
	row1 := resMat.Elements[1].(*interpreter.ListValue)
	if row0.Elements[0].(*interpreter.FloatValue).Val != 4 || row0.Elements[1].(*interpreter.FloatValue).Val != 4 ||
		row1.Elements[0].(*interpreter.FloatValue).Val != 10 || row1.Elements[1].(*interpreter.FloatValue).Val != 8 {
		t.Errorf("matmul result mismatch: row0=%+v, row1=%+v", row0, row1)
	}

	// 3. dot product: [1, 2, 3] . [4, 5, 6] = 4 + 10 + 18 = 32
	dotFn, _ := aiMod.Get("dot")
	v1 := interpreter.NewList(&interpreter.FloatValue{Val: 1}, &interpreter.FloatValue{Val: 2}, &interpreter.FloatValue{Val: 3})
	v2 := interpreter.NewList(&interpreter.FloatValue{Val: 4}, &interpreter.FloatValue{Val: 5}, &interpreter.FloatValue{Val: 6})
	dotRes := dotFn.(*interpreter.BuiltinFunction).Fn(v1, v2).(*interpreter.FloatValue).Val
	if dotRes != 32.0 {
		t.Errorf("expected dot product 32.0, got %f", dotRes)
	}

	// 4. relu and sigmoid
	reluFn, _ := aiMod.Get("relu")
	sigFn, _ := aiMod.Get("sigmoid")
	if reluFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: -5.0}).(*interpreter.FloatValue).Val != 0.0 {
		t.Errorf("expected relu(-5) == 0")
	}
	if sigFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 0.0}).(*interpreter.FloatValue).Val != 0.5 {
		t.Errorf("expected sigmoid(0) == 0.5")
	}

	// 5. softmax: sum of outputs == 1.0
	softmaxFn, _ := aiMod.Get("softmax")
	softRes := softmaxFn.(*interpreter.BuiltinFunction).Fn(
		interpreter.NewList(&interpreter.FloatValue{Val: 1.0}, &interpreter.FloatValue{Val: 2.0}, &interpreter.FloatValue{Val: 3.0}),
	).(*interpreter.ListValue)
	sum := 0.0
	for _, e := range softRes.Elements {
		sum += e.(*interpreter.FloatValue).Val
	}
	if math.Abs(sum-1.0) > 1e-6 {
		t.Errorf("expected softmax sum to be 1.0, got %f", sum)
	}

	// 6. dense_forward: input [ [1, 2] ] * weights [ [1, 0], [0, 1] ] + bias [ 5, 10 ] = [ [6, 12] ]
	denseFn, _ := aiMod.Get("dense_forward")
	inMat := interpreter.NewList(interpreter.NewList(&interpreter.FloatValue{Val: 1}, &interpreter.FloatValue{Val: 2}))
	wMat := interpreter.NewList(
		interpreter.NewList(&interpreter.FloatValue{Val: 1}, &interpreter.FloatValue{Val: 0}),
		interpreter.NewList(&interpreter.FloatValue{Val: 0}, &interpreter.FloatValue{Val: 1}),
	)
	bias := interpreter.NewList(&interpreter.FloatValue{Val: 5}, &interpreter.FloatValue{Val: 10})
	denseOut := denseFn.(*interpreter.BuiltinFunction).Fn(inMat, wMat, bias, &interpreter.StringValue{Val: "linear"}).(*interpreter.ListValue)
	outRow := denseOut.Elements[0].(*interpreter.ListValue)
	if outRow.Elements[0].(*interpreter.FloatValue).Val != 6.0 || outRow.Elements[1].(*interpreter.FloatValue).Val != 12.0 {
		t.Errorf("dense_forward mismatch: %+v", outRow)
	}
}
