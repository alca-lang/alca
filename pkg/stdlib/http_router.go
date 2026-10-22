package stdlib

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/alca-lang/alca/pkg/interpreter"
)

type httpRoute struct {
	method       string
	pattern      string
	segments     []string
	handler      interpreter.Value
	options      *interpreter.MapValue
	schema       *interpreter.MapValue
	dependencies *interpreter.MapValue
	score        int
}

type httpRouter struct {
	mu         sync.RWMutex
	routes     []*httpRoute
	middleware []interpreter.Value
	value      *interpreter.MapValue
}

func newHTTPRouter() *httpRouter {
	router := &httpRouter{value: interpreter.NewMap()}
	router.registerMethod("route", router.routeBuiltin)
	for method, name := range map[string]string{
		"GET": "get", "POST": "post", "PUT": "put", "PATCH": "patch", "DELETE": "delete",
		"HEAD": "head", "OPTIONS": "options",
	} {
		httpMethod := method
		router.registerMethod(name, func(args ...interpreter.Value) interpreter.Value {
			return router.addRoute(httpMethod, args...)
		})
	}
	router.registerMethod("use", router.useBuiltin)
	router.registerMethod("serve", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "router.serve requires a port")
		}
		port, ok := args[0].(*interpreter.IntegerValue)
		if !ok {
			return interpreter.NewError(400, "router.serve port must be an integer")
		}
		return startHTTPServer(int(port.Val), router)
	})
	router.registerMethod("openapi", router.openapiBuiltin)
	return router
}

func (router *httpRouter) registerMethod(name string, fn func(...interpreter.Value) interpreter.Value) {
	router.value.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
}

func (router *httpRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := makeHTTPRequestValue(w, r)
	if err != nil {
		status := http.StatusBadRequest
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
		}
		writeHTTPError(w, status, err.Error())
		return
	}

	route, params, pathMatched := router.match(r.Method, r.URL.Path)
	if route == nil {
		if pathMatched {
			allowed := router.allowedMethods(r.URL.Path)
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			writeHTTPError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeHTTPError(w, http.StatusNotFound, "route not found")
		return
	}

	paramMap := interpreter.NewMap()
	for key, value := range params {
		paramMap.Set(key, &interpreter.StringValue{Val: value})
	}
	req.Set("params", paramMap)

	if err := validateHTTPRequest(req, route.schema); err != nil {
		status := http.StatusUnprocessableEntity
		var requestError *httpRequestError
		if errors.As(err, &requestError) {
			status = requestError.status
		}
		writeHTTPError(w, status, err.Error())
		return
	}
	if err := resolveHTTPDependencies(req, route.dependencies); err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	router.mu.RLock()
	middleware := append([]interpreter.Value(nil), router.middleware...)
	router.mu.RUnlock()
	response, err := router.runMiddleware(middleware, 0, route.handler, req)
	if err != nil {
		writeHTTPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if r.Method == http.MethodHead {
		writeHTTPValue(httpHeadResponseWriter{ResponseWriter: w}, response)
		return
	}
	writeHTTPValue(w, response)
}

func (router *httpRouter) routeBuiltin(args ...interpreter.Value) interpreter.Value {
	if len(args) != 3 && len(args) != 4 {
		return interpreter.NewError(400, "router.route requires (method, path, handler, options?)")
	}
	method, ok := args[0].(*interpreter.StringValue)
	if !ok {
		return interpreter.NewError(400, "route method must be a string")
	}
	return router.addRoute(strings.ToUpper(method.Val), args[1:]...)
}

func (router *httpRouter) addRoute(method string, args ...interpreter.Value) interpreter.Value {
	if len(args) != 2 && len(args) != 3 {
		return interpreter.NewError(400, "route registration requires (path, handler, options?)")
	}
	path, ok := args[0].(*interpreter.StringValue)
	if !ok {
		return interpreter.NewError(400, "route path must be a string")
	}
	if path.Val == "" || path.Val[0] != '/' {
		return interpreter.NewError(400, "route path must start with '/'")
	}
	if _, ok := args[1].(*interpreter.FuncValue); !ok {
		if _, builtin := args[1].(*interpreter.BuiltinFunction); !builtin {
			return interpreter.NewError(400, "route handler must be a function")
		}
	}

	options := interpreter.NewMap()
	if len(args) == 3 {
		var valid bool
		options, valid = args[2].(*interpreter.MapValue)
		if !valid {
			return interpreter.NewError(400, "route options must be a map")
		}
	}
	var schema, dependencies *interpreter.MapValue
	if value, exists := options.Get("schema"); exists {
		var valid bool
		schema, valid = value.(*interpreter.MapValue)
		if !valid {
			return interpreter.NewError(400, "route schema must be a map")
		}
		if err := validateSchemaDefinition(schema); err != nil {
			return interpreter.NewError(400, err.Error())
		}
	}
	if value, exists := options.Get("dependencies"); exists {
		var valid bool
		dependencies, valid = value.(*interpreter.MapValue)
		if !valid {
			return interpreter.NewError(400, "route dependencies must be a map of names to functions")
		}
		for _, key := range dependencies.Keys() {
			dependency, _ := dependencies.Get(key)
			if !isHTTPCallable(dependency) {
				return interpreter.NewError(400, "route dependency "+key+" must be a function")
			}
		}
	}

	segments, score, err := parseHTTPRoutePattern(path.Val)
	if err != nil {
		return interpreter.NewError(400, err.Error())
	}
	route := &httpRoute{
		method:       method,
		pattern:      path.Val,
		segments:     segments,
		handler:      args[1],
		options:      options,
		schema:       schema,
		dependencies: dependencies,
		score:        score,
	}
	router.mu.Lock()
	defer router.mu.Unlock()
	for _, current := range router.routes {
		if current.method == method && current.pattern == path.Val {
			return interpreter.NewError(400, "duplicate route "+method+" "+path.Val)
		}
	}
	router.routes = append(router.routes, route)
	return interpreter.TRUE
}

func (router *httpRouter) useBuiltin(args ...interpreter.Value) interpreter.Value {
	if len(args) != 1 || !isHTTPCallable(args[0]) {
		return interpreter.NewError(400, "router.use requires a middleware function")
	}
	router.mu.Lock()
	router.middleware = append(router.middleware, args[0])
	router.mu.Unlock()
	return interpreter.TRUE
}

func (router *httpRouter) openapiBuiltin(args ...interpreter.Value) interpreter.Value {
	if len(args) > 1 {
		return interpreter.NewError(400, "router.openapi accepts an optional route path")
	}
	if len(args) == 0 {
		return router.openapiDocument()
	}
	path, ok := args[0].(*interpreter.StringValue)
	if !ok || path.Val == "" || path.Val[0] != '/' {
		return interpreter.NewError(400, "OpenAPI route path must start with '/'")
	}
	handler := &interpreter.BuiltinFunction{
		Name: "openapi_handler",
		Fn: func(...interpreter.Value) interpreter.Value {
			return httpJSONResponseBuiltin(router.openapiDocument())
		},
	}
	return router.addRoute(http.MethodGet, []interpreter.Value{
		&interpreter.StringValue{Val: path.Val},
		handler,
	}...)
}

func (router *httpRouter) match(method, path string) (*httpRoute, map[string]string, bool) {
	router.mu.RLock()
	defer router.mu.RUnlock()
	var best *httpRoute
	var bestParams map[string]string
	pathMatched := false
	for _, route := range router.routes {
		params, ok := matchHTTPRoute(route, path)
		if !ok {
			continue
		}
		pathMatched = true
		methodMatches := route.method == method || (method == http.MethodHead && route.method == http.MethodGet)
		if methodMatches && (best == nil || route.score > best.score) {
			best = route
			bestParams = params
		}
	}
	return best, bestParams, pathMatched
}

type httpHeadResponseWriter struct {
	http.ResponseWriter
}

func (w httpHeadResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (router *httpRouter) allowedMethods(path string) []string {
	router.mu.RLock()
	defer router.mu.RUnlock()
	set := make(map[string]struct{})
	for _, route := range router.routes {
		if _, ok := matchHTTPRoute(route, path); ok {
			set[route.method] = struct{}{}
			if route.method == http.MethodGet {
				set[http.MethodHead] = struct{}{}
			}
		}
	}
	methods := make([]string, 0, len(set))
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		if _, ok := set[method]; ok {
			methods = append(methods, method)
		}
	}
	return methods
}

func (router *httpRouter) runMiddleware(stack []interpreter.Value, index int, handler interpreter.Value, req *interpreter.MapValue) (interpreter.Value, error) {
	if index >= len(stack) {
		return invokeHTTPFunction(handler, req)
	}
	next := &interpreter.BuiltinFunction{
		Name: "next",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			nextReq := req
			if len(args) > 1 {
				return interpreter.NewError(400, "next accepts at most one request")
			}
			if len(args) == 1 {
				var ok bool
				nextReq, ok = args[0].(*interpreter.MapValue)
				if !ok {
					return interpreter.NewError(400, "next request must be a map")
				}
			}
			result, err := router.runMiddleware(stack, index+1, handler, nextReq)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			if result == nil {
				return interpreter.NULL
			}
			return result
		},
	}
	return invokeHTTPFunction(stack[index], req, next)
}

func parseHTTPRoutePattern(pattern string) ([]string, int, error) {
	if strings.ContainsAny(pattern, "?#") {
		return nil, 0, fmt.Errorf("route patterns cannot contain a query or fragment")
	}
	if pattern != "/" && strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
	}
	segments := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	if pattern == "/" {
		segments = nil
	}
	score := 0
	seen := make(map[string]bool)
	for i, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			if strings.HasSuffix(name, ":path") {
				name = strings.TrimSuffix(name, ":path")
				if i != len(segments)-1 {
					return nil, 0, fmt.Errorf("path wildcard must be the last route segment")
				}
			}
			if name == "" || strings.ContainsAny(name, "{}:/") || seen[name] {
				return nil, 0, fmt.Errorf("invalid or duplicate path parameter %q", name)
			}
			seen[name] = true
			continue
		}
		if strings.ContainsAny(segment, "{}") {
			return nil, 0, fmt.Errorf("invalid route segment %q", segment)
		}
		score++
	}
	return segments, score, nil
}

func matchHTTPRoute(route *httpRoute, requestPath string) (map[string]string, bool) {
	pathSegments := strings.Split(strings.TrimPrefix(requestPath, "/"), "/")
	if requestPath == "/" {
		pathSegments = nil
	}
	params := make(map[string]string)
	for i, pattern := range route.segments {
		if strings.HasPrefix(pattern, "{") && strings.HasSuffix(pattern, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(pattern, "{"), "}")
			if strings.HasSuffix(name, ":path") {
				name = strings.TrimSuffix(name, ":path")
				if i >= len(pathSegments) {
					return nil, false
				}
				params[name] = strings.Join(pathSegments[i:], "/")
				return params, true
			}
			if i >= len(pathSegments) || pathSegments[i] == "" {
				return nil, false
			}
			decoded, err := url.PathUnescape(pathSegments[i])
			if err != nil {
				return nil, false
			}
			params[name] = decoded
			continue
		}
		if i >= len(pathSegments) || pathSegments[i] != pattern {
			return nil, false
		}
	}
	return params, len(pathSegments) == len(route.segments)
}

func isHTTPCallable(value interpreter.Value) bool {
	switch value.(type) {
	case *interpreter.BuiltinFunction, *interpreter.FuncValue:
		return true
	default:
		return false
	}
}

type httpFieldSpec struct {
	typeName string
	required bool
}

type httpRequestError struct {
	status  int
	message string
}

func (err *httpRequestError) Error() string {
	return err.message
}

func validateSchemaDefinition(schema *interpreter.MapValue) error {
	for _, section := range schema.Keys() {
		if section != "path" && section != "query" && section != "body" {
			return fmt.Errorf("unsupported validation schema section %q", section)
		}
		fields, ok := schema.Get(section)
		if !ok {
			continue
		}
		fieldMap, ok := fields.(*interpreter.MapValue)
		if !ok {
			return fmt.Errorf("schema section %q must be a map of fields", section)
		}
		for _, name := range fieldMap.Keys() {
			if _, err := parseHTTPFieldSpec(mustGet(fieldMap, name)); err != nil {
				return fmt.Errorf("schema %s.%s: %w", section, name, err)
			}
		}
	}
	return nil
}

func mustGet(value *interpreter.MapValue, key string) interpreter.Value {
	item, _ := value.Get(key)
	return item
}

func parseHTTPFieldSpec(value interpreter.Value) (httpFieldSpec, error) {
	spec := httpFieldSpec{required: true}
	if name, ok := value.(*interpreter.StringValue); ok {
		spec.typeName = name.Val
	} else if fields, ok := value.(*interpreter.MapValue); ok {
		typeValue, exists := fields.Get("type")
		if !exists {
			return spec, fmt.Errorf("descriptor requires a type")
		}
		typeName, ok := typeValue.(*interpreter.StringValue)
		if !ok {
			return spec, fmt.Errorf("type must be a string")
		}
		spec.typeName = typeName.Val
		if required, exists := fields.Get("required"); exists {
			boolean, ok := required.(*interpreter.BoolValue)
			if !ok {
				return spec, fmt.Errorf("required must be a boolean")
			}
			spec.required = boolean.Val
		}
	} else {
		return spec, fmt.Errorf("field descriptor must be a type string or map")
	}
	switch spec.typeName {
	case "any", "json", "string", "int", "float", "number", "bool", "object", "array":
		return spec, nil
	default:
		return spec, fmt.Errorf("unsupported type %q", spec.typeName)
	}
}

func validateHTTPRequest(request *interpreter.MapValue, schema *interpreter.MapValue) error {
	if schema == nil {
		return nil
	}
	for _, section := range []string{"path", "query", "body"} {
		value, exists := schema.Get(section)
		if !exists {
			continue
		}
		fields := value.(*interpreter.MapValue)
		var input *interpreter.MapValue
		switch section {
		case "path":
			value, _ := request.Get("params")
			input, _ = value.(*interpreter.MapValue)
		case "query":
			value, _ := request.Get("query")
			input, _ = value.(*interpreter.MapValue)
		case "body":
			jsonValue, hasJSON := request.Get("json")
			if !hasJSON {
				body, _ := request.Get("body")
				bodyString := interpreter.CoerceToString(body)
				if bodyString == "" {
					return fmt.Errorf("request body must be a JSON object")
				}
				var decoded interface{}
				if err := json.Unmarshal([]byte(bodyString), &decoded); err != nil {
					return &httpRequestError{
						status:  http.StatusBadRequest,
						message: fmt.Sprintf("request body must be valid JSON: %v", err),
					}
				}
				jsonValue = convertGoValueToAlca(decoded)
				request.Set("json", jsonValue)
			}
			input, _ = jsonValue.(*interpreter.MapValue)
			if input == nil {
				return fmt.Errorf("request JSON body must be an object")
			}
		}
		if input == nil {
			return fmt.Errorf("request %s must be an object", section)
		}
		converted := interpreter.NewMap()
		for _, name := range fields.Keys() {
			spec, err := parseHTTPFieldSpec(mustGet(fields, name))
			if err != nil {
				return err
			}
			field, present := input.Get(name)
			if !present {
				if spec.required {
					return fmt.Errorf("missing required %s field %q", section, name)
				}
				continue
			}
			if section == "query" {
				if list, ok := field.(*interpreter.ListValue); ok {
					items := list.ElementsSlice()
					if spec.typeName == "array" {
						values := make([]interpreter.Value, len(items))
						for i, item := range items {
							convertedItem, conversionErr := convertHTTPValue(item, "string", section, name)
							if conversionErr != nil {
								return conversionErr
							}
							values[i] = convertedItem
						}
						converted.Set(name, interpreter.NewList(values...))
						continue
					}
					if len(items) == 0 {
						continue
					}
					field = items[0]
				}
			}
			result, err := convertHTTPValue(field, spec.typeName, section, name)
			if err != nil {
				return err
			}
			converted.Set(name, result)
		}
		switch section {
		case "path":
			request.Set("params", converted)
		case "query":
			request.Set("query", converted)
		case "body":
			request.Set("json", converted)
		}
	}
	return nil
}

func convertHTTPValue(value interpreter.Value, typeName, section, name string) (interpreter.Value, error) {
	fail := func() (interpreter.Value, error) {
		return nil, fmt.Errorf("field %q in %s must be %s", name, section, typeName)
	}
	switch typeName {
	case "any", "json":
		return value, nil
	case "string":
		if _, ok := value.(*interpreter.StringValue); ok {
			return value, nil
		}
	case "int":
		switch item := value.(type) {
		case *interpreter.IntegerValue:
			return item, nil
		case *interpreter.StringValue:
			parsed, err := strconv.ParseInt(item.Val, 10, 64)
			if err == nil {
				return &interpreter.IntegerValue{Val: parsed}, nil
			}
		}
	case "float":
		switch item := value.(type) {
		case *interpreter.FloatValue:
			return item, nil
		case *interpreter.IntegerValue:
			return &interpreter.FloatValue{Val: float64(item.Val)}, nil
		case *interpreter.StringValue:
			parsed, err := strconv.ParseFloat(item.Val, 64)
			if err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
				return &interpreter.FloatValue{Val: parsed}, nil
			}
		}
	case "number":
		switch value.(type) {
		case *interpreter.IntegerValue, *interpreter.FloatValue:
			return value, nil
		case *interpreter.StringValue:
			if parsed, err := strconv.ParseInt(value.(*interpreter.StringValue).Val, 10, 64); err == nil {
				return &interpreter.IntegerValue{Val: parsed}, nil
			}
			if parsed, err := strconv.ParseFloat(value.(*interpreter.StringValue).Val, 64); err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) {
				return &interpreter.FloatValue{Val: parsed}, nil
			}
		}
	case "bool":
		switch item := value.(type) {
		case *interpreter.BoolValue:
			return item, nil
		case *interpreter.StringValue:
			parsed, err := strconv.ParseBool(item.Val)
			if err == nil {
				if parsed {
					return interpreter.TRUE, nil
				}
				return interpreter.FALSE, nil
			}
		}
	case "object":
		if _, ok := value.(*interpreter.MapValue); ok {
			return value, nil
		}
	case "array":
		if _, ok := value.(*interpreter.ListValue); ok {
			return value, nil
		}
	}
	return fail()
}

func resolveHTTPDependencies(request *interpreter.MapValue, dependencies *interpreter.MapValue) error {
	if dependencies == nil {
		return nil
	}
	resolved := interpreter.NewMap()
	for _, name := range dependencies.Keys() {
		fn, _ := dependencies.Get(name)
		value, err := invokeHTTPFunction(fn, request)
		if err != nil {
			return fmt.Errorf("dependency %q failed: %w", name, err)
		}
		resolved.Set(name, value)
	}
	request.Set("dependencies", resolved)
	return nil
}

func (router *httpRouter) openapiDocument() interpreter.Value {
	router.mu.RLock()
	routes := append([]*httpRoute(nil), router.routes...)
	router.mu.RUnlock()

	paths := make(map[string]interface{})
	for _, route := range routes {
		path := openAPIPath(route.pattern)
		item, _ := paths[path].(map[string]interface{})
		if item == nil {
			item = make(map[string]interface{})
			paths[path] = item
		}
		operation := map[string]interface{}{
			"responses": map[string]interface{}{
				"200": map[string]interface{}{"description": "Successful response"},
			},
		}
		for _, metadata := range []string{"summary", "description"} {
			if value, ok := route.options.Get(metadata); ok {
				operation[metadata] = interpreter.CoerceToString(value)
			}
		}
		if tags, ok := route.options.Get("tags"); ok {
			operation["tags"] = convertAlcaValueToGo(tags)
		}
		if route.schema != nil {
			parameters := make([]interface{}, 0)
			for _, section := range []string{"path", "query"} {
				fieldsValue, exists := route.schema.Get(section)
				if !exists {
					continue
				}
				fields := fieldsValue.(*interpreter.MapValue)
				for _, name := range fields.Keys() {
					spec, err := parseHTTPFieldSpec(mustGet(fields, name))
					if err != nil {
						continue
					}
					parameters = append(parameters, map[string]interface{}{
						"name": name, "in": section, "required": spec.required,
						"schema": map[string]interface{}{"type": openAPIType(spec.typeName)},
					})
				}
			}
			if len(parameters) > 0 {
				operation["parameters"] = parameters
			}
			if bodyValue, exists := route.schema.Get("body"); exists {
				fields := bodyValue.(*interpreter.MapValue)
				operation["requestBody"] = map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": openAPIObjectSchema(fields),
						},
					},
				}
			}
		}
		item[strings.ToLower(route.method)] = operation
	}
	return convertGoValueToAlca(map[string]interface{}{
		"openapi": "3.0.3",
		"info":    map[string]interface{}{"title": "Alca API", "version": "1.0.0"},
		"paths":   paths,
	})
}

func openAPIPath(pattern string) string {
	return strings.ReplaceAll(pattern, ":path}", "}")
}

func openAPIType(name string) string {
	switch name {
	case "int":
		return "integer"
	case "float", "number":
		return "number"
	case "bool":
		return "boolean"
	case "object":
		return "object"
	case "array":
		return "array"
	default:
		return "string"
	}
}

func openAPIObjectSchema(fields *interpreter.MapValue) map[string]interface{} {
	properties := make(map[string]interface{})
	required := make([]string, 0)
	for _, name := range fields.Keys() {
		spec, err := parseHTTPFieldSpec(mustGet(fields, name))
		if err != nil {
			continue
		}
		properties[name] = map[string]interface{}{"type": openAPIType(spec.typeName)}
		if spec.required {
			required = append(required, name)
		}
	}
	schema := map[string]interface{}{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
