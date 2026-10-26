// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

const maxWebSocketMessageSize = 16 << 20

type WebSocketConnection struct {
	conn     net.Conn
	reader   *bufio.Reader
	writeMu  sync.Mutex
	readMu   sync.Mutex
	stateMu  sync.Mutex
	isClosed bool
}

var (
	websocketConnections = make(map[string]*WebSocketConnection)
	websocketMutex       sync.RWMutex
	connectionCounter    int64
)

func registerWebSocketConnection(ws *WebSocketConnection) string {
	websocketMutex.Lock()
	defer websocketMutex.Unlock()
	connectionCounter++
	id := fmt.Sprintf("ws_%d", connectionCounter)
	websocketConnections[id] = ws
	return id
}

func getWebSocketConnection(id string) (*WebSocketConnection, bool) {
	websocketMutex.RLock()
	defer websocketMutex.RUnlock()
	ws, ok := websocketConnections[id]
	return ws, ok
}

func removeWebSocketConnection(id string) {
	websocketMutex.Lock()
	delete(websocketConnections, id)
	websocketMutex.Unlock()
}

func (ws *WebSocketConnection) close() error {
	ws.stateMu.Lock()
	if ws.isClosed {
		ws.stateMu.Unlock()
		return nil
	}
	ws.isClosed = true
	ws.stateMu.Unlock()
	return ws.conn.Close()
}

func (ws *WebSocketConnection) writeFrame(opcode byte, payload []byte) error {
	if len(payload) > maxWebSocketMessageSize {
		return errors.New("WebSocket message exceeds 16 MiB")
	}
	if opcode&0x8 != 0 && len(payload) > 125 {
		return errors.New("invalid WebSocket control frame")
	}
	mask := [4]byte{}
	if _, err := rand.Read(mask[:]); err != nil {
		return fmt.Errorf("generate WebSocket mask: %w", err)
	}
	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, 0x80|opcode)
	switch {
	case len(payload) < 126:
		frame = append(frame, 0x80|byte(len(payload)))
	case len(payload) <= 65535:
		frame = append(frame, 0x80|126, byte(len(payload)>>8), byte(len(payload)))
	default:
		frame = append(frame, 0x80|127)
		var extended [8]byte
		binary.BigEndian.PutUint64(extended[:], uint64(len(payload)))
		frame = append(frame, extended[:]...)
	}
	frame = append(frame, mask[:]...)
	start := len(frame)
	frame = append(frame, payload...)
	for i := start; i < len(frame); i++ {
		frame[i] ^= mask[(i-start)&3]
	}

	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()
	ws.stateMu.Lock()
	closed := ws.isClosed
	ws.stateMu.Unlock()
	if closed {
		return errors.New("WebSocket connection is closed")
	}
	for len(frame) > 0 {
		n, err := ws.conn.Write(frame)
		if err != nil {
			return err
		}
		frame = frame[n:]
	}
	return nil
}

type webSocketFrame struct {
	fin     bool
	opcode  byte
	payload []byte
}

func (ws *WebSocketConnection) readFrame() (webSocketFrame, error) {
	var frame webSocketFrame
	var header [2]byte
	if _, err := io.ReadFull(ws.reader, header[:]); err != nil {
		return frame, err
	}
	frame.fin = header[0]&0x80 != 0
	if header[0]&0x70 != 0 {
		return frame, errors.New("unsupported WebSocket extension bits")
	}
	frame.opcode = header[0] & 0x0f
	if header[1]&0x80 != 0 {
		return frame, errors.New("server sent a masked WebSocket frame")
	}
	size := uint64(header[1] & 0x7f)
	switch size {
	case 126:
		var extended [2]byte
		if _, err := io.ReadFull(ws.reader, extended[:]); err != nil {
			return frame, err
		}
		size = uint64(binary.BigEndian.Uint16(extended[:]))
		if size < 126 {
			return frame, errors.New("non-canonical WebSocket frame length")
		}
	case 127:
		var extended [8]byte
		if _, err := io.ReadFull(ws.reader, extended[:]); err != nil {
			return frame, err
		}
		size = binary.BigEndian.Uint64(extended[:])
		if size < 65536 || size>>63 != 0 {
			return frame, errors.New("invalid WebSocket frame length")
		}
	}
	if size > maxWebSocketMessageSize {
		return frame, errors.New("WebSocket message exceeds 16 MiB")
	}
	if frame.opcode >= 8 && (!frame.fin || size > 125) {
		return frame, errors.New("invalid WebSocket control frame")
	}
	switch frame.opcode {
	case 0, 1, 2, 8, 9, 10:
	default:
		return frame, errors.New("unknown WebSocket opcode")
	}
	frame.payload = make([]byte, int(size))
	if _, err := io.ReadFull(ws.reader, frame.payload); err != nil {
		return frame, err
	}
	return frame, nil
}

func (ws *WebSocketConnection) readMessage() (byte, []byte, error) {
	var opcode byte
	var payload []byte
	for {
		frame, err := ws.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch frame.opcode {
		case 8:
			if len(frame.payload) == 1 {
				return 0, nil, errors.New("invalid WebSocket close payload")
			}
			if len(frame.payload) >= 2 {
				code := int(binary.BigEndian.Uint16(frame.payload[:2]))
				if !validWebSocketCloseCode(code) || !utf8.Valid(frame.payload[2:]) {
					return 0, nil, errors.New("invalid WebSocket close status or reason")
				}
			}
			_ = ws.writeFrame(8, frame.payload)
			_ = ws.close()
			return 8, frame.payload, io.EOF
		case 9:
			if err := ws.writeFrame(10, frame.payload); err != nil {
				return 0, nil, err
			}
			continue
		case 10:
			continue
		case 1, 2:
			if opcode != 0 {
				return 0, nil, errors.New("new WebSocket message before fragmented message completed")
			}
			opcode = frame.opcode
			payload = append(payload, frame.payload...)
		case 0:
			if opcode == 0 {
				return 0, nil, errors.New("unexpected WebSocket continuation frame")
			}
			payload = append(payload, frame.payload...)
		}
		if len(payload) > maxWebSocketMessageSize {
			return 0, nil, errors.New("WebSocket message exceeds 16 MiB")
		}
		if frame.fin {
			if opcode == 1 && !utf8.Valid(payload) {
				return 0, nil, errors.New("WebSocket text message is not valid UTF-8")
			}
			return opcode, payload, nil
		}
		if frame.opcode >= 8 {
			continue
		}
	}
}

func websocketConnectTimeout(value interpreter.Value) (time.Duration, error) {
	if value == nil || value.Type() == interpreter.NULL_VAL {
		return 30 * time.Second, nil
	}
	var seconds float64
	switch v := value.(type) {
	case *interpreter.IntegerValue:
		seconds = float64(v.Val)
	case *interpreter.FloatValue:
		seconds = v.Val
	default:
		return 0, errors.New("timeout must be a number of seconds")
	}
	if seconds <= 0 || seconds > 300 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, errors.New("timeout must be greater than 0 and at most 300 seconds")
	}
	timeout := time.Duration(seconds * float64(time.Second))
	if timeout <= 0 {
		return 0, errors.New("timeout is too small")
	}
	return timeout, nil
}

func websocket_connect(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 4 {
		return interpreter.NewError(400, "urllib2.websocket_connect requires (url, headers?, protocols?, timeout?)")
	}
	urlStr := interpreter.CoerceToString(args[0])
	parsedURL, err := url.Parse(urlStr)
	if err != nil || parsedURL.Host == "" || parsedURL.User != nil || parsedURL.Fragment != "" {
		return interpreter.NewError(400, "invalid WebSocket URL")
	}
	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		return interpreter.NewError(400, "WebSocket URL must use ws:// or wss:// scheme")
	}
	if err := sandbox.RequireURL(urlStr); err != nil {
		return handleIOError("urllib2.websocket_connect", urlStr, err)
	}
	timeout := 30 * time.Second
	if len(args) > 3 {
		timeout, err = websocketConnectTimeout(args[3])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
	}
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", net.JoinHostPort(parsedURL.Hostname(), port))
	if err != nil {
		return interpreter.NewError(500, "WebSocket connection failed: "+err.Error())
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if parsedURL.Scheme == "wss" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: parsedURL.Hostname(), MinVersion: tls.VersionTLS12})
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return interpreter.NewError(500, "WebSocket TLS handshake failed: "+err.Error())
		}
		conn = tlsConn
	}
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return interpreter.NewError(500, "failed to generate WebSocket key: "+err.Error())
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	httpURL := *parsedURL
	if httpURL.Scheme == "ws" {
		httpURL.Scheme = "http"
	} else {
		httpURL.Scheme = "https"
	}
	req, err := http.NewRequest(http.MethodGet, httpURL.String(), nil)
	if err != nil {
		conn.Close()
		return interpreter.NewError(400, "invalid WebSocket request: "+err.Error())
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")

	var protocols []string
	if len(args) > 2 && args[2] != nil && args[2].Type() != interpreter.NULL_VAL {
		values, ok := args[2].(*interpreter.ListValue)
		if !ok {
			conn.Close()
			return interpreter.NewError(400, "protocols must be a list of strings")
		}
		for _, value := range values.Elements {
			protocol := interpreter.CoerceToString(value)
			if !isWebSocketToken(protocol) {
				conn.Close()
				return interpreter.NewError(400, "invalid WebSocket protocol")
			}
			protocols = append(protocols, protocol)
		}
	}
	if len(protocols) > 0 {
		req.Header.Set("Sec-WebSocket-Protocol", strings.Join(protocols, ", "))
	}
	if len(args) > 1 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
		headers, ok := args[1].(*interpreter.MapValue)
		if !ok {
			conn.Close()
			return interpreter.NewError(400, "headers must be a map")
		}
		for _, name := range headers.Keys() {
			switch strings.ToLower(name) {
			case "upgrade", "connection", "sec-websocket-key", "sec-websocket-version", "sec-websocket-protocol", "sec-websocket-extensions":
				conn.Close()
				return interpreter.NewError(400, "WebSocket protocol headers cannot be overridden")
			}
			value, _ := headers.Get(name)
			req.Header.Set(name, interpreter.CoerceToString(value))
		}
	}
	if err := req.Write(conn); err != nil {
		conn.Close()
		return interpreter.NewError(500, "failed to write WebSocket handshake: "+err.Error())
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		conn.Close()
		return interpreter.NewError(500, "WebSocket handshake failed: "+err.Error())
	}
	fail := func(message string) interpreter.Value {
		conn.Close()
		return interpreter.NewError(500, message)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		return fail(fmt.Sprintf("WebSocket handshake failed: expected status 101, got %d", resp.StatusCode))
	}
	if !headerHasToken(resp.Header, "Upgrade", "websocket") || !headerHasToken(resp.Header, "Connection", "upgrade") {
		return fail("WebSocket handshake failed: invalid Upgrade or Connection header")
	}
	acceptHeaders := resp.Header.Values("Sec-WebSocket-Accept")
	if len(acceptHeaders) != 1 || acceptHeaders[0] != computeWebSocketAccept(key) {
		return fail("WebSocket handshake failed: invalid Sec-WebSocket-Accept header")
	}
	if resp.Header.Get("Sec-WebSocket-Extensions") != "" {
		return fail("WebSocket handshake failed: server selected an unsupported extension")
	}
	selectedProtocols := resp.Header.Values("Sec-WebSocket-Protocol")
	if len(selectedProtocols) > 1 {
		return fail("WebSocket handshake failed: server selected multiple subprotocols")
	}
	selectedProtocol := resp.Header.Get("Sec-WebSocket-Protocol")
	if selectedProtocol != "" && !isWebSocketToken(selectedProtocol) {
		return fail("WebSocket handshake failed: server selected an invalid subprotocol")
	}
	if selectedProtocol != "" && !containsString(protocols, selectedProtocol) {
		return fail("WebSocket handshake failed: server selected an unrequested protocol")
	}
	_ = conn.SetDeadline(time.Time{})
	ws := &WebSocketConnection{conn: conn, reader: reader}
	id := registerWebSocketConnection(ws)
	result := interpreter.NewMap()
	result.Set("id", &interpreter.StringValue{Val: id})
	result.Set("url", &interpreter.StringValue{Val: urlStr})
	result.Set("status", &interpreter.IntegerValue{Val: http.StatusSwitchingProtocols})
	result.Set("protocol", &interpreter.StringValue{Val: selectedProtocol})
	return result
}

func headerHasToken(header http.Header, name, token string) bool {
	for _, value := range header.Values(name) {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func isWebSocketToken(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			continue
		}
		if !strings.ContainsRune("!#$%&'*+-.^_`|~", ch) {
			return false
		}
	}
	return true
}

func computeWebSocketAccept(key string) string {
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	sum := sha1.Sum([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func websocket_send(args ...interpreter.Value) interpreter.Value {
	if len(args) < 2 || len(args) > 3 {
		return interpreter.NewError(400, "urllib2.websocket_send requires (conn_id, data, opcode?)")
	}
	id := interpreter.CoerceToString(args[0])
	ws, ok := getWebSocketConnection(id)
	if !ok {
		return interpreter.NewError(404, "WebSocket connection not found")
	}
	opcode := byte(1)
	var payload []byte
	switch value := args[1].(type) {
	case *interpreter.StringValue:
		payload = []byte(value.Val)
	case *interpreter.BlobValue:
		opcode = 2
		payload = value.Data
	default:
		return interpreter.NewError(400, "WebSocket data must be a string or blob")
	}
	if len(args) == 3 {
		switch value := args[2].(type) {
		case *interpreter.IntegerValue:
			if value.Val != 1 && value.Val != 2 {
				return interpreter.NewError(400, "websocket_send only accepts text or binary messages")
			}
			opcode = byte(value.Val)
		case *interpreter.StringValue:
			switch strings.ToLower(value.Val) {
			case "text":
				opcode = 1
			case "binary":
				opcode = 2
			default:
				return interpreter.NewError(400, "opcode must be text or binary")
			}
		default:
			return interpreter.NewError(400, "opcode must be text, binary, 1, or 2")
		}
	}
	if opcode != 1 && opcode != 2 {
		return interpreter.NewError(400, "websocket_send only accepts text or binary messages")
	}
	if opcode == 1 && !utf8.Valid(payload) {
		return interpreter.NewError(400, "text message must be valid UTF-8")
	}
	if err := ws.writeFrame(opcode, payload); err != nil {
		return interpreter.NewError(500, "WebSocket send failed: "+err.Error())
	}
	return interpreter.TRUE
}

func websocket_receive(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 2 {
		return interpreter.NewError(400, "urllib2.websocket_receive requires (conn_id, timeout?)")
	}
	id := interpreter.CoerceToString(args[0])
	ws, ok := getWebSocketConnection(id)
	if !ok {
		return interpreter.NewError(404, "WebSocket connection not found")
	}
	ws.readMu.Lock()
	defer ws.readMu.Unlock()
	if len(args) == 2 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
		timeout, err := websocketConnectTimeout(args[1])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		_ = ws.conn.SetReadDeadline(time.Now().Add(timeout))
		defer ws.conn.SetReadDeadline(time.Time{})
	}
	opcode, payload, err := ws.readMessage()
	if err != nil {
		if errors.Is(err, io.EOF) {
			removeWebSocketConnection(id)
			return interpreter.NewError(400, "WebSocket peer closed the connection")
		}
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			_ = ws.close()
			removeWebSocketConnection(id)
		}
		return interpreter.NewError(500, "WebSocket receive failed: "+err.Error())
	}
	if opcode == 1 {
		return &interpreter.StringValue{Val: string(payload)}
	}
	return &interpreter.BlobValue{Data: payload}
}

func websocket_ping(args ...interpreter.Value) interpreter.Value {
	return websocketControlFrame("ping", 9, args...)
}

func websocket_pong(args ...interpreter.Value) interpreter.Value {
	return websocketControlFrame("pong", 10, args...)
}

func websocketControlFrame(name string, opcode byte, args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 2 {
		return interpreter.NewError(400, "urllib2.websocket_"+name+" requires (conn_id, data?)")
	}
	ws, ok := getWebSocketConnection(interpreter.CoerceToString(args[0]))
	if !ok {
		return interpreter.NewError(404, "WebSocket connection not found")
	}
	var payload []byte
	if len(args) == 2 && args[1] != nil && args[1].Type() != interpreter.NULL_VAL {
		switch value := args[1].(type) {
		case *interpreter.StringValue:
			payload = []byte(value.Val)
		case *interpreter.BlobValue:
			payload = value.Data
		default:
			return interpreter.NewError(400, "control frame data must be a string or blob")
		}
	}
	if len(payload) > 125 {
		return interpreter.NewError(400, "control frame data cannot exceed 125 bytes")
	}
	if err := ws.writeFrame(opcode, payload); err != nil {
		return interpreter.NewError(500, "WebSocket "+name+" failed: "+err.Error())
	}
	return interpreter.TRUE
}

func websocket_close_impl(args ...interpreter.Value) interpreter.Value {
	if len(args) < 1 || len(args) > 3 {
		return interpreter.NewError(400, "urllib2.websocket_close requires (conn_id, code?, reason?)")
	}
	id := interpreter.CoerceToString(args[0])
	ws, ok := getWebSocketConnection(id)
	if !ok {
		return interpreter.NewError(404, "WebSocket connection not found")
	}
	code := 1000
	reason := ""
	if len(args) > 1 {
		value, ok := args[1].(*interpreter.IntegerValue)
		if !ok || value.Val < 0 || value.Val > 4999 {
			return interpreter.NewError(400, "close code must be an integer")
		}
		code = int(value.Val)
	}
	if len(args) > 2 {
		reason = interpreter.CoerceToString(args[2])
	}
	if !validWebSocketCloseCode(code) || len(reason) > 123 || !utf8.ValidString(reason) {
		return interpreter.NewError(400, "invalid WebSocket close code or reason")
	}
	payload := make([]byte, 2, 2+len(reason))
	binary.BigEndian.PutUint16(payload, uint16(code))
	payload = append(payload, reason...)
	if err := ws.writeFrame(8, payload); err != nil {
		return interpreter.NewError(500, "WebSocket close failed: "+err.Error())
	}
	if err := ws.close(); err != nil {
		return interpreter.NewError(500, "WebSocket close failed: "+err.Error())
	}
	removeWebSocketConnection(id)
	return interpreter.TRUE
}

func validWebSocketCloseCode(code int) bool {
	return (code >= 1000 && code <= 1014 && code != 1004 && code != 1005 && code != 1006) || (code >= 3000 && code <= 4999)
}
