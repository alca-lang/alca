// Package lsp implements the Language Server Protocol (LSP) for Alca IDE tooling.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/typecheck"
)

// Server implements a standard JSON-RPC 2.0 Language Server.
type Server struct {
	reader  *bufio.Reader
	writer  io.Writer
	mu      sync.Mutex
	docs    map[string]string // URI -> document content
	running bool
}

// NewServer creates a new LSP server instance.
func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{
		reader: bufio.NewReader(in),
		writer: out,
		docs:   make(map[string]string),
	}
}

// Serve starts the LSP request processing loop until exit.
func (s *Server) Serve() error {
	s.running = true
	for s.running {
		// Read headers (Content-Length: <n>\r\n\r\n)
		contentLength := 0
		for {
			line, err := s.reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					cl, err := strconv.Atoi(strings.TrimSpace(parts[1]))
					if err == nil {
						contentLength = cl
					}
				}
			}
		}

		if contentLength == 0 {
			continue
		}

		body := make([]byte, contentLength)
		_, err := io.ReadFull(s.reader, body)
		if err != nil {
			return err
		}

		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			continue
		}

		s.handleMessage(req)
	}
	return nil
}

func (s *Server) handleMessage(req map[string]interface{}) {
	method, _ := req["method"].(string)
	id := req["id"]

	switch method {
	case "initialize":
		res := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"textDocumentSync": 1, // Full document sync
				"completionProvider": map[string]interface{}{
					"triggerCharacters": []string{".", ":"},
				},
				"hoverProvider": true,
			},
			"serverInfo": map[string]interface{}{
				"name":    "alca-lsp",
				"version": "1.0.0",
			},
		}
		s.sendResponse(id, res, nil)

	case "initialized":
		// Client confirmed initialization

	case "textDocument/didOpen":
		if params, ok := req["params"].(map[string]interface{}); ok {
			if doc, ok := params["textDocument"].(map[string]interface{}); ok {
				uri, _ := doc["uri"].(string)
				text, _ := doc["text"].(string)
				s.docs[uri] = text
				s.publishDiagnostics(uri, text)
			}
		}

	case "textDocument/didChange":
		if params, ok := req["params"].(map[string]interface{}); ok {
			if doc, ok := params["textDocument"].(map[string]interface{}); ok {
				uri, _ := doc["uri"].(string)
				if changes, ok := params["contentChanges"].([]interface{}); ok && len(changes) > 0 {
					if change, ok := changes[0].(map[string]interface{}); ok {
						text, _ := change["text"].(string)
						s.docs[uri] = text
						s.publishDiagnostics(uri, text)
					}
				}
			}
		}

	case "textDocument/didClose":
		if params, ok := req["params"].(map[string]interface{}); ok {
			if doc, ok := params["textDocument"].(map[string]interface{}); ok {
				uri, _ := doc["uri"].(string)
				delete(s.docs, uri)
			}
		}

	case "textDocument/completion":
		completions := s.getCompletions()
		s.sendResponse(id, completions, nil)

	case "textDocument/hover":
		var contents string = "### Alca Programming Language\nModern high-performance language."
		if params, ok := req["params"].(map[string]interface{}); ok {
			if pos, ok := params["position"].(map[string]interface{}); ok {
				_ = pos
				contents = "### Alca Symbol\nAlca standard symbol definition."
			}
		}
		s.sendResponse(id, map[string]interface{}{
			"contents": map[string]interface{}{
				"kind":  "markdown",
				"value": contents,
			},
		}, nil)

	case "shutdown":
		s.sendResponse(id, nil, nil)

	case "exit":
		s.running = false
	}
}

func (s *Server) publishDiagnostics(uri, text string) {
	l := lexer.New(text)
	p := parser.New(l)
	prog := p.ParseProgram()

	type RangePos struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	}
	type DiagnosticRange struct {
		Start RangePos `json:"start"`
		End   RangePos `json:"end"`
	}
	type DiagnosticItem struct {
		Range    DiagnosticRange `json:"range"`
		Severity int             `json:"severity"` // 1 = Error
		Source   string          `json:"source"`
		Message  string          `json:"message"`
	}

	diagnostics := make([]DiagnosticItem, 0)
	allErrors := p.SyntaxErrors()

	if len(allErrors) == 0 {
		checker := typecheck.NewChecker(uri, text)
		typeErrors := checker.Check(prog)
		allErrors = append(allErrors, typeErrors...)
	}

	for _, syntaxErr := range allErrors {
		line := syntaxErr.Line - 1
		if line < 0 {
			line = 0
		}
		col := syntaxErr.Column - 1
		if col < 0 {
			col = 0
		}

		length := syntaxErr.Length
		if length <= 0 {
			length = len(syntaxErr.Token)
			if length <= 0 {
				length = 1
			}
		}

		diagnostics = append(diagnostics, DiagnosticItem{
			Range: DiagnosticRange{
				Start: RangePos{Line: line, Character: col},
				End:   RangePos{Line: line, Character: col + length},
			},
			Severity: 1,
			Source:   "alca",
			Message:  syntaxErr.Message,
		})
	}

	s.sendNotification("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": diagnostics,
	})
}

func (s *Server) getCompletions() []map[string]interface{} {
	items := make([]map[string]interface{}, 0)

	keywords := []string{
		"func", "shfunc", "return", "if", "else", "elif", "while", "for", "loop", "foreach", "in",
		"break", "continue", "and", "or", "not", "true", "false", "null", "struct", "pub", "use",
		"as", "ba", "threads", "wait", "watch", "test", "mock", "assert", "tx", "match", "case",
	}
	for _, kw := range keywords {
		items = append(items, map[string]interface{}{
			"label":         kw,
			"kind":          14, // Keyword
			"detail":        "Alca keyword",
			"documentation": fmt.Sprintf("Keyword: `%s`", kw),
		})
	}

	types := []string{"string", "int", "float", "bool", "list", "map", "blob", "secret", "json"}
	for _, t := range types {
		items = append(items, map[string]interface{}{
			"label":         t,
			"kind":          7, // Class/Type
			"detail":        "Alca primitive type",
			"documentation": fmt.Sprintf("Type: `%s`", t),
		})
	}

	modules := []string{
		"std.db", "std.sql", "std.sqlite", "std.postgres", "std.metrics",
		"std.io", "std.regex", "std.re", "std.archive", "std.zip",
		"std.formats", "std.csv", "std.ini", "std.sys", "std.cli",
		"std.log", "std.math", "std.time", "std.path", "std.crypto", "std.net", "std.env",
		"std.gui", "std.ffi", "std.ai", "std.security",
	}
	for _, mod := range modules {
		items = append(items, map[string]interface{}{
			"label":         mod,
			"kind":          9, // Module
			"detail":        "Standard library module",
			"documentation": fmt.Sprintf("Standard module: `%s`", mod),
		})
	}

	builtins := []string{
		"print", "println", "len", "type", "int", "float", "string", "bool", "list",
		"append", "pop", "push", "keys", "values", "range", "sleep", "exit", "input",
		"json_encode", "json_decode", "abs", "max", "min", "time_now", "env",
	}
	for _, b := range builtins {
		items = append(items, map[string]interface{}{
			"label":         b,
			"kind":          3, // Function
			"detail":        "Builtin function",
			"documentation": fmt.Sprintf("Builtin: `%s(...)`", b),
		})
	}

	return items
}

func (s *Server) sendResponse(id interface{}, result interface{}, err interface{}) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
	}
	if err != nil {
		resp["error"] = err
	} else {
		resp["result"] = result
	}
	s.writeMessage(resp)
}

func (s *Server) sendNotification(method string, params interface{}) {
	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	s.writeMessage(notif)
}

func (s *Server) writeMessage(msg map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	_, _ = s.writer.Write([]byte(header))
	_, _ = s.writer.Write(data)
}

// RunStdio starts an LSP server on os.Stdin and os.Stdout.
func RunStdio() error {
	srv := NewServer(os.Stdin, os.Stdout)
	return srv.Serve()
}
