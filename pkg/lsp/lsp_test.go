package lsp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/lsp"
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
