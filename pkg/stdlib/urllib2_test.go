// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestUrllib2TcpRequestHonorsNetworkSandbox(t *testing.T) {
	previousPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.Policy{Net: sandbox.Capability{Roots: []string{"allowed.example"}}})
	defer sandbox.SetPolicy(previousPolicy)

	result := urllib2TcpRequest(
		&interpreter.StringValue{Val: "GET"},
		&interpreter.StringValue{Val: "http://blocked.example/test"},
	)
	if err, ok := result.(*interpreter.ErrorValue); !ok || err.Code != 403 {
		t.Fatalf("tcp_request bypassed the network policy: %v", result)
	}
	wsResult := websocket_handshake(&interpreter.StringValue{Val: "ws://blocked.example/test"})
	if err, ok := wsResult.(*interpreter.ErrorValue); !ok || err.Code != 403 {
		t.Fatalf("legacy websocket_handshake bypassed the network policy: %v", wsResult)
	}
}

func TestWebSocketTransportHandshakeAndMessageExchange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("HTTP test server does not support hijacking")
			return
		}
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("hijack connection: %v", err)
			return
		}
		defer conn.Close()
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		accept := base64.StdEncoding.EncodeToString(sum[:])
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + accept + "\r\n\r\n")
		if err := rw.Flush(); err != nil {
			t.Errorf("flush handshake response: %v", err)
			return
		}

		header := make([]byte, 2)
		if _, err := io.ReadFull(rw, header); err != nil {
			t.Errorf("read client frame header: %v", err)
			return
		}
		if header[0] != 0x81 || header[1]&0x80 == 0 {
			t.Errorf("client frame header = %x, want masked text frame", header)
			return
		}
		size := int(header[1] & 0x7f)
		if size == 126 {
			var extended [2]byte
			if _, err := io.ReadFull(rw, extended[:]); err != nil {
				t.Errorf("read extended frame length: %v", err)
				return
			}
			size = int(binary.BigEndian.Uint16(extended[:]))
		}
		var mask [4]byte
		if _, err := io.ReadFull(rw, mask[:]); err != nil {
			t.Errorf("read mask: %v", err)
			return
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(rw, payload); err != nil {
			t.Errorf("read client payload: %v", err)
			return
		}
		for i := range payload {
			payload[i] ^= mask[i&3]
		}
		if string(payload) != "hello" {
			t.Errorf("client payload = %q", payload)
			return
		}
		_, _ = conn.Write([]byte{0x81, 0x05, 'w', 'o', 'r', 'l', 'd'})
		_, _ = io.ReadFull(rw, header)
	}))
	defer server.Close()

	previousPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.AllowAll())
	defer sandbox.SetPolicy(previousPolicy)

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	result := websocket_connect(&interpreter.StringValue{Val: wsURL})
	connected, ok := result.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("websocket_connect returned %T: %v", result, result)
	}
	id := mustGet(connected, "id").(*interpreter.StringValue).Val
	if sent := websocket_send(&interpreter.StringValue{Val: id}, &interpreter.StringValue{Val: "hello"}); sent != interpreter.TRUE {
		t.Fatalf("websocket_send returned %v", sent)
	}
	received := websocket_receive(&interpreter.StringValue{Val: id})
	if value, ok := received.(*interpreter.StringValue); !ok || value.Val != "world" {
		t.Fatalf("websocket_receive returned %v", received)
	}
	if closed := websocket_close_impl(&interpreter.StringValue{Val: id}); closed != interpreter.TRUE {
		t.Fatalf("websocket_close returned %v", closed)
	}
}
