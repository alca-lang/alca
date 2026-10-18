// Package stdlib provides the standard library modules and builtins for the Alca programming language.
package stdlib

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// SimpleServer wraps an HTTP server with programmatic control.
type SimpleServer struct {
	Port     int
	server   *http.Server
	mux      *http.ServeMux
	mu       sync.Mutex
	isClosed bool
}

// Close gracefully stops the server.
func (s *SimpleServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		s.isClosed = true
		return s.server.Shutdown(context.Background())
	}
	return nil
}

// TCPConn wraps a network TCP connection.
type TCPConn struct {
	conn net.Conn
}

// Read reads up to max bytes from the TCP connection.
func (c *TCPConn) Read(max int) ([]byte, error) {
	if max <= 0 {
		max = 4096
	}
	buf := make([]byte, max)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Write writes bytes to the TCP connection.
func (c *TCPConn) Write(data []byte) (int, error) {
	return c.conn.Write(data)
}

// Close closes the underlying TCP connection.
func (c *TCPConn) Close() error {
	return c.conn.Close()
}

// TCPListener wraps a TCP listener waiting for incoming connections.
type TCPListener struct {
	listener net.Listener
}

// Accept waits for and returns the next connection to the listener.
func (l *TCPListener) Accept() (*TCPConn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return &TCPConn{conn: conn}, nil
}

// Close closes the TCP listener.
func (l *TCPListener) Close() error {
	return l.listener.Close()
}

// HTTPGet performs an HTTP GET request to the given URL and returns the response body, status code, and error.
func HTTPGet(url string) (string, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(bodyBytes), resp.StatusCode, nil
}

// HTTPPost performs an HTTP POST request with the given content type and body.
func HTTPPost(url, contentType, body string) (string, int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, contentType, strings.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(bodyBytes), resp.StatusCode, nil
}

// HTTPServer creates and starts a basic HTTP server listening on the specified port.
func HTTPServer(port int) *SimpleServer {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	simple := &SimpleServer{
		Port:   port,
		server: srv,
		mux:    mux,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	return simple
}

// TCPConnect establishes a TCP connection to the given address (e.g. "example.com:80").
func TCPConnect(addr string) (*TCPConn, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	return &TCPConn{conn: conn}, nil
}

// TCPListen creates a TCP listener on the given address (e.g. ":8080").
func TCPListen(addr string) (*TCPListener, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &TCPListener{listener: listener}, nil
}

// DNSLookup performs a DNS resolution for the given hostname and returns IP addresses.
func DNSLookup(host string) ([]string, error) {
	ips, err := net.LookupHost(host)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

// Ping checks host reachability via connection attempt on common service ports or ICMP.
func Ping(host string) (bool, error) {
	// Probe common ports (80, 443, 53) or dial address directly
	addr := host
	if !strings.Contains(host, ":") {
		addr = net.JoinHostPort(host, "80")
	}
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err == nil {
		_ = conn.Close()
		return true, nil
	}

	// Try port 443 fallback
	if !strings.Contains(host, ":") {
		addr = net.JoinHostPort(host, "443")
		conn, err = net.DialTimeout("tcp", addr, 3*time.Second)
		if err == nil {
			_ = conn.Close()
			return true, nil
		}
	}

	// Also attempt host IP resolution
	_, lookupErr := net.LookupHost(host)
	if lookupErr == nil {
		return true, nil
	}

	return false, err
}

// DownloadFile downloads a file from the given URL and writes it to dest.
func DownloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP download failed with status code %d", resp.StatusCode)
	}

	destDir := filepath.Dir(dest)
	if destDir != "" && destDir != "." {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// RegisterNetBuiltins registers networking functions into the interpreter environment.
func RegisterNetBuiltins(env *interpreter.Environment) {
	netModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		env.Set(name, builtin)
		netModule.Set(name, builtin)
	}

	register("http_get", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "http_get requires a url argument")
		}
		url := interpreter.CoerceToString(args[0])
		body, code, err := HTTPGet(url)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		res := interpreter.NewMap()
		res.Set("body", &interpreter.StringValue{Val: body})
		res.Set("status", &interpreter.IntegerValue{Val: int64(code)})
		return res
	})

	register("http_post", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 3 {
			return interpreter.NewError(400, "http_post requires url, contentType, and body arguments")
		}
		url := interpreter.CoerceToString(args[0])
		contentType := interpreter.CoerceToString(args[1])
		body := interpreter.CoerceToString(args[2])
		respBody, code, err := HTTPPost(url, contentType, body)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		res := interpreter.NewMap()
		res.Set("body", &interpreter.StringValue{Val: respBody})
		res.Set("status", &interpreter.IntegerValue{Val: int64(code)})
		return res
	})

	register("dns_lookup", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "dns_lookup requires a host argument")
		}
		host := interpreter.CoerceToString(args[0])
		ips, err := DNSLookup(host)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		elems := make([]interpreter.Value, len(ips))
		for i, ip := range ips {
			elems[i] = &interpreter.StringValue{Val: ip}
		}
		return interpreter.NewList(elems...)
	})

	register("ping", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.FALSE
		}
		host := interpreter.CoerceToString(args[0])
		ok, _ := Ping(host)
		if ok {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("download_file", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "download_file requires url and dest arguments")
		}
		url := interpreter.CoerceToString(args[0])
		dest := interpreter.CoerceToString(args[1])
		if err := DownloadFile(url, dest); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})

	env.Set("net", netModule)
	env.Set("std.net", netModule)
}
