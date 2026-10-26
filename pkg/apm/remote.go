// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alca-lang/alca/pkg/version"
)

// DefaultServerTimeout is the default timeout for connecting/accessing the registry server.
const DefaultServerTimeout = 15 * time.Second

var ipOrURLRegex = regexp.MustCompile(`https?://[^\s"'<>]+|\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b`)

// StripServerInfo removes any IP addresses and HTTP URLs from the string.
func StripServerInfo(s string) string {
	return ipOrURLRegex.ReplaceAllString(s, "registry")
}

func cleanRegistryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) || strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "deadline") {
		return errors.New("registry connection timed out (15s)")
	}
	msg := err.Error()
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "connectex") || strings.Contains(msg, "no such host") || strings.Contains(msg, "actively refused") {
		return errors.New("registry unreachable")
	}
	return errors.New(StripServerInfo(msg))
}

// HTTPDoer is the subset of *http.Client used by the registry client.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientOptions configure a registry client.
type ClientOptions struct {
	// Token authenticates publish operations (Bearer token).
	Token string
	// InsecureTLS disables certificate verification (development only).
	InsecureTLS bool
	// HTTPClient replaces the default HTTP client (tests, proxies).
	HTTPClient HTTPDoer
	// Retries is the number of retries for transient failures. Defaults to 1.
	Retries int
	// UserAgent overrides the default user agent.
	UserAgent string
	// Timeout overrides the default 15s server access timeout.
	Timeout time.Duration
}

// Client talks to an Alca registry node.
type Client struct {
	base    string
	http    HTTPDoer
	token   string
	retries int
	ua      string
	timeout time.Duration
}

// NewClient creates a registry client for baseURL.
func NewClient(baseURL string, opts ClientOptions) *Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultServerTimeout
	}
	doer := opts.HTTPClient
	if doer == nil {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
		}
		if opts.InsecureTLS {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- explicit development opt-in
		}
		doer = &http.Client{Timeout: timeout, Transport: transport}
	}
	retries := opts.Retries
	if retries <= 0 {
		retries = 1
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = "alca-apm/" + version.Version
	}
	return &Client{base: base, http: doer, token: opts.Token, retries: retries, ua: ua, timeout: timeout}
}

// BaseURL returns the registry URL of this client.
func (c *Client) BaseURL() string { return c.base }

func (c *Client) do(method, path string, body func() (io.Reader, string, error), auth bool) (*http.Response, error) {
	if c.base == "" {
		return nil, fmt.Errorf("registry URL is empty")
	}
	endpoint := c.base + path
	timeout := c.timeout
	if timeout <= 0 {
		timeout = DefaultServerTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if ctx.Err() != nil {
			return nil, cleanRegistryError(ctx.Err())
		}
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(200*(1<<uint(attempt-1))) * time.Millisecond):
			case <-ctx.Done():
				return nil, cleanRegistryError(ctx.Err())
			}
		}
		var reader io.Reader
		contentType := ""
		if body != nil {
			var err error
			reader, contentType, err = body()
			if err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
		if err != nil {
			return nil, cleanRegistryError(err)
		}
		req.Header.Set("User-Agent", c.ua)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if auth && c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, cleanRegistryError(ctx.Err())
			}
			continue // network error: retry
		}
		if resp.StatusCode >= 500 {
			lastErr = readHTTPError(resp)
			continue // server error: retry
		}
		return resp, nil
	}
	return nil, cleanRegistryError(lastErr)
}

func readHTTPError(resp *http.Response) error {
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var er ErrorResponse
	if json.Unmarshal(data, &er) == nil && er.Error != "" {
		return fmt.Errorf("registry error %d: %s", resp.StatusCode, er.Error)
	}
	msg := strings.TrimSpace(string(data))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("registry error %d: %s", resp.StatusCode, msg)
}

func decodeJSON(resp *http.Response, out any) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readHTTPError(resp)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 64<<20))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("registry returned an invalid response: %w", err)
	}
	return nil
}

// List returns published packages, optionally filtered by a search query.
func (c *Client) List(query string, limit, offset int) (*ListResponse, error) {
	q := url.Values{}
	if query != "" {
		q.Set("q", query)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	path := APIBasePath + "/packages"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	resp, err := c.do(http.MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}
	var out ListResponse
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PackageExists checks whether a package exists in the registry.
// Returns (true, versions, nil) if package exists and has versions.
// Returns (false, nil, nil) if package definitely does not exist (404 / no versions).
// Returns (false, nil, err) if registry could not be reached or timed out.
func (c *Client) PackageExists(name string) (bool, []VersionInfo, error) {
	resp, err := c.do(http.MethodGet, APIBasePath+"/packages/"+url.PathEscape(name), nil, false)
	if err != nil {
		return false, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, nil, readHTTPError(resp)
	}
	var out PackageResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, 64<<20))
	if err := dec.Decode(&out); err != nil {
		return false, nil, fmt.Errorf("registry returned an invalid response: %w", err)
	}
	if len(out.Versions) == 0 {
		return false, nil, nil
	}
	return true, out.Versions, nil
}

// Versions returns every published version of a package, oldest first.
func (c *Client) Versions(name string) ([]VersionInfo, error) {
	resp, err := c.do(http.MethodGet, APIBasePath+"/packages/"+url.PathEscape(name), nil, false)
	if err != nil {
		return nil, err
	}
	var out PackageResponse
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return out.Versions, nil
}

// Version returns the metadata of one published package version.
func (c *Client) Version(name, ver string) (*VersionInfo, error) {
	resp, err := c.do(http.MethodGet, APIBasePath+"/packages/"+url.PathEscape(name)+"/"+url.PathEscape(ver), nil, false)
	if err != nil {
		return nil, err
	}
	var out VersionInfo
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProgressFunc is called periodically during a download.
type ProgressFunc func(downloaded, total int64, speed float64)

// DownloadWithProgress fetches a package archive with progress reporting and checksum verification.
func (c *Client) DownloadWithProgress(name, ver, expected string, onProgress ProgressFunc) ([]byte, error) {
	resp, err := c.do(http.MethodGet, APIBasePath+"/packages/"+url.PathEscape(name)+"/"+url.PathEscape(ver)+"/download", nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readHTTPError(resp)
	}
	total := resp.ContentLength
	start := time.Now()
	var downloaded int64
	var buf bytes.Buffer
	chunk := make([]byte, 32*1024)
	lastUpdate := time.Now()
	for {
		n, rerr := resp.Body.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			downloaded += int64(n)
			if downloaded > MaxPackageBytes {
				return nil, fmt.Errorf("package %s %s exceeds the size limit (%d bytes)", name, ver, MaxPackageBytes)
			}
			now := time.Now()
			elapsed := now.Sub(start).Seconds()
			var speed float64
			if elapsed > 0 {
				speed = float64(downloaded) / elapsed
			}
			if onProgress != nil && (now.Sub(lastUpdate) >= 50*time.Millisecond || rerr != nil) {
				onProgress(downloaded, total, speed)
				lastUpdate = now
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return nil, fmt.Errorf("download interrupted: %w", rerr)
		}
	}
	data := buf.Bytes()
	if expected != "" {
		if got := sha256Hex(data); got != expected {
			return nil, fmt.Errorf("checksum mismatch for %s %s: expected %s, got %s", name, ver, expected, got)
		}
	}
	return data, nil
}

// Download fetches a package archive and verifies its checksum when expected is non-empty.
func (c *Client) Download(name, ver, expected string) ([]byte, error) {
	return c.DownloadWithProgress(name, ver, expected, nil)
}

// DownloadArtifactWithProgress fetches a compiled build for a platform with progress reporting.
func (c *Client) DownloadArtifactWithProgress(name, ver, goos, goarch, expected string, onProgress ProgressFunc) ([]byte, error) {
	path := fmt.Sprintf("%s/packages/%s/%s/artifacts/%s/%s", APIBasePath, url.PathEscape(name), url.PathEscape(ver), url.PathEscape(goos), url.PathEscape(goarch))
	resp, err := c.do(http.MethodGet, path, nil, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readHTTPError(resp)
	}
	total := resp.ContentLength
	start := time.Now()
	var downloaded int64
	var buf bytes.Buffer
	chunk := make([]byte, 32*1024)
	lastUpdate := time.Now()
	for {
		n, rerr := resp.Body.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			downloaded += int64(n)
			if downloaded > MaxFileBytes {
				return nil, fmt.Errorf("artifact %s %s exceeds size limit (%d bytes)", name, ver, MaxFileBytes)
			}
			now := time.Now()
			elapsed := now.Sub(start).Seconds()
			var speed float64
			if elapsed > 0 {
				speed = float64(downloaded) / elapsed
			}
			if onProgress != nil && (now.Sub(lastUpdate) >= 50*time.Millisecond || rerr != nil) {
				onProgress(downloaded, total, speed)
				lastUpdate = now
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return nil, err
		}
	}
	data := buf.Bytes()
	if expected != "" {
		if got := sha256Hex(data); got != expected {
			return nil, fmt.Errorf("checksum mismatch for artifact %s %s %s/%s", name, ver, goos, goarch)
		}
	}
	return data, nil
}

// DownloadArtifact fetches a compiled build for a platform.
func (c *Client) DownloadArtifact(name, ver, goos, goarch, expected string) ([]byte, error) {
	return c.DownloadArtifactWithProgress(name, ver, goos, goarch, expected, nil)
}

// Publish uploads a package archive. The request is idempotent: re-uploading
// the same archive after a network failure succeeds if the server already
// stored it.
func (c *Client) Publish(archive []byte) (*PublishResponse, error) {
	resp, err := c.do(http.MethodPost, APIBasePath+"/publish", func() (io.Reader, string, error) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		part, err := mw.CreateFormFile("package", "package.tar.gz")
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(archive); err != nil {
			return nil, "", err
		}
		if err := mw.Close(); err != nil {
			return nil, "", err
		}
		return &buf, mw.FormDataContentType(), nil
	}, true)
	if err != nil {
		return nil, err
	}
	var out PublishResponse
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UploadArtifact publishes a compiled build for a platform. meta.SHA256 must
// match bin; the server verifies it again.
func (c *Client) UploadArtifact(name, ver, goos, goarch string, meta ArtifactMeta, bin []byte) error {
	if meta.SHA256 == "" {
		meta.SHA256 = sha256Hex(bin)
	}
	if got := sha256Hex(bin); got != meta.SHA256 {
		return fmt.Errorf("artifact checksum does not match its metadata")
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/packages/%s/%s/artifacts/%s/%s", APIBasePath, url.PathEscape(name), url.PathEscape(ver), url.PathEscape(goos), url.PathEscape(goarch))
	resp, err := c.do(http.MethodPost, path, func() (io.Reader, string, error) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if err := mw.WriteField("meta", string(metaJSON)); err != nil {
			return nil, "", err
		}
		part, err := mw.CreateFormFile("artifact", goos+"-"+goarch+".bin")
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(bin); err != nil {
			return nil, "", err
		}
		if err := mw.Close(); err != nil {
			return nil, "", err
		}
		return &buf, mw.FormDataContentType(), nil
	}, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readHTTPError(resp)
	}
	return nil
}
