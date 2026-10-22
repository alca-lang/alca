package stdlib

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

func TestAuditedIOErrorsAndLargeLines(t *testing.T) {
	root := t.TempDir()
	oldPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.AllowAll())
	defer sandbox.SetPolicy(oldPolicy)

	if exists, err := FileExistsChecked(filepath.Join(root, "missing")); err != nil || exists {
		t.Fatalf("missing path = %v, %v; want false, nil", exists, err)
	}
	longLine := strings.Repeat("x", 128*1024)
	file := filepath.Join(root, "long.txt")
	if err := os.WriteFile(file, []byte(longLine+"\r\nlast\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lines, err := ReadLines(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("ReadLines returned %d lines", len(lines))
	}
	if lines[0] != longLine || lines[1] != "last" {
		t.Fatalf("ReadLines returned unexpected content")
	}
	if _, err := FileExistsChecked(root); err != nil {
		t.Fatalf("directory should be stat-able: %v", err)
	}
}

func TestAuditedBytesWalkJSONAndStringModules(t *testing.T) {
	oldPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.AllowAll())
	defer sandbox.SetPolicy(oldPolicy)

	env := interpreter.NewEnvironment()
	RegisterIOBuiltins(env)
	RegisterFormatsBuiltins(env)
	RegisterStringBuiltins(env)

	path := filepath.Join(t.TempDir(), "data.bin")
	writeBytes, _ := env.Get("write_bytes")
	readBytes, _ := env.Get("read_bytes")
	payload := []byte{0, 1, 2, 255}
	if result := writeBytes.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: path},
		&interpreter.BlobValue{Data: payload},
	); result != interpreter.TRUE {
		t.Fatalf("write_bytes failed: %v", result)
	}
	read := readBytes.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: path})
	blob, ok := read.(*interpreter.BlobValue)
	if !ok || string(blob.Data) != string(payload) {
		t.Fatalf("read_bytes returned %T %v", read, read)
	}

	jsonValue, _ := env.Get("json")
	jsonModule := jsonValue.(*interpreter.MapValue)
	parse, _ := jsonModule.Get("parse")
	encode, _ := jsonModule.Get("encode")
	decoded := parse.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: `{"a":[1,true]}`})
	encoded := encode.(*interpreter.BuiltinFunction).Fn(decoded)
	if got := encoded.(*interpreter.StringValue).Val; got != `{"a":[1,true]}` {
		t.Fatalf("JSON round trip = %s", got)
	}
	if got := parse.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "{"}); got.Type() != interpreter.ERROR_VAL {
		t.Fatalf("invalid JSON parse returned %v", got)
	}

	strValue, _ := env.Get("std.string")
	strModule := strValue.(*interpreter.MapValue)
	slice, _ := strModule.Get("slice")
	result := slice.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "a🙂bc"},
		&interpreter.IntegerValue{Val: 1},
		&interpreter.IntegerValue{Val: 2},
	)
	if got := result.(*interpreter.StringValue).Val; got != "🙂" {
		t.Fatalf("Unicode string slice = %q", got)
	}
}

func TestAuditedNetworkAndCompressionLimits(t *testing.T) {
	oldPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.AllowAll())
	defer sandbox.SetPolicy(oldPolicy)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, maxHTTPBodyBytes+1))
	}))
	defer server.Close()
	if _, _, err := HTTPGet(server.URL); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("HTTPGet oversized response error = %v", err)
	}

	encoded, err := GzipCompress([]byte(strings.Repeat("x", 1024)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := GzipDecompressLimit(encoded, 10); err == nil {
		t.Fatal("gzip decompression exceeded its output limit without error")
	}
}

func TestAuditedMathTimeAndCryptoErrors(t *testing.T) {
	env := interpreter.NewEnvironment()
	RegisterMathBuiltins(env)
	RegisterTimeBuiltins(env)
	RegisterCryptoBuiltins(env)

	mathModule, _ := env.Get("math")
	sqrt, _ := mathModule.(*interpreter.MapValue).Get("sqrt")
	if result := sqrt.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "not a number"}); result.Type() != interpreter.ERROR_VAL {
		t.Fatalf("math.sqrt invalid argument returned %v", result)
	}

	timeModule, _ := env.Get("time")
	duration, _ := timeModule.(*interpreter.MapValue).Get("parse_duration")
	if got := duration.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "150ms"}); got.(*interpreter.IntegerValue).Val != 150_000_000 {
		t.Fatalf("parse_duration result = %v", got)
	}
	format, _ := timeModule.(*interpreter.MapValue).Get("format")
	formatted := format.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 0},
		&interpreter.StringValue{Val: "datetime"},
		&interpreter.StringValue{Val: "America/New_York"},
	)
	if got := formatted.(*interpreter.StringValue).Val; got != "1969-12-31 19:00:00" {
		t.Fatalf("timezone format = %q", got)
	}
	measure, _ := timeModule.(*interpreter.MapValue).Get("measure")
	if result := measure.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "not callable"}); result.Type() != interpreter.ERROR_VAL {
		t.Fatalf("measure invalid callback returned %v", result)
	}

	cryptoModule, _ := env.Get("crypto")
	randomBytes, _ := cryptoModule.(*interpreter.MapValue).Get("random_bytes")
	if got := randomBytes.(*interpreter.BuiltinFunction).Fn(&interpreter.IntegerValue{Val: 24}); len(got.(*interpreter.BlobValue).Data) != 24 {
		t.Fatalf("random_bytes returned %d bytes", len(got.(*interpreter.BlobValue).Data))
	}
}

func TestPreparedDBStatementsAndQueryRow(t *testing.T) {
	oldPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.AllowAll())
	defer sandbox.SetPolicy(oldPolicy)
	defer CloseAllDatabases()

	env := interpreter.NewEnvironment()
	RegisterDBBuiltins(env)
	moduleValue, _ := env.Get("db")
	module := moduleValue.(*interpreter.MapValue)
	sqliteValue, _ := module.Get("sqlite")
	connection := sqliteValue.(*interpreter.BuiltinFunction).Fn()
	conn, ok := connection.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("db.sqlite returned %T: %v", connection, connection)
	}
	execValue, _ := conn.Get("exec")
	create := execValue.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "CREATE TABLE audit_test (value TEXT)"})
	if create.Type() == interpreter.ERROR_VAL {
		t.Fatalf("create table failed: %v", create)
	}
	prepareValue, _ := conn.Get("prepare")
	preparedValue := prepareValue.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "INSERT INTO audit_test(value) VALUES (?)"})
	prepared, ok := preparedValue.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("prepare returned %T: %v", preparedValue, preparedValue)
	}
	preparedExecValue, _ := prepared.Get("exec")
	if result := preparedExecValue.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "row"}); result.Type() == interpreter.ERROR_VAL {
		t.Fatalf("prepared insert failed: %v", result)
	}
	queryRowValue, _ := conn.Get("query_row")
	row := queryRowValue.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "SELECT value FROM audit_test"}).(*interpreter.MapValue)
	value, found := row.Get("value")
	if !found || value.(*interpreter.StringValue).Val != "row" {
		t.Fatalf("query_row returned %v", row)
	}
}
