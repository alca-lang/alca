package stdlib

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

func TestUrllib2URLHelpers(t *testing.T) {
	parsed := urllib2Parse(&interpreter.StringValue{Val: "https://user:secret@example.test:8443/a/b;mode=fast?q=one#fragment"})
	result, ok := parsed.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("parse returned %T: %v", parsed, parsed)
	}
	for key, want := range map[string]string{
		"scheme": "https", "netloc": "user:secret@example.test:8443",
		"path": "/a/b", "params": "mode=fast", "query": "q=one",
		"fragment": "fragment", "username": "user", "password": "secret",
		"hostname": "example.test",
	} {
		value := mustGet(result, key).(*interpreter.StringValue).Val
		if value != want {
			t.Errorf("parse[%q] = %q, want %q", key, value, want)
		}
	}
	if port := mustGet(result, "port").(*interpreter.IntegerValue).Val; port != 8443 {
		t.Fatalf("parse port = %d, want 8443", port)
	}

	joined := urllib2Join(
		&interpreter.StringValue{Val: "https://example.test/a/b"},
		&interpreter.StringValue{Val: "../c?q=1"},
	).(*interpreter.StringValue).Val
	if joined != "https://example.test/c?q=1" {
		t.Fatalf("join = %q", joined)
	}
	quoted := urllib2Quote(&interpreter.StringValue{Val: "a b/c&d"}).(*interpreter.StringValue).Val
	if quoted != "a%20b/c%26d" {
		t.Fatalf("quote = %q", quoted)
	}
	unquoted := urllib2Unquote(&interpreter.StringValue{Val: "a+b%2Fc"}).(*interpreter.StringValue).Val
	if unquoted != "a+b/c" {
		t.Fatalf("unquote = %q", unquoted)
	}

	query := interpreter.NewMap()
	query.Set("tag", interpreter.NewList(
		&interpreter.StringValue{Val: "go lang"},
		&interpreter.StringValue{Val: "alca"},
	))
	query.Set("page", interpreter.NewInteger(2))
	encoded := urllib2EncodeQuery(query).(*interpreter.StringValue).Val
	if encoded != "page=2&tag=go+lang&tag=alca" {
		t.Fatalf("urlencode = %q", encoded)
	}
	parsedQuery := urllib2ParseQuery(&interpreter.StringValue{Val: "?tag=one&tag=two&page=2"}).(*interpreter.MapValue)
	tags := mustGet(parsedQuery, "tag").(*interpreter.ListValue).ElementsSlice()
	if len(tags) != 2 || tags[1].(*interpreter.StringValue).Val != "two" {
		t.Fatalf("parse_qs repeated values = %#v", tags)
	}
	pairs := urllib2ParseQueryList(&interpreter.StringValue{Val: "a=1&b=two"}).(*interpreter.ListValue)
	if pairs.Len() != 2 {
		t.Fatalf("parse_qsl returned %d pairs", pairs.Len())
	}
	firstPair := pairs.ElementsSlice()[0].(*interpreter.ListValue).ElementsSlice()
	if firstPair[0].(*interpreter.StringValue).Val != "a" || firstPair[1].(*interpreter.StringValue).Val != "1" {
		t.Fatalf("parse_qsl did not preserve input order: %#v", firstPair)
	}
}

func TestUrllib2HTTPRequestsAndURLRetrieve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get":
			w.Header().Add("X-Test", "one")
			w.Header().Add("X-Test", "two")
			_, _ = io.WriteString(w, r.URL.Query().Get("q"))
		case "/post":
			if got := r.Header.Get("X-Request"); got != "present" {
				http.Error(w, "missing request header", http.StatusBadRequest)
				return
			}
			data, _ := io.ReadAll(r.Body)
			_, _ = io.WriteString(w, r.Header.Get("Content-Type")+"|"+string(data))
		case "/redirect":
			http.Redirect(w, r, "/get?q=redirected", http.StatusFound)
		case "/file":
			_, _ = io.WriteString(w, "downloaded")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	previousPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.Policy{Net: sandbox.Capability{All: true}, Write: sandbox.Capability{All: true}})
	defer sandbox.SetPolicy(previousPolicy)

	base := server.URL
	getResponse := urllib2Get(&interpreter.StringValue{Val: base + "/get?q=hello"}).(*interpreter.MapValue)
	if status := mustGet(getResponse, "status").(*interpreter.IntegerValue).Val; status != http.StatusOK {
		t.Fatalf("GET status = %d", status)
	}
	if body := mustGet(getResponse, "body").(*interpreter.StringValue).Val; body != "hello" {
		t.Fatalf("GET body = %q", body)
	}
	headers := mustGet(getResponse, "headers").(*interpreter.MapValue)
	testHeaders := mustGet(headers, "X-Test").(*interpreter.ListValue).ElementsSlice()
	if len(testHeaders) != 2 {
		t.Fatalf("response header values = %d, want 2", len(testHeaders))
	}

	data := interpreter.NewMap()
	data.Set("name", &interpreter.StringValue{Val: "Ada Lovelace"})
	requestHeaders := interpreter.NewMap()
	requestHeaders.Set("X-Request", &interpreter.StringValue{Val: "present"})
	postResponse := urllib2Post(
		&interpreter.StringValue{Val: base + "/post"},
		data,
		requestHeaders,
		interpreter.NewInteger(3),
	).(*interpreter.MapValue)
	postBody := mustGet(postResponse, "body").(*interpreter.StringValue).Val
	if postBody != "application/x-www-form-urlencoded|name=Ada+Lovelace" {
		t.Fatalf("POST body = %q", postBody)
	}

	redirectResponse := urllib2Get(&interpreter.StringValue{Val: base + "/redirect"}).(*interpreter.MapValue)
	if body := mustGet(redirectResponse, "body").(*interpreter.StringValue).Val; body != "redirected" {
		t.Fatalf("redirect body = %q", body)
	}

	destination := filepath.Join(t.TempDir(), "nested", "download.txt")
	retrieved := urllib2URLRetrieve(
		&interpreter.StringValue{Val: base + "/file"},
		&interpreter.StringValue{Val: destination},
	).(*interpreter.MapValue)
	if filename := mustGet(retrieved, "filename").(*interpreter.StringValue).Val; filename != destination {
		t.Fatalf("retrieved filename = %q", filename)
	}
	content, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "downloaded" {
		t.Fatalf("downloaded content = %q", content)
	}
}

func TestUrllib2ModuleRegistration(t *testing.T) {
	env := interpreter.NewEnvironment()
	RegisterUrllib2Builtins(env)
	module, ok := env.Get("std.urllib2")
	if !ok {
		t.Fatal("std.urllib2 was not registered")
	}
	moduleMap, ok := module.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("std.urllib2 has type %T, want map", module)
	}
	if _, ok := moduleMap.Get("request"); !ok {
		t.Fatal("urllib2.request was not registered")
	}
	if flat, ok := env.Get("urllib2"); !ok || flat != module {
		t.Fatal("urllib2 shorthand was not registered to the same module")
	}
}
