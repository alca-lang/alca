// Package stdlib provides the HTTP client and server standard library module for Alca.
package stdlib

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

const maxHTTPBodyBytes = 16 << 20

// HTTPServerInstance manages a running HTTP server with dynamic Alca route handling.
type HTTPServerInstance struct {
	server   *http.Server
	listener net.Listener
	mu       sync.Mutex
	isClosed bool
}

// Close stops the HTTP server.
func (s *HTTPServerInstance) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isClosed {
		return nil
	}
	s.isClosed = true
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// RegisterHTTPBuiltins registers "std.http" and "http" modules into the environment.
func RegisterHTTPBuiltins(env *interpreter.Environment) {
	httpMod := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		httpMod.Set(name, bf)
		env.Set(name, bf)
	}

	// request(method, url, headers?, body?, options?) -> map
	httpRequest := func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "http_request requires (method, url) arguments")
		}
		method := strings.ToUpper(interpreter.CoerceToString(args[0]))
		url := interpreter.CoerceToString(args[1])
		if err := sandbox.RequireURL(url); err != nil {
			return handleIOError("http_request", url, err)
		}

		var bodyReader io.Reader
		if len(args) >= 4 && args[3] != nil {
			bodyReader = strings.NewReader(interpreter.CoerceToString(args[3]))
		}

		req, err := http.NewRequest(method, url, bodyReader)
		if err != nil {
			return interpreter.NewError(400, "invalid request: "+err.Error())
		}

		if len(args) >= 3 && args[2] != nil {
			if hm, ok := args[2].(*interpreter.MapValue); ok {
				for k, v := range hm.Pairs {
					setHTTPHeaderValues(req.Header, k, v)
				}
			}
		}

		timeout, err := httpRequestTimeout(args)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		client := policyHTTPClient(timeout)
		resp, err := client.Do(req)
		if err != nil {
			return handleIOError("http_request", url, err)
		}
		defer resp.Body.Close()

		respBytes, err := readLimitedHTTPBody(resp.Body)
		if err != nil {
			if errors.Is(err, errHTTPBodyTooLarge) {
				return interpreter.NewError(413, err.Error())
			}
			return interpreter.NewError(500, "failed to read response body: "+err.Error())
		}

		res := interpreter.NewMap()
		res.Set("status", &interpreter.IntegerValue{Val: int64(resp.StatusCode)})
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			res.Set("ok", interpreter.TRUE)
		} else {
			res.Set("ok", interpreter.FALSE)
		}
		res.Set("body", &interpreter.StringValue{Val: string(respBytes)})

		respHeaders := interpreter.NewMap()
		for k, values := range resp.Header {
			headerValues := make([]interpreter.Value, len(values))
			for i, value := range values {
				headerValues[i] = &interpreter.StringValue{Val: value}
			}
			respHeaders.Set(k, interpreter.NewList(headerValues...))
		}
		res.Set("headers", respHeaders)
		return res
	}
	register("http_request", httpRequest)
	httpMod.Set("request", &interpreter.BuiltinFunction{Name: "request", Fn: httpRequest})

	// Convenience wrappers
	makeMethodCaller := func(method string) func(args ...interpreter.Value) interpreter.Value {
		return func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, method+" requires a url argument")
			}
			newArgs := []interpreter.Value{
				&interpreter.StringValue{Val: method},
				args[0],
			}
			if len(args) > 1 {
				newArgs = append(newArgs, args[1:]...)
			}
			return httpRequest(newArgs...)
		}
	}

	httpMod.Set("get", &interpreter.BuiltinFunction{Name: "get", Fn: makeMethodCaller("GET")})
	httpMod.Set("post", &interpreter.BuiltinFunction{Name: "post", Fn: makeMethodCaller("POST")})
	httpMod.Set("put", &interpreter.BuiltinFunction{Name: "put", Fn: makeMethodCaller("PUT")})
	httpMod.Set("delete", &interpreter.BuiltinFunction{Name: "delete", Fn: makeMethodCaller("DELETE")})
	httpMod.Set("patch", &interpreter.BuiltinFunction{Name: "patch", Fn: makeMethodCaller("PATCH")})

	// http_serve(port, handler) -> server object
	register("http_serve", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "http_serve requires (port, handler) arguments")
		}
		iv, ok := args[0].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "port must be an integer")
		}
		return startHTTPServer(int(iv.Val), newLegacyHTTPHandler(args[1]))
	})

	httpMod.Set("router", &interpreter.BuiltinFunction{
		Name: "router",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) != 0 {
				return interpreter.NewError(400, "http.router does not accept arguments")
			}
			return newHTTPRouter().value
		},
	})
	httpMod.Set("response", &interpreter.BuiltinFunction{
		Name: "response",
		Fn:   httpResponseBuiltin,
	})
	httpMod.Set("json", &interpreter.BuiltinFunction{
		Name: "json",
		Fn:   httpJSONResponseBuiltin,
	})

	env.Set("http", httpMod)
	env.Set("std.http", httpMod)
}

func newLegacyHTTPHandler(handlerVal interpreter.Value) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqMap, err := makeHTTPRequestValue(w, r, false)
		if err != nil {
			writeHTTPError(w, http.StatusRequestEntityTooLarge, err.Error())
			return
		}
		response, err := invokeHTTPFunction(handlerVal, reqMap)
		if err != nil {
			writeHTTPError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeHTTPValue(w, response)
	})
}

func startHTTPServer(port int, handler http.Handler) interpreter.Value {
	if port < 0 || port > 65535 {
		return interpreter.NewError(400, "port must be between 0 and 65535")
	}
	addr := fmt.Sprintf(":%d", port)
	if err := sandbox.RequireNet(addr); err != nil {
		return handleIOError("http_serve", addr, err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return interpreter.NewError(500, "listen failed: "+err.Error())
	}
	srv := newHTTPServer(handler)
	inst := &HTTPServerInstance{server: srv, listener: ln}
	go func() {
		_ = srv.Serve(ln)
	}()

	serverObj := interpreter.NewMap()
	serverObj.Set("port", &interpreter.IntegerValue{Val: int64(ln.Addr().(*net.TCPAddr).Port)})
	serverObj.Set("close", &interpreter.BuiltinFunction{
		Name: "close",
		Fn: func(...interpreter.Value) interpreter.Value {
			if err := inst.Close(); err != nil {
				return interpreter.NewError(500, err.Error())
			}
			return interpreter.TRUE
		},
	})
	return serverObj
}

func makeHTTPRequestValue(w http.ResponseWriter, r *http.Request, parseJSON ...bool) (*interpreter.MapValue, error) {
	reqMap := interpreter.NewMap()
	reqMap.Set("method", &interpreter.StringValue{Val: r.Method})
	reqMap.Set("url", &interpreter.StringValue{Val: r.URL.String()})
	reqMap.Set("path", &interpreter.StringValue{Val: r.URL.Path})

	hdrs := interpreter.NewMap()
	for k, values := range r.Header {
		headerValues := make([]interpreter.Value, len(values))
		for i, value := range values {
			headerValues[i] = &interpreter.StringValue{Val: value}
		}
		hdrs.Set(k, interpreter.NewList(headerValues...))
	}
	reqMap.Set("headers", hdrs)

	query := interpreter.NewMap()
	for key, values := range r.URL.Query() {
		items := make([]interpreter.Value, len(values))
		for i, value := range values {
			items[i] = &interpreter.StringValue{Val: value}
		}
		query.Set(key, interpreter.NewList(items...))
	}
	reqMap.Set("query", query)
	reqMap.Set("params", interpreter.NewMap())
	reqMap.Set("dependencies", interpreter.NewMap())

	r.Body = http.MaxBytesReader(w, r.Body, maxHTTPBodyBytes)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	body := &interpreter.StringValue{Val: string(bodyBytes)}
	reqMap.Set("body", body)
	shouldParseJSON := len(parseJSON) == 0 || parseJSON[0]
	if shouldParseJSON && strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") && len(bodyBytes) > 0 {
		var decoded interface{}
		if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
			return nil, fmt.Errorf("invalid JSON request body: %w", err)
		}
		reqMap.Set("json", convertGoValueToAlca(decoded))
	}
	return reqMap, nil
}

func invokeHTTPFunction(fn interpreter.Value, args ...interpreter.Value) (interpreter.Value, error) {
	switch callable := fn.(type) {
	case *interpreter.BuiltinFunction:
		value := callable.Fn(args...)
		if errVal, ok := value.(*interpreter.ErrorValue); ok {
			return nil, fmt.Errorf("%s", errVal.Message)
		}
		return value, nil
	case *interpreter.FuncValue:
		childInterp := interpreter.New()
		value, err := childInterp.InvokeFunction(callable, args)
		if err != nil {
			return nil, err
		}
		if errorValue, ok := value.(*interpreter.ErrorValue); ok {
			return nil, fmt.Errorf("%s", errorValue.Message)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("handler must be a function")
	}
}

func writeHTTPValue(w http.ResponseWriter, response interpreter.Value) {
	switch res := response.(type) {
	case nil, *interpreter.NullValue:
		w.WriteHeader(http.StatusOK)
	case *interpreter.StringValue:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, res.Val)
	case *interpreter.MapValue:
		status := http.StatusOK
		if value, ok := res.Get("status"); ok {
			if code, ok := value.(*interpreter.IntegerValue); ok && code.Val >= 100 && code.Val <= 599 {
				status = int(code.Val)
			}
		}
		if value, ok := res.Get("headers"); ok {
			if headers, ok := value.(*interpreter.MapValue); ok {
				for _, key := range headers.Keys() {
					header, _ := headers.Get(key)
					setHTTPResponseHeaderValues(w.Header(), key, header)
				}
			}
		}
		body := ""
		if value, ok := res.Get("body"); ok {
			body = interpreter.CoerceToString(value)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	default:
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, interpreter.CoerceToString(response))
	}
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": message})
}

func httpResponseBuiltin(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 3 {
		return interpreter.NewError(400, "http.response requires (status, body?, headers?)")
	}
	status, ok := args[0].(*interpreter.IntegerValue)
	if !ok || status.Val < 100 || status.Val > 599 {
		return interpreter.NewError(400, "HTTP response status must be an integer from 100 to 599")
	}
	response := interpreter.NewMap()
	response.Set("status", status)
	if len(args) > 1 {
		response.Set("body", args[1])
	}
	if len(args) > 2 {
		headers, ok := args[2].(*interpreter.MapValue)
		if !ok {
			return interpreter.NewError(400, "HTTP response headers must be a map")
		}
		response.Set("headers", headers)
	}
	return response
}

func httpJSONResponseBuiltin(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 3 {
		return interpreter.NewError(400, "http.json requires (value, status?, headers?)")
	}
	status := int64(http.StatusOK)
	if len(args) > 1 {
		value, ok := args[1].(*interpreter.IntegerValue)
		if !ok || value.Val < 100 || value.Val > 599 {
			return interpreter.NewError(400, "HTTP response status must be an integer from 100 to 599")
		}
		status = value.Val
	}
	headers := interpreter.NewMap()
	headers.Set("Content-Type", &interpreter.StringValue{Val: "application/json; charset=utf-8"})
	if len(args) > 2 {
		extra, ok := args[2].(*interpreter.MapValue)
		if !ok {
			return interpreter.NewError(400, "HTTP response headers must be a map")
		}
		for _, key := range extra.Keys() {
			value, _ := extra.Get(key)
			headers.Set(key, value)
		}
	}
	data, err := json.Marshal(convertAlcaValueToGo(args[0]))
	if err != nil {
		return interpreter.NewError(400, "JSON response encoding failed: "+err.Error())
	}
	response := interpreter.NewMap()
	response.Set("status", &interpreter.IntegerValue{Val: status})
	response.Set("headers", headers)
	response.Set("body", &interpreter.StringValue{Val: string(data)})
	return response
}

func httpRequestTimeout(args []interpreter.Value) (time.Duration, error) {
	const defaultTimeout = 30 * time.Second
	if len(args) < 5 || args[4] == nil {
		return defaultTimeout, nil
	}
	options, ok := args[4].(*interpreter.MapValue)
	if !ok {
		return 0, fmt.Errorf("HTTP request options must be a map")
	}
	timeoutValue, exists := options.Get("timeout")
	if !exists {
		return defaultTimeout, nil
	}
	var seconds float64
	switch value := timeoutValue.(type) {
	case *interpreter.IntegerValue:
		seconds = float64(value.Val)
	case *interpreter.FloatValue:
		seconds = value.Val
	default:
		return 0, fmt.Errorf("HTTP timeout must be a number of seconds")
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > 300 {
		return 0, fmt.Errorf("HTTP timeout must be greater than 0 and no more than 300 seconds")
	}
	duration := time.Duration(seconds * float64(time.Second))
	if duration <= 0 {
		return 0, fmt.Errorf("HTTP timeout is too small")
	}
	return duration, nil
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func setHTTPHeaderValues(headers http.Header, key string, value interpreter.Value) {
	if values, ok := value.(*interpreter.ListValue); ok {
		headers.Del(key)
		for _, item := range values.ElementsSlice() {
			headers.Add(key, interpreter.CoerceToString(item))
		}
		return
	}
	headers.Set(key, interpreter.CoerceToString(value))
}

func setHTTPResponseHeaderValues(headers http.Header, key string, value interpreter.Value) {
	if values, ok := value.(*interpreter.ListValue); ok {
		for _, item := range values.ElementsSlice() {
			headers.Add(key, interpreter.CoerceToString(item))
		}
		return
	}
	headers.Add(key, interpreter.CoerceToString(value))
}
