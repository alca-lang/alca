// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

// This file defines the JSON wire format shared by the APM client and Alca
// registry nodes. It is versioned through URL paths (/api/v1/...) and never
// carries executable payloads: packages travel as tar.gz archives and compiled
// builds as opaque .bin files with their ArtifactMeta sidecar.

// APIBasePath is the URL prefix of the registry API.
const APIBasePath = "/api/v1"

// ArtifactInfo describes one compiled build of a published version.
type ArtifactInfo struct {
	GOOS           string         `json:"goos"`
	GOARCH         string         `json:"goarch"`
	AlcaVersion    string         `json:"alca_version"`
	PayloadVersion uint32         `json:"payload_version"`
	SHA256         string         `json:"sha256"`
	Size           int64          `json:"size,omitempty"`
	Entry          string         `json:"entry,omitempty"`
	SourceHash     string         `json:"source_hash,omitempty"`
	Exports        map[string]int `json:"exports,omitempty"`
	CreatedAt      string         `json:"created_at,omitempty"`
}

// VersionInfo is the registry metadata of one published package version.
type VersionInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description,omitempty"`
	Entry        string            `json:"entry"`
	Alca         string            `json:"alca,omitempty"`
	License      string            `json:"license,omitempty"`
	Authors      []string          `json:"authors,omitempty"`
	Repository   string            `json:"repository,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	PublishedAt  string            `json:"published_at,omitempty"`
	Publisher    string            `json:"publisher,omitempty"`
	SHA256       string            `json:"sha256"`
	Size         int64             `json:"size,omitempty"`
	Artifacts    []ArtifactInfo    `json:"artifacts,omitempty"`
	Yanked       bool              `json:"yanked,omitempty"`
}

// PackageSummary is one entry of a package listing.
type PackageSummary struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Latest      string `json:"latest,omitempty"`
	Versions    int    `json:"versions"`
	Publisher   string `json:"publisher,omitempty"`
	Yanked      bool   `json:"yanked,omitempty"`
}

// ListResponse is the response of GET /api/v1/packages.
type ListResponse struct {
	Packages []PackageSummary `json:"packages"`
	Total    int              `json:"total"`
}

// PackageResponse is the response of GET /api/v1/packages/{name}.
type PackageResponse struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Versions    []VersionInfo `json:"versions"`
}

// PublishResponse is the response of a successful publish.
type PublishResponse struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size,omitempty"`
	Status  string `json:"status,omitempty"` // "published" or "already-published"
}

// ErrorResponse is the JSON error returned by a registry node.
type ErrorResponse struct {
	Error string `json:"error"`
}

// RegisterRequest is the body of POST /api/v1/register.
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest is the body of POST /api/v1/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse is returned by register and login. Token is a one-time secret:
// nodes store only its hash, so it is shown exactly once.
type AuthResponse struct {
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	Token     string `json:"token"`
	Message   string `json:"message,omitempty"`
}

// TokenInfo describes one issued token without revealing it.
type TokenInfo struct {
	ID         string `json:"id"`
	Prefix     string `json:"prefix"`
	Label      string `json:"label,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

// AccountResponse is returned by GET /api/v1/account.
type AccountResponse struct {
	Username  string      `json:"username"`
	Email     string      `json:"email,omitempty"`
	CreatedAt string      `json:"created_at,omitempty"`
	Operator  bool        `json:"operator,omitempty"`
	Tokens    []TokenInfo `json:"tokens,omitempty"`
}
