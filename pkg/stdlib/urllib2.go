package stdlib

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

const maxURLRequestTimeout = 300 * time.Second

// RegisterUrllib2Builtins registers URL parsing and HTTP request helpers.
func RegisterUrllib2Builtins(env *interpreter.Environment) {
	module := interpreter.NewMap()
	register := func(name string, fn func(...interpreter.Value) interpreter.Value) {
		module.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	register("parse", urllib2Parse)
	register("split", urllib2Split)
	register("join", urllib2Join)
	register("quote", urllib2Quote)
	register("unquote", urllib2Unquote)
	register("urlencode", urllib2EncodeQuery)
	register("parse_qs", urllib2ParseQuery)
	register("parse_qsl", urllib2ParseQueryList)
	register("request", urllib2Request)
	register("urlopen", urllib2URLopen)
	register("get", urllib2Get)
	register("post", urllib2Post)
	register("urlretrieve", urllib2URLRetrieve)

	env.Set("urllib2", module)
	env.Set("std.urllib2", module)
}

func urllib2Parse(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 2 {
		return interpreter.NewError(400, "urllib2.parse requires (url, scheme?)")
	}
	raw := interpreter.CoerceToString(args[0])
	parsed, err := url.Parse(raw)
	if err != nil {
		return interpreter.NewError(400, "invalid URL: "+err.Error())
	}
	if len(args) == 2 && parsed.Scheme == "" {
		parsed.Scheme = interpreter.CoerceToString(args[1])
	}
	path, params := splitURLParams(parsed.EscapedPath())
	netloc := parsed.Host
	if parsed.User != nil {
		netloc = parsed.User.String() + "@" + netloc
	}
	result := interpreter.NewMap()
	result.Set("scheme", &interpreter.StringValue{Val: parsed.Scheme})
	result.Set("netloc", &interpreter.StringValue{Val: netloc})
	result.Set("path", &interpreter.StringValue{Val: path})
	result.Set("params", &interpreter.StringValue{Val: params})
	result.Set("query", &interpreter.StringValue{Val: parsed.RawQuery})
	result.Set("fragment", &interpreter.StringValue{Val: parsed.Fragment})
	result.Set("username", urllib2String(parsed.User, func(user *url.Userinfo) string {
		return user.Username()
	}))
	result.Set("password", urllib2String(parsed.User, func(user *url.Userinfo) string {
		password, _ := user.Password()
		return password
	}))
	hostname := parsed.Hostname()
	result.Set("hostname", &interpreter.StringValue{Val: hostname})
	port := interpreter.Value(interpreter.NULL)
	if parsed.Port() != "" {
		if value, parseErr := strconv.ParseInt(parsed.Port(), 10, 64); parseErr == nil {
			port = &interpreter.IntegerValue{Val: value}
		}
	}
	result.Set("port", port)
	return result
}

func urllib2String(user *url.Userinfo, value func(*url.Userinfo) string) interpreter.Value {
	if user == nil {
		return interpreter.NULL
	}
	return &interpreter.StringValue{Val: value(user)}
}

func splitURLParams(path string) (string, string) {
	index := strings.LastIndexByte(path, ';')
	if index < 0 {
		return path, ""
	}
	return path[:index], path[index+1:]
}

func urllib2Split(args ...interpreter.Value) interpreter.Value {
	parsed := urllib2Parse(args...)
	if err, ok := parsed.(*interpreter.ErrorValue); ok {
		return err
	}
	fields := parsed.(*interpreter.MapValue)
	names := []string{"scheme", "netloc", "path", "params", "query", "fragment"}
	values := make([]interpreter.Value, len(names))
	for i, name := range names {
		values[i], _ = fields.Get(name)
	}
	return interpreter.NewList(values...)
}

func urllib2Join(args ...interpreter.Value) interpreter.Value {
	if len(args) != 2 {
		return interpreter.NewError(400, "urllib2.join requires (base_url, relative_url)")
	}
	base, err := url.Parse(interpreter.CoerceToString(args[0]))
	if err != nil {
		return interpreter.NewError(400, "invalid base URL: "+err.Error())
	}
	reference, err := url.Parse(interpreter.CoerceToString(args[1]))
	if err != nil {
		return interpreter.NewError(400, "invalid relative URL: "+err.Error())
	}
	return &interpreter.StringValue{Val: base.ResolveReference(reference).String()}
}

func urllib2Quote(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 2 {
		return interpreter.NewError(400, "urllib2.quote requires (value, safe?)")
	}
	safe := "/"
	if len(args) == 2 {
		safe = interpreter.CoerceToString(args[1])
	}
	escaped := url.QueryEscape(interpreter.CoerceToString(args[0]))
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	for _, r := range safe {
		escaped = strings.ReplaceAll(escaped, url.QueryEscape(string(r)), string(r))
	}
	return &interpreter.StringValue{Val: escaped}
}

func urllib2Unquote(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 2 {
		return interpreter.NewError(400, "urllib2.unquote requires (value, plus_as_space?)")
	}
	value := interpreter.CoerceToString(args[0])
	if len(args) == 2 {
		if boolean, ok := args[1].(*interpreter.BoolValue); !ok {
			return interpreter.NewError(400, "plus_as_space must be a boolean")
		} else if boolean.Val {
			decoded, err := url.QueryUnescape(value)
			if err != nil {
				return interpreter.NewError(400, "invalid URL escape: "+err.Error())
			}
			return &interpreter.StringValue{Val: decoded}
		}
	}
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return interpreter.NewError(400, "invalid URL escape: "+err.Error())
	}
	return &interpreter.StringValue{Val: decoded}
}

func urllib2EncodeQuery(args ...interpreter.Value) interpreter.Value {
	if len(args) != 1 {
		return interpreter.NewError(400, "urllib2.urlencode requires a map")
	}
	values, ok := args[0].(*interpreter.MapValue)
	if !ok {
		return interpreter.NewError(400, "urllib2.urlencode expects a map")
	}
	query := url.Values{}
	for _, key := range values.Keys() {
		value, _ := values.Get(key)
		switch list := value.(type) {
		case *interpreter.ListValue:
			for _, item := range list.ElementsSlice() {
				query.Add(key, interpreter.CoerceToString(item))
			}
		default:
			query.Add(key, interpreter.CoerceToString(value))
		}
	}
	return &interpreter.StringValue{Val: query.Encode()}
}

func urllib2ParseQuery(args ...interpreter.Value) interpreter.Value {
	if len(args) != 1 {
		return interpreter.NewError(400, "urllib2.parse_qs requires a query string")
	}
	values, err := url.ParseQuery(strings.TrimPrefix(interpreter.CoerceToString(args[0]), "?"))
	if err != nil {
		return interpreter.NewError(400, "invalid query string: "+err.Error())
	}
	result := interpreter.NewMap()
	for key, items := range values {
		converted := make([]interpreter.Value, len(items))
		for i, item := range items {
			converted[i] = &interpreter.StringValue{Val: item}
		}
		result.Set(key, interpreter.NewList(converted...))
	}
	return result
}

func urllib2ParseQueryList(args ...interpreter.Value) interpreter.Value {
	if len(args) != 1 {
		return interpreter.NewError(400, "urllib2.parse_qsl requires a query string")
	}
	query := strings.TrimPrefix(interpreter.CoerceToString(args[0]), "?")
	pairs := make([]interpreter.Value, 0)
	for _, pair := range strings.Split(query, "&") {
		if pair == "" {
			continue
		}
		key, value, hasValue := strings.Cut(pair, "=")
		if !hasValue || value == "" {
			continue
		}
		decodedKey, err := url.QueryUnescape(key)
		if err != nil {
			return interpreter.NewError(400, "invalid query string: "+err.Error())
		}
		decodedValue, err := url.QueryUnescape(value)
		if err != nil {
			return interpreter.NewError(400, "invalid query string: "+err.Error())
		}
		pairs = append(pairs, interpreter.NewList(
			&interpreter.StringValue{Val: decodedKey},
			&interpreter.StringValue{Val: decodedValue},
		))
	}
	return interpreter.NewList(pairs...)
}

func urllib2Request(args ...interpreter.Value) interpreter.Value {
	if len(args) < 2 || len(args) > 5 {
		return interpreter.NewError(400, "urllib2.request requires (method, url, data?, headers?, timeout?)")
	}
	method := strings.ToUpper(interpreter.CoerceToString(args[0]))
	rawURL := interpreter.CoerceToString(args[1])
	if method == "" {
		return interpreter.NewError(400, "HTTP method cannot be empty")
	}
	if err := sandbox.RequireURL(rawURL); err != nil {
		return handleIOError("urllib2.request", rawURL, err)
	}

	var body io.Reader
	contentType := ""
	if len(args) > 2 && args[2] != nil && args[2].Type() != interpreter.NULL_VAL {
		switch data := args[2].(type) {
		case *interpreter.StringValue:
			body = strings.NewReader(data.Val)
		case *interpreter.BlobValue:
			body = strings.NewReader(string(data.Data))
		case *interpreter.MapValue:
			values := url.Values{}
			for _, key := range data.Keys() {
				value, _ := data.Get(key)
				if list, ok := value.(*interpreter.ListValue); ok {
					for _, item := range list.ElementsSlice() {
						values.Add(key, interpreter.CoerceToString(item))
					}
				} else {
					values.Add(key, interpreter.CoerceToString(value))
				}
			}
			body = strings.NewReader(values.Encode())
			contentType = "application/x-www-form-urlencoded"
		default:
			return interpreter.NewError(400, "request data must be a string, blob, map, or null")
		}
	}

	request, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return interpreter.NewError(400, "invalid HTTP request: "+err.Error())
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if len(args) > 3 && args[3] != nil && args[3].Type() != interpreter.NULL_VAL {
		headers, ok := args[3].(*interpreter.MapValue)
		if !ok {
			return interpreter.NewError(400, "request headers must be a map")
		}
		for _, key := range headers.Keys() {
			value, _ := headers.Get(key)
			setHTTPHeaderValues(request.Header, key, value)
		}
	}
	timeout, err := urllib2Timeout(args)
	if err != nil {
		return interpreter.NewError(400, err.Error())
	}
	response, err := policyHTTPClient(timeout).Do(request)
	if err != nil {
		return handleIOError("urllib2.request", rawURL, err)
	}
	defer response.Body.Close()
	bodyBytes, err := readLimitedHTTPBody(response.Body)
	if err != nil {
		if errors.Is(err, errHTTPBodyTooLarge) {
			return interpreter.NewError(413, err.Error())
		}
		return interpreter.NewError(500, "failed to read HTTP response: "+err.Error())
	}
	return urllib2ResponseValue(response, bodyBytes)
}

func urllib2Timeout(args []interpreter.Value) (time.Duration, error) {
	timeout := 30 * time.Second
	if len(args) < 5 || args[4] == nil || args[4].Type() == interpreter.NULL_VAL {
		return timeout, nil
	}
	var seconds float64
	switch value := args[4].(type) {
	case *interpreter.IntegerValue:
		seconds = float64(value.Val)
	case *interpreter.FloatValue:
		seconds = value.Val
	default:
		return 0, fmt.Errorf("timeout must be a number of seconds")
	}
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || seconds > maxURLRequestTimeout.Seconds() {
		return 0, fmt.Errorf("timeout must be greater than 0 and at most 300 seconds")
	}
	duration := time.Duration(seconds * float64(time.Second))
	if duration <= 0 {
		return 0, fmt.Errorf("timeout is too small")
	}
	return duration, nil
}

func urllib2ResponseValue(response *http.Response, body []byte) interpreter.Value {
	headers := interpreter.NewMap()
	for key, values := range response.Header {
		converted := make([]interpreter.Value, len(values))
		for i, value := range values {
			converted[i] = &interpreter.StringValue{Val: value}
		}
		headers.Set(key, interpreter.NewList(converted...))
	}
	result := interpreter.NewMap()
	result.Set("status", &interpreter.IntegerValue{Val: int64(response.StatusCode)})
	result.Set("reason", &interpreter.StringValue{Val: response.Status})
	result.Set("body", &interpreter.StringValue{Val: string(body)})
	result.Set("headers", headers)
	result.Set("url", &interpreter.StringValue{Val: response.Request.URL.String()})
	return result
}

func urllib2URLopen(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 4 {
		return interpreter.NewError(400, "urllib2.urlopen requires (url, data?, headers?, timeout?)")
	}
	method := http.MethodGet
	if len(args) > 1 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
		method = http.MethodPost
	}
	newArgs := []interpreter.Value{
		&interpreter.StringValue{Val: method},
		args[0],
	}
	newArgs = append(newArgs, args[1:]...)
	return urllib2Request(newArgs...)
}

func urllib2Get(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 3 {
		return interpreter.NewError(400, "urllib2.get requires (url, headers?, timeout?)")
	}
	var headers interpreter.Value = interpreter.NULL
	var timeout interpreter.Value = interpreter.NULL
	if len(args) > 1 {
		headers = args[1]
	}
	if len(args) > 2 {
		timeout = args[2]
	}
	return urllib2Request(&interpreter.StringValue{Val: http.MethodGet}, args[0], interpreter.NULL, headers, timeout)
}

func urllib2Post(args ...interpreter.Value) interpreter.Value {
	if len(args) < 2 || len(args) > 4 {
		return interpreter.NewError(400, "urllib2.post requires (url, data, headers?, timeout?)")
	}
	var headers interpreter.Value = interpreter.NULL
	var timeout interpreter.Value = interpreter.NULL
	if len(args) > 2 {
		headers = args[2]
	}
	if len(args) > 3 {
		timeout = args[3]
	}
	return urllib2Request(&interpreter.StringValue{Val: http.MethodPost}, args[0], args[1], headers, timeout)
}

func urllib2URLRetrieve(args ...interpreter.Value) interpreter.Value {
	if len(args) != 2 {
		return interpreter.NewError(400, "urllib2.urlretrieve requires (url, destination)")
	}
	rawURL := interpreter.CoerceToString(args[0])
	destination := interpreter.CoerceToString(args[1])
	if err := sandbox.RequireURL(rawURL); err != nil {
		return handleIOError("urllib2.urlretrieve", rawURL, err)
	}
	if err := sandbox.RequireWrite(destination); err != nil {
		return handleIOError("urllib2.urlretrieve", destination, err)
	}
	response, err := policyHTTPClient(5 * time.Minute).Get(rawURL)
	if err != nil {
		return handleIOError("urllib2.urlretrieve", rawURL, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return interpreter.NewError(int64(response.StatusCode), "HTTP download failed: "+response.Status)
	}
	data, err := readLimitedHTTPBody(response.Body)
	if err != nil {
		if errors.Is(err, errHTTPBodyTooLarge) {
			return interpreter.NewError(413, err.Error())
		}
		return interpreter.NewError(500, "failed to read download: "+err.Error())
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return handleIOError("urllib2.urlretrieve", destination, err)
	}
	if err := os.WriteFile(destination, data, 0644); err != nil {
		return handleIOError("urllib2.urlretrieve", destination, err)
	}
	result := interpreter.NewMap()
	result.Set("filename", &interpreter.StringValue{Val: destination})
	result.Set("headers", urllib2HeaderMap(response.Header))
	return result
}

func urllib2HeaderMap(source http.Header) *interpreter.MapValue {
	result := interpreter.NewMap()
	for key, values := range source {
		items := make([]interpreter.Value, len(values))
		for i, item := range values {
			items[i] = &interpreter.StringValue{Val: item}
		}
		result.Set(key, interpreter.NewList(items...))
	}
	return result
}
