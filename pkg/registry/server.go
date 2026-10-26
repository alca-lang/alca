// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alca-lang/alca/pkg/apm"
)

// Token authorizes publishing. Tokens are configured by the node operator and
// exchanged for author identities.
type Token struct {
	Token  string   `json:"token"`
	Author string   `json:"author"`
	Scopes []string `json:"scopes"`
}

// Config is the registry node configuration.
type Config struct {
	// Listen is the TCP address, e.g. "127.0.0.1:8787".
	Listen string `json:"listen"`
	// DataDir is the storage root.
	DataDir string `json:"data_dir"`
	// Tokens authorize publishing. They are node-level credentials configured
	// by the operator; an empty list disables them (accounts may still publish
	// when AllowRegistration is set).
	Tokens []Token `json:"tokens"`
	// AllowRegistration enables self-service accounts: users register with a
	// username, email and password and receive one-time tokens to publish with.
	AllowRegistration bool `json:"allow_registration"`
	// MaxUploadBytes caps a single publish or artifact upload. Default 64 MiB.
	MaxUploadBytes int64 `json:"max_upload_bytes"`
	// TLSCert/TLSKey enable direct TLS termination. Leave empty behind a
	// reverse proxy that terminates TLS.
	TLSCert string `json:"tls_cert"`
	TLSKey  string `json:"tls_key"`
	// ReadOnly serves downloads but refuses all writes.
	ReadOnly bool `json:"read_only"`
	// LogLevel is "debug", "info", "warn" or "error".
	LogLevel string `json:"log_level"`
}

// DefaultConfig returns the configuration defaults.
func DefaultConfig() Config {
	return Config{
		Listen:         "127.0.0.1:8787",
		DataDir:        "data",
		MaxUploadBytes: apm.MaxPackageBytes / 2,
		LogLevel:       "info",
	}
}

// LoadConfig reads a JSON configuration file over the defaults.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config %s is not valid JSON: %w", path, err)
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = apm.MaxPackageBytes / 2
	}
	return cfg, nil
}

// Server is an Alca registry node.
type Server struct {
	cfg      Config
	store    *Store
	accounts *AccountStore
	log      *slog.Logger
	mux      *http.ServeMux
}

type ctxKey int

const authorKey ctxKey = 1

// New creates a server, opening the storage directory.
func New(cfg Config, logger *slog.Logger) (*Server, error) {
	if cfg.Listen == "" {
		cfg.Listen = DefaultConfig().Listen
	}
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultConfig().DataDir
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = DefaultConfig().MaxUploadBytes
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}))
	}
	store, err := NewStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	accounts, err := NewAccountStore(filepath.Join(store.Root(), "accounts.json"))
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, store: store, accounts: accounts, log: logger, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

// Store exposes the node storage (used by tests and tooling).
func (s *Server) Store() *Store { return s.store }

// Accounts exposes the account store (used by tests and tooling).
func (s *Server) Accounts() *AccountStore { return s.accounts }

func parseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET "+apm.APIBasePath+"/packages", s.handleList)
	s.mux.HandleFunc("GET "+apm.APIBasePath+"/packages/{name}", s.handlePackage)
	s.mux.HandleFunc("GET "+apm.APIBasePath+"/packages/{name}/{version}", s.handleVersion)
	s.mux.HandleFunc("GET "+apm.APIBasePath+"/packages/{name}/{version}/download", s.handleDownload)
	s.mux.HandleFunc("GET "+apm.APIBasePath+"/packages/{name}/{version}/artifacts/{goos}/{goarch}", s.handleArtifactGet)
	s.mux.HandleFunc("POST "+apm.APIBasePath+"/register", s.handleRegister)
	s.mux.HandleFunc("POST "+apm.APIBasePath+"/login", s.handleLogin)
	s.mux.HandleFunc("GET "+apm.APIBasePath+"/account", s.withToken(s.handleAccount))
	s.mux.HandleFunc("POST "+apm.APIBasePath+"/logout", s.withToken(s.handleLogout))
	s.mux.HandleFunc("POST "+apm.APIBasePath+"/publish", s.withAuthor(s.handlePublish))
	s.mux.HandleFunc("POST "+apm.APIBasePath+"/packages/{name}/{version}/artifacts/{goos}/{goarch}", s.withAuthor(s.handleArtifactPost))
}

// Handler returns the HTTP handler, wrapped with request logging.
func (s *Server) Handler() http.Handler {
	return s.logRequests(s.mux)
}

// Run serves until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Listen,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("alca registry listening", "addr", s.cfg.Listen, "data", s.store.Root(),
			"tls", s.cfg.TLSCert != "", "read_only", s.cfg.ReadOnly, "registration", s.cfg.AllowRegistration,
			"accounts", s.accounts.Count(), "tokens", len(s.cfg.Tokens))
		if s.cfg.TLSCert != "" || s.cfg.TLSKey != "" {
			errCh <- srv.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			errCh <- srv.ListenAndServe()
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		} else if rec.status >= 400 {
			level = slog.LevelWarn
		}
		s.log.Log(r.Context(), level, "request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"bytes", rec.bytes, "duration", time.Since(start).Round(time.Millisecond).String(),
			"remote", r.RemoteAddr)
	})
}

type authorHandler func(http.ResponseWriter, *http.Request, principal)

// principal is a request identity: an account username or the author label of
// a node operator token. Operator tokens bypass package ownership.
type principal struct {
	Name     string
	Operator bool
}

// withToken authenticates a bearer token (a node operator token or an account
// token). It does not require a writable node, so it also guards reads such as
// GET /account.
func (s *Server) withToken(next authorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.authenticate(r)
		if err != nil {
			s.writeAuthError(w, err)
			return
		}
		next(w, r, p)
	}
}

// withAuthor authenticates a token and requires a writable (non read-only) node.
func (s *Server) withAuthor(next authorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.ReadOnly {
			s.writeError(w, http.StatusForbidden, "this registry node is read-only")
			return
		}
		if len(s.cfg.Tokens) == 0 && s.accounts.Count() == 0 {
			s.writeError(w, http.StatusForbidden, "publishing is disabled on this node (no accounts and no operator tokens configured)")
			return
		}
		s.withToken(next)(w, r)
	}
}

var errForbidden = errors.New("forbidden")

func (s *Server) writeAuthError(w http.ResponseWriter, err error) {
	status := http.StatusUnauthorized
	if errors.Is(err, errForbidden) {
		status = http.StatusForbidden
	}
	s.writeError(w, status, err.Error())
}

// authenticate resolves the Authorization header to a principal. Node operator
// tokens are checked first, then account tokens.
func (s *Server) authenticate(r *http.Request) (principal, error) {
	presented, err := bearerToken(r)
	if err != nil {
		return principal{}, err
	}
	for _, t := range s.cfg.Tokens {
		if subtle.ConstantTimeCompare([]byte(t.Token), []byte(presented)) == 1 {
			if !hasScope(t.Scopes, "publish") {
				return principal{}, fmt.Errorf("%w: token for %s lacks the publish scope", errForbidden, t.Author)
			}
			return principal{Name: t.Author, Operator: true}, nil
		}
	}
	if acct, tok, err := s.accounts.FindToken(presented); err == nil {
		if err := s.accounts.TouchToken(acct.Username, tok.ID); err != nil {
			s.log.Warn("cannot record token use", "user", acct.Username, "error", err)
		}
		return principal{Name: acct.Username}, nil
	}
	return principal{}, fmt.Errorf("invalid token")
}

// bearerToken extracts the bearer token from a request.
func bearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", fmt.Errorf("missing bearer token")
	}
	presented := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if presented == "" {
		return "", fmt.Errorf("missing bearer token")
	}
	return presented, nil
}

func hasScope(scopes []string, want string) bool {
	for _, sc := range scopes {
		if sc == want || sc == "*" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "ok\n")
}

// handleRegister creates an account and returns its first token, shown once.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		s.writeError(w, http.StatusForbidden, "this registry node is read-only")
		return
	}
	if !s.cfg.AllowRegistration {
		s.writeError(w, http.StatusForbidden, ErrRegistrationClosed.Error())
		return
	}
	var req apm.RegisterRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	acct, token, err := s.accounts.Register(req.Username, req.Email, req.Password)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, ErrUsernameTaken), errors.Is(err, ErrEmailTaken):
			status = http.StatusConflict
		case errors.Is(err, ErrInvalidInput):
			status = http.StatusBadRequest
		}
		s.log.Warn("registration rejected", "user", strings.ToLower(strings.TrimSpace(req.Username)), "reason", err.Error())
		s.writeError(w, status, err.Error())
		return
	}
	s.log.Info("register", "user", acct.Username, "email", acct.Email, "remote", r.RemoteAddr)
	s.writeTokenResponse(w, http.StatusCreated, acct, token)
}

// handleLogin exchanges a username and password for a new one-time token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.ReadOnly {
		s.writeError(w, http.StatusForbidden, "this registry node is read-only")
		return
	}
	var req apm.LoginRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	acct, err := s.accounts.Authenticate(req.Username, req.Password)
	if err != nil {
		s.log.Warn("login rejected", "user", strings.ToLower(strings.TrimSpace(req.Username)), "remote", r.RemoteAddr)
		s.writeError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		return
	}
	token, _, err := s.accounts.IssueToken(acct.Username, "login")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.log.Info("login", "user", acct.Username, "remote", r.RemoteAddr)
	s.writeTokenResponse(w, http.StatusOK, acct, token)
}

// handleAccount reports the signed-in identity and its tokens; tokens are
// listed by prefix only, since the plaintext is never stored.
func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request, p principal) {
	if p.Operator {
		s.writeJSON(w, http.StatusOK, apm.AccountResponse{Username: p.Name, Operator: true})
		return
	}
	acct, err := s.accounts.Snapshot(p.Name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}
	resp := apm.AccountResponse{Username: acct.Username, Email: acct.Email, CreatedAt: acct.CreatedAt}
	for _, t := range acct.Tokens {
		resp.Tokens = append(resp.Tokens, apm.TokenInfo{
			ID: t.ID, Prefix: t.Prefix, Label: t.Label, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt,
		})
	}
	s.writeJSON(w, http.StatusOK, resp)
}

// handleLogout revokes the token the request authenticated with.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, p principal) {
	if p.Operator {
		s.writeError(w, http.StatusBadRequest, "node operator tokens live in the server configuration file and cannot be revoked remotely")
		return
	}
	presented, err := bearerToken(r)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if _, err := s.accounts.RevokeToken(presented); err != nil {
		s.writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	s.log.Info("logout", "user", p.Name, "remote", r.RemoteAddr)
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// writeTokenResponse returns a one-time token. The response is marked no-store
// so intermediaries never cache a credential.
func (s *Server) writeTokenResponse(w http.ResponseWriter, status int, acct *Account, token string) {
	w.Header().Set("Cache-Control", "no-store")
	s.writeJSON(w, status, apm.AuthResponse{
		Username:  acct.Username,
		Email:     acct.Email,
		CreatedAt: acct.CreatedAt,
		Token:     token,
		Message:   "Store this token now: it is not shown again.",
	})
}

// decodeJSONBody reads a small JSON request body.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, out any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
		return fmt.Errorf("expected a JSON request body, got Content-Type %q", ct)
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

// checkOwner enforces first-publisher package ownership: only the account that
// published the first version of a package (or a node operator token) may
// publish further versions of it.
func (s *Server) checkOwner(name string, who principal) error {
	if who.Operator {
		return nil
	}
	versions, err := s.store.Versions(name)
	if err != nil || len(versions) == 0 {
		return nil // a brand new package: the caller becomes its owner
	}
	owner := versions[0].Publisher
	if owner == "" || strings.EqualFold(owner, who.Name) {
		return nil
	}
	return fmt.Errorf("package %s is owned by %s; only that account can publish new versions", name, owner)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = min(n, 500)
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}
	packages, total, err := s.store.List(r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.writeJSON(w, http.StatusOK, apm.ListResponse{Packages: packages, Total: total})
}

func (s *Server) handlePackage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	versions, err := s.store.Versions(name)
	if err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("package %q not found", name))
		return
	}
	resp := apm.PackageResponse{Name: name}
	if len(versions) > 0 {
		resp.Description = versions[len(versions)-1].Description
	}
	// Newest first: easier for CLI display.
	for i := len(versions) - 1; i >= 0; i-- {
		resp.Versions = append(resp.Versions, versions[i])
	}
	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	info, err := s.store.Version(r.PathValue("name"), r.PathValue("version"))
	if err != nil {
		s.writeError(w, http.StatusNotFound, "version not found")
		return
	}
	s.writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	name, version := r.PathValue("name"), r.PathValue("version")
	data, info, err := s.store.PackageArchive(name, version)
	if err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("package %s %s not found", name, version))
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("X-Alca-Sha256", info.SHA256)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handleArtifactGet(w http.ResponseWriter, r *http.Request) {
	name, version := r.PathValue("name"), r.PathValue("version")
	goos, goarch := r.PathValue("goos"), r.PathValue("goarch")
	data, meta, err := s.store.ArtifactBin(name, version, goos, goarch)
	if err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("no %s/%s build for %s %s", goos, goarch, name, version))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Alca-Sha256", meta.SHA256)
	w.Header().Set("X-Alca-Alca-Version", meta.AlcaVersion)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request, who principal) {
	archive, err := readMultipartFile(w, r, "package", s.cfg.MaxUploadBytes)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	staging, err := s.store.TempDir("publish-*")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(staging)
	if err := apm.UnpackArchive(archive, staging); err != nil {
		s.writeError(w, http.StatusBadRequest, "package archive is invalid: "+err.Error())
		return
	}
	m, err := apm.CheckPackageStructure(staging)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "package validation failed: "+err.Error())
		return
	}
	if err := s.checkOwner(m.Package.Name, who); err != nil {
		s.log.Warn("publish rejected", "package", m.Package.Name, "author", who.Name, "reason", err.Error())
		s.writeError(w, http.StatusForbidden, err.Error())
		return
	}
	info := apm.VersionInfo{
		Name:         m.Package.Name,
		Version:      m.Package.Version,
		Description:  m.Package.Description,
		Entry:        m.Package.Entry,
		Alca:         m.Package.Alca,
		License:      m.Package.License,
		Authors:      m.Package.Authors,
		Repository:   m.Package.Repository,
		Dependencies: m.Dependencies,
		PublishedAt:  time.Now().UTC().Format(time.RFC3339),
		Publisher:    who.Name,
	}
	already, err := s.store.SavePackage(info, archive)
	switch {
	case errors.Is(err, ErrVersionExists):
		s.log.Warn("publish rejected", "package", info.Name, "version", info.Version, "author", who.Name, "reason", "version exists")
		s.writeError(w, http.StatusConflict, fmt.Sprintf("%s %s is already published with different content", info.Name, info.Version))
		return
	case err != nil:
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusCreated
	label := "published"
	if already {
		status = http.StatusOK
		label = "already-published"
	}
	s.log.Info("publish", "package", info.Name, "version", info.Version, "author", who.Name, "status", label, "bytes", len(archive))
	s.writeJSON(w, status, apm.PublishResponse{Name: info.Name, Version: info.Version, SHA256: info.SHA256, Size: info.Size, Status: label})
}

func (s *Server) handleArtifactPost(w http.ResponseWriter, r *http.Request, who principal) {
	name, version := r.PathValue("name"), r.PathValue("version")
	goos, goarch := r.PathValue("goos"), r.PathValue("goarch")
	if _, err := s.store.Version(name, version); err != nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("publish %s %s before uploading builds for it", name, version))
		return
	}
	if err := s.checkOwner(name, who); err != nil {
		s.writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}
	metaText := r.FormValue("meta")
	if metaText == "" {
		s.writeError(w, http.StatusBadRequest, "missing artifact metadata field \"meta\"")
		return
	}
	var meta apm.ArtifactMeta
	if err := json.Unmarshal([]byte(metaText), &meta); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid artifact metadata: "+err.Error())
		return
	}
	file, _, err := r.FormFile("artifact")
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "missing artifact file field \"artifact\"")
		return
	}
	defer file.Close()
	bin, err := io.ReadAll(io.LimitReader(file, apm.MaxFileBytes+1))
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "cannot read artifact: "+err.Error())
		return
	}
	if len(bin) > apm.MaxFileBytes {
		s.writeError(w, http.StatusRequestEntityTooLarge, "artifact exceeds the size limit")
		return
	}
	if err := validateArtifactMeta(meta, name, version, goos, goarch, bin); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	already, err := s.store.SaveArtifact(name, version, goos, goarch, bin, meta)
	switch {
	case errors.Is(err, ErrArtifactExists):
		s.writeError(w, http.StatusConflict, fmt.Sprintf("a different %s/%s build is already published for %s %s", goos, goarch, name, version))
		return
	case errors.Is(err, ErrNotFound):
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("%s %s is not published", name, version))
		return
	case err != nil:
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := http.StatusCreated
	label := "uploaded"
	if already {
		status = http.StatusOK
		label = "already-uploaded"
	}
	s.log.Info("artifact", "package", name, "version", version, "goos", goos, "goarch", goarch,
		"author", who.Name, "status", label, "bytes", len(bin))
	s.writeJSON(w, status, map[string]string{"status": label, "goos": goos, "goarch": goarch, "sha256": sha256HexBytes(bin)})
}

// validateArtifactMeta checks that uploaded metadata is coherent with the URL,
// the binary and this toolchain's artifact format.
func validateArtifactMeta(meta apm.ArtifactMeta, name, version, goos, goarch string, bin []byte) error {
	if meta.Format != apm.ArtifactFormat || meta.FormatVersion != apm.ArtifactFormatVersion {
		return fmt.Errorf("unsupported artifact format %q v%d", meta.Format, meta.FormatVersion)
	}
	if meta.Name != name || meta.Version != version {
		return fmt.Errorf("artifact metadata names %s %s, expected %s %s", meta.Name, meta.Version, name, version)
	}
	if meta.GOOS != goos || meta.GOARCH != goarch {
		return fmt.Errorf("artifact metadata targets %s/%s, expected %s/%s", meta.GOOS, meta.GOARCH, goos, goarch)
	}
	if _, err := apm.ParseVersion(meta.AlcaVersion); err != nil {
		return fmt.Errorf("artifact metadata has an invalid alca_version %q", meta.AlcaVersion)
	}
	if meta.PayloadVersion == 0 {
		return fmt.Errorf("artifact metadata is missing payload_version")
	}
	if meta.SHA256 == "" {
		return fmt.Errorf("artifact metadata is missing sha256")
	}
	if sha256HexBytes(bin) != meta.SHA256 {
		return fmt.Errorf("artifact checksum does not match its metadata")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, status int, msg string) {
	s.writeJSON(w, status, apm.ErrorResponse{Error: msg})
}

// readMultipartFile reads one file field from a multipart body with a hard size
// cap enforced before buffering.
func readMultipartFile(w http.ResponseWriter, r *http.Request, field string, maxBytes int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("request is not a valid multipart form: %w", err)
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cannot read multipart form: %w", err)
		}
		if part.FormName() != field {
			io.Copy(io.Discard, io.LimitReader(part, 1<<20))
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, maxBytes+1))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", field, err)
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("%s exceeds the upload limit (%d bytes)", field, maxBytes)
		}
		return data, nil
	}
	return nil, fmt.Errorf("missing multipart file field %q", field)
}

// ListenAddress returns the effective listen address for a config, resolving
// the port so operators can use ":0" in tests.
func (c Config) ListenAddress() string { return c.Listen }

// AddrHostPort splits the listen address, defaulting the host to 127.0.0.1.
func AddrHostPort(addr string) (string, string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return host, port
}
