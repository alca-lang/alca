// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/sandbox"
)

func TestHTTPRouterRoutesValidationDependenciesAndMiddleware(t *testing.T) {
	router := newHTTPRouter()
	handler := &interpreter.BuiltinFunction{
		Name: "handler",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			request := args[0].(*interpreter.MapValue)
			params, _ := request.Get("params")
			query, _ := request.Get("query")
			body, _ := request.Get("json")
			dependencies, _ := request.Get("dependencies")
			result := interpreter.NewMap()
			result.Set("params", params)
			result.Set("query", query)
			result.Set("body", body)
			result.Set("user", mustGet(dependencies.(*interpreter.MapValue), "user"))
			return httpJSONResponseBuiltin(result)
		},
	}
	userDependency := &interpreter.BuiltinFunction{
		Name: "load_user",
		Fn: func(...interpreter.Value) interpreter.Value {
			return &interpreter.StringValue{Val: "test-user"}
		},
	}
	dependencies := interpreter.NewMap()
	dependencies.Set("user", userDependency)

	pathFields := interpreter.NewMap()
	pathFields.Set("id", &interpreter.StringValue{Val: "int"})
	queryFields := interpreter.NewMap()
	queryFields.Set("active", &interpreter.StringValue{Val: "bool"})
	bodyFields := interpreter.NewMap()
	bodyFields.Set("name", &interpreter.StringValue{Val: "string"})
	schema := interpreter.NewMap()
	schema.Set("path", pathFields)
	schema.Set("query", queryFields)
	schema.Set("body", bodyFields)
	options := interpreter.NewMap()
	options.Set("schema", schema)
	options.Set("dependencies", dependencies)
	options.Set("summary", &interpreter.StringValue{Val: "Read a user"})

	get := mustGet(router.value, "post").(*interpreter.BuiltinFunction)
	if result := get.Fn(
		&interpreter.StringValue{Val: "/users/{id}"},
		handler,
		options,
	); result != interpreter.TRUE {
		t.Fatalf("route registration failed: %v", result)
	}

	use := mustGet(router.value, "use").(*interpreter.BuiltinFunction)
	middleware := &interpreter.BuiltinFunction{
		Name: "middleware",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			next := args[1].(*interpreter.BuiltinFunction)
			response := next.Fn(args[0]).(*interpreter.MapValue)
			headers, _ := response.Get("headers")
			headers.(*interpreter.MapValue).Set("X-Middleware", &interpreter.StringValue{Val: "ran"})
			return response
		},
	}
	if result := use.Fn(middleware); result != interpreter.TRUE {
		t.Fatalf("middleware registration failed: %v", result)
	}

	request := httptest.NewRequest(http.MethodPost, "/users/42?active=true", strings.NewReader(`{"name":"Ada"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Middleware"); got != "ran" {
		t.Fatalf("middleware header = %q, want ran", got)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	params := decoded["params"].(map[string]interface{})
	if params["id"] != float64(42) {
		t.Fatalf("validated path parameter = %#v, want integer 42", params["id"])
	}
	query := decoded["query"].(map[string]interface{})
	if query["active"] != true {
		t.Fatalf("validated query parameter = %#v, want true", query["active"])
	}
	if decoded["user"] != "test-user" {
		t.Fatalf("injected dependency = %#v, want test-user", decoded["user"])
	}
	body := decoded["body"].(map[string]interface{})
	if body["name"] != "Ada" {
		t.Fatalf("validated body field = %#v, want Ada", body["name"])
	}
}

func TestHTTPRouterValidationErrorsAndMethods(t *testing.T) {
	router := newHTTPRouter()
	handler := &interpreter.BuiltinFunction{
		Name: "ok",
		Fn: func(...interpreter.Value) interpreter.Value {
			return &interpreter.StringValue{Val: "ok"}
		},
	}
	querySchema := interpreter.NewMap()
	querySchema.Set("count", &interpreter.StringValue{Val: "int"})
	schema := interpreter.NewMap()
	schema.Set("query", querySchema)
	options := interpreter.NewMap()
	options.Set("schema", schema)
	mustGet(router.value, "get").(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "/items"},
		handler,
		options,
	)

	tests := []struct {
		name   string
		method string
		target string
		status int
	}{
		{name: "invalid query", method: http.MethodGet, target: "/items?count=nope", status: http.StatusUnprocessableEntity},
		{name: "missing query", method: http.MethodGet, target: "/items", status: http.StatusUnprocessableEntity},
		{name: "method not allowed", method: http.MethodPost, target: "/items", status: http.StatusMethodNotAllowed},
		{name: "not found", method: http.MethodGet, target: "/missing", status: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, test.target, nil))
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
		})
	}
}

func TestHTTPRouterHeadFallsBackToGet(t *testing.T) {
	router := newHTTPRouter()
	handler := &interpreter.BuiltinFunction{
		Name: "get",
		Fn: func(...interpreter.Value) interpreter.Value {
			return &interpreter.StringValue{Val: "body"}
		},
	}
	mustGet(router.value, "get").(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "/health"},
		handler,
	)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("HEAD status = %d, want 200", response.Code)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("HEAD response unexpectedly wrote a body: %q", response.Body.String())
	}
}

func TestHTTPRouterOpenAPIGeneration(t *testing.T) {
	router := newHTTPRouter()
	pathSchema := interpreter.NewMap()
	pathSchema.Set("id", &interpreter.StringValue{Val: "int"})
	schema := interpreter.NewMap()
	schema.Set("path", pathSchema)
	options := interpreter.NewMap()
	options.Set("schema", schema)
	options.Set("summary", &interpreter.StringValue{Val: "Read a user"})
	handler := &interpreter.BuiltinFunction{Name: "read_user", Fn: func(...interpreter.Value) interpreter.Value {
		return interpreter.NULL
	}}
	mustGet(router.value, "get").(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: "/users/{id}"},
		handler,
		options,
	)
	doc := router.openapiDocument().(*interpreter.MapValue)
	paths := mustGet(doc, "paths").(*interpreter.MapValue)
	pathItem := mustGet(paths, "/users/{id}").(*interpreter.MapValue)
	get := mustGet(pathItem, "get").(*interpreter.MapValue)
	if mustGet(get, "summary").(*interpreter.StringValue).Val != "Read a user" {
		t.Fatal("OpenAPI summary was not included")
	}
	parameters := mustGet(get, "parameters").(*interpreter.ListValue).ElementsSlice()
	if len(parameters) != 1 {
		t.Fatalf("OpenAPI parameter count = %d, want 1", len(parameters))
	}
	parameter := parameters[0].(*interpreter.MapValue)
	if mustGet(parameter, "name").(*interpreter.StringValue).Val != "id" {
		t.Fatal("OpenAPI path parameter name is incorrect")
	}
}

func TestHTTPResponseHelpers(t *testing.T) {
	jsonResponse := httpJSONResponseBuiltin(
		&interpreter.StringValue{Val: "ok"},
		&interpreter.IntegerValue{Val: http.StatusCreated},
	)
	response := jsonResponse.(*interpreter.MapValue)
	if mustGet(response, "status").(*interpreter.IntegerValue).Val != http.StatusCreated {
		t.Fatal("JSON response did not retain the requested status")
	}
	headers := mustGet(response, "headers").(*interpreter.MapValue)
	if mustGet(headers, "Content-Type").(*interpreter.StringValue).Val != "application/json; charset=utf-8" {
		t.Fatal("JSON response did not set its content type")
	}
	if _, ok := httpResponseBuiltin(&interpreter.StringValue{Val: "200"}).(*interpreter.ErrorValue); !ok {
		t.Fatal("response helper should reject a non-integer status")
	}
}

func TestHTTPRouterAlcaHandler(t *testing.T) {
	source := `
router := http.router();
handler := |request: map[string, json]| http.json({"data": request["json"], "dependencies": request["dependencies"]});
fields := {"name": "string"};
schema: map[string, json] = {"body": fields};
dependency := |request: map[string, json]| "alca-user";
dependencies: map[string, json] = {"user": dependency};
options: map[string, json] = {"schema": schema, "dependencies": dependencies};
middleware := |request: map[string, json], next: json| next(request);
router.use(middleware);
registered := router.post("/echo", handler, options);
`
	parsed := parser.New(lexer.New(source))
	program := parsed.ParseProgram()
	if errors := parsed.SyntaxErrors(); len(errors) > 0 {
		t.Fatalf("unexpected syntax error: %s", errors[0].Message)
	}
	runtime := interpreter.New()
	RegisterHTTPBuiltins(runtime.Env())
	if err := runtime.Execute(program); err != nil {
		t.Fatalf("executing HTTP router Alca setup failed: %v", err)
	}
	routerValue, ok := runtime.Env().Get("router")
	if !ok {
		t.Fatal("Alca router variable was not created")
	}
	routerMap := routerValue.(*interpreter.MapValue)
	registrationValue, _ := runtime.Env().Get("registered")
	if errValue, failed := registrationValue.(*interpreter.ErrorValue); failed {
		t.Fatalf("Alca route registration returned error: %s", errValue.Message)
	}

	previousPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.Policy{Net: sandbox.Capability{All: true}})
	defer sandbox.SetPolicy(previousPolicy)
	serverValue := mustGet(routerMap, "serve").(*interpreter.BuiltinFunction).Fn(interpreter.NewInteger(0))
	server, ok := serverValue.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("router.serve failed: %v", serverValue)
	}
	defer mustGet(server, "close").(*interpreter.BuiltinFunction).Fn()
	port := mustGet(server, "port").(*interpreter.IntegerValue).Val
	client := &http.Client{Timeout: 3 * time.Second}
	var response *http.Response
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		request, requestErr := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/echo", port), strings.NewReader(`{"name":"Ada"}`))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err = client.Do(request)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("request to Alca HTTP router failed: %v", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.StatusCode, responseBody)
	}
	var responseJSON map[string]interface{}
	if err := json.Unmarshal(responseBody, &responseJSON); err != nil {
		t.Fatalf("response body is invalid JSON: %v", err)
	}
	if data := responseJSON["data"].(map[string]interface{}); data["name"] != "Ada" {
		t.Fatalf("JSON body delivered to handler = %#v, want Ada", data)
	}
	if dependencies := responseJSON["dependencies"].(map[string]interface{}); dependencies["user"] != "alca-user" {
		t.Fatalf("injected dependencies = %#v, want alca-user", dependencies)
	}
}

type mockHostCallable struct {
	called bool
}

func (m *mockHostCallable) Type() interpreter.ValueType { return interpreter.FUNC_VAL }
func (m *mockHostCallable) Inspect() string             { return "<host_func>" }
func (m *mockHostCallable) Clone() interpreter.Value    { return m }
func (m *mockHostCallable) Call(args []interpreter.Value) (interpreter.Value, error) {
	m.called = true
	req := args[0].(*interpreter.MapValue)
	params, _ := req.Get("params")
	pMap := params.(*interpreter.MapValue)
	tokenVal, _ := pMap.Get("token")
	res := interpreter.NewMap()
	res.Set("token", tokenVal)
	return httpJSONResponseBuiltin(res), nil
}

func TestHTTPRouterBotTokenSegmentAndHostCallable(t *testing.T) {
	router := newHTTPRouter()
	handler := &mockHostCallable{}
	res := router.addRoute("POST", &interpreter.StringValue{Val: "/bot{token}/webhook"}, handler)
	if errVal, ok := res.(*interpreter.ErrorValue); ok {
		t.Fatalf("failed to add /bot{token}/webhook route: %s", errVal.Message)
	}

	req := httptest.NewRequest(http.MethodPost, "/bot123456:ABC/webhook", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if !handler.called {
		t.Fatalf("host callable handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "123456:ABC") {
		t.Fatalf("expected body to contain token, got: %s", w.Body.String())
	}
}
