package lsp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/lsp"
	"github.com/alca-lang/alca/pkg/typecheck"
)

func sendRPC(input *bytes.Buffer, method string, id interface{}, params interface{}) {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if id != nil {
		req["id"] = id
	}
	if params != nil {
		req["params"] = params
	}
	body, _ := json.Marshal(req)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	input.WriteString(header)
	input.Write(body)
}

func readRPC(output *bytes.Buffer) map[string]interface{} {
	str := output.String()
	headerEnd := strings.Index(str, "\r\n\r\n")
	if headerEnd == -1 {
		return nil
	}
	body := str[headerEnd+4:]
	var res map[string]interface{}
	_ = json.Unmarshal([]byte(body), &res)
	return res
}

// readAllRPC decodes every JSON-RPC message in the server output buffer.
func readAllRPC(raw string) []map[string]interface{} {
	var msgs []map[string]interface{}
	for {
		headerEnd := strings.Index(raw, "\r\n\r\n")
		if headerEnd == -1 {
			return msgs
		}
		length := 0
		for _, line := range strings.Split(raw[:headerEnd], "\r\n") {
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				n, err := strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
				if err == nil {
					length = n
				}
			}
		}
		body := raw[headerEnd+4:]
		if length <= 0 || length > len(body) {
			return msgs
		}
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(body[:length]), &msg); err != nil {
			return msgs
		}
		msgs = append(msgs, msg)
		raw = body[length:]
	}
}

// resultByID returns the result of the response with the given request id.
func resultByID(msgs []map[string]interface{}, id int) interface{} {
	for _, m := range msgs {
		if v, ok := m["id"].(float64); ok && int(v) == id {
			return m["result"]
		}
	}
	return nil
}

// completionLabels extracts the set of labels from a completion result.
func completionLabels(t *testing.T, result interface{}) map[string]map[string]interface{} {
	t.Helper()
	items, ok := result.([]interface{})
	if !ok {
		t.Fatalf("completion result is not an array: %T (%v)", result, result)
	}
	labels := make(map[string]map[string]interface{}, len(items))
	for _, it := range items {
		item, ok := it.(map[string]interface{})
		if !ok {
			t.Fatalf("completion item is not an object: %T", it)
		}
		label, _ := item["label"].(string)
		labels[label] = item
	}
	return labels
}

// hoverValue extracts the markdown text from a hover result.
func hoverValue(t *testing.T, result interface{}) string {
	t.Helper()
	res, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("hover result is not an object: %T (%v)", result, result)
	}
	contents, ok := res["contents"].(map[string]interface{})
	if !ok {
		t.Fatalf("hover result has no contents object: %v", res)
	}
	value, _ := contents["value"].(string)
	return value
}

// serveWith runs a full LSP session over the queued requests.
func serveWith(t *testing.T, in *bytes.Buffer) []map[string]interface{} {
	t.Helper()
	out := &bytes.Buffer{}
	srv := lsp.NewServer(in, out)
	if err := srv.Serve(); err != nil {
		t.Fatalf("LSP server error: %v", err)
	}
	return readAllRPC(out.String())
}

func openDoc(in *bytes.Buffer, uri, text string) {
	sendRPC(in, "textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":  uri,
			"text": text,
		},
	})
}

func hoverRequest(in *bytes.Buffer, id int, uri string, line, char int) {
	sendRPC(in, "textDocument/hover", id, map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": line, "character": char},
	})
}

func completionRequest(in *bytes.Buffer, id int, uri string, line, char int) {
	sendRPC(in, "textDocument/completion", id, map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": line, "character": char},
	})
}

func TestLSPInitializeAndCompletion(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	sendRPC(in, "initialize", 1, map[string]interface{}{})
	sendRPC(in, "textDocument/completion", 2, map[string]interface{}{})
	sendRPC(in, "shutdown", 3, nil)
	sendRPC(in, "exit", nil, nil)

	srv := lsp.NewServer(in, out)
	if err := srv.Serve(); err != nil {
		t.Fatalf("LSP server error: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "alca-lsp") {
		t.Fatalf("expected serverInfo to contain alca-lsp, got: %s", outStr)
	}
	if !strings.Contains(outStr, "std.db") || !strings.Contains(outStr, "std.metrics") {
		t.Fatalf("expected completions to contain std.db and std.metrics, got: %s", outStr)
	}
}

func TestLSPDiagnostics(t *testing.T) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	// Send invalid Alca code with unclosed paren
	invalidCode := "print(\"hello"
	sendRPC(in, "textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":  "file:///test.al",
			"text": invalidCode,
		},
	})
	sendRPC(in, "shutdown", 1, nil)
	sendRPC(in, "exit", nil, nil)

	srv := lsp.NewServer(in, out)
	if err := srv.Serve(); err != nil {
		t.Fatalf("LSP server error: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "textDocument/publishDiagnostics") {
		t.Fatalf("expected diagnostics notification, got: %s", outStr)
	}
}

func TestLSPHoverVariableShowsType(t *testing.T) {
	const uri = "file:///hover-var.al"
	in := &bytes.Buffer{}
	openDoc(in, uri, "x := 42;\nprint(x);\n")
	hoverRequest(in, 1, uri, 1, 6) // the `x` inside print(x)
	sendRPC(in, "shutdown", 2, nil)
	sendRPC(in, "exit", nil, nil)

	msgs := serveWith(t, in)
	value := hoverValue(t, resultByID(msgs, 1))
	if !strings.Contains(value, "x: int") {
		t.Fatalf("hover on variable x: expected %q to contain %q", value, "x: int")
	}
}

func TestLSPHoverStdModuleMemberShowsSignature(t *testing.T) {
	const uri = "file:///hover-mod.al"
	code := "use std.math;\n\nx := math.sqrt(9);\nprint(x);\n"
	in := &bytes.Buffer{}
	openDoc(in, uri, code)
	hoverRequest(in, 1, uri, 2, 11) // inside `sqrt`
	sendRPC(in, "shutdown", 2, nil)
	sendRPC(in, "exit", nil, nil)

	msgs := serveWith(t, in)
	value := hoverValue(t, resultByID(msgs, 1))
	if !strings.Contains(value, "math.sqrt") || !strings.Contains(value, "func(float) -> float") {
		t.Fatalf("hover on math.sqrt: expected signature, got: %q", value)
	}
}

func TestLSPHoverStructFieldShowsType(t *testing.T) {
	const uri = "file:///hover-struct.al"
	code := strings.Join([]string{
		"struct Server:",
		"    host: string;",
		"    port: int = 80;",
		"",
		"srv := Server(\"127.0.0.1\", 8080);",
		"print(srv.host);",
		"",
	}, "\n")
	in := &bytes.Buffer{}
	openDoc(in, uri, code)
	hoverRequest(in, 1, uri, 5, 11) // inside `host` in srv.host
	sendRPC(in, "shutdown", 2, nil)
	sendRPC(in, "exit", nil, nil)

	msgs := serveWith(t, in)
	value := hoverValue(t, resultByID(msgs, 1))
	if !strings.Contains(value, "Server.host") || !strings.Contains(value, "string") {
		t.Fatalf("hover on srv.host: expected %q to contain %q, got: %q", value, "Server.host: string", value)
	}
}

func TestLSPCompletionUsesRealTables(t *testing.T) {
	in := &bytes.Buffer{}
	openDoc(in, "file:///complete.al", "")
	completionRequest(in, 1, "file:///complete.al", 0, 0)
	sendRPC(in, "shutdown", 2, nil)
	sendRPC(in, "exit", nil, nil)

	msgs := serveWith(t, in)
	labels := completionLabels(t, resultByID(msgs, 1))

	// A primitive type keyword from pkg/token, a std module from the stdlib
	// table, and a global builtin; none of these were in the old hardcoded list.
	for _, want := range []string{"secret", "std.metrics", "print"} {
		if _, ok := labels[want]; !ok {
			t.Fatalf("completion is missing %q (got %d labels)", want, len(labels))
		}
	}
}

func TestLSPCompletionIncludesDocumentSymbols(t *testing.T) {
	const uri = "file:///symbols.al"
	code := strings.Join([]string{
		"struct Server:",
		"    host: string;",
		"    port: int = 80;",
		"",
		"func ping(host: string) -> bool:",
		"    return true;",
		"",
		"srv := Server(\"127.0.0.1\", 8080);",
		"",
	}, "\n")
	in := &bytes.Buffer{}
	openDoc(in, uri, code)
	completionRequest(in, 1, uri, 0, 0)
	sendRPC(in, "shutdown", 2, nil)
	sendRPC(in, "exit", nil, nil)

	msgs := serveWith(t, in)
	labels := completionLabels(t, resultByID(msgs, 1))

	for _, want := range []string{"Server", "host", "port", "ping", "srv"} {
		if _, ok := labels[want]; !ok {
			t.Fatalf("completion is missing document symbol %q", want)
		}
	}
	if item := labels["Server"]; item["kind"].(float64) != 22 {
		t.Fatalf("expected struct kind 22 for Server, got %v", item["kind"])
	}
	if item := labels["srv"]; item["kind"].(float64) != 6 {
		t.Fatalf("expected variable kind 6 for srv, got %v", item["kind"])
	}
}

func TestLSPCompletionAfterStdDotListsModules(t *testing.T) {
	const uri = "file:///std-dot.al"
	in := &bytes.Buffer{}
	openDoc(in, uri, "x := std.")
	completionRequest(in, 1, uri, 0, 9)
	sendRPC(in, "shutdown", 2, nil)
	sendRPC(in, "exit", nil, nil)

	msgs := serveWith(t, in)
	labels := completionLabels(t, resultByID(msgs, 1))

	want := typecheck.StdModuleNames()
	if len(labels) != len(want) {
		t.Fatalf("completion after `std.` has %d items, want %d (modules only)", len(labels), len(want))
	}
	for _, name := range want {
		if _, ok := labels[name]; !ok {
			t.Fatalf("completion after `std.` is missing module %q", name)
		}
	}
	if _, ok := labels["foreach"]; ok {
		t.Fatalf("completion after `std.` should not offer keywords")
	}
}

func TestLSPCompletionAfterModuleDotListsMembers(t *testing.T) {
	const uri = "file:///mod-dot.al"
	in := &bytes.Buffer{}
	openDoc(in, uri, "x := math.")
	completionRequest(in, 1, uri, 0, 10)
	sendRPC(in, "shutdown", 2, nil)
	sendRPC(in, "exit", nil, nil)

	msgs := serveWith(t, in)
	labels := completionLabels(t, resultByID(msgs, 1))

	members := typecheck.StdModuleTypes("math")
	if len(labels) != len(members) {
		t.Fatalf("completion after `math.` has %d items, want %d (members only)", len(labels), len(members))
	}
	for name := range members {
		if _, ok := labels[name]; !ok {
			t.Fatalf("completion after `math.` is missing member %q", name)
		}
	}
	if _, ok := labels["std"]; ok {
		t.Fatalf("completion after `math.` should not offer other namespaces")
	}
}
