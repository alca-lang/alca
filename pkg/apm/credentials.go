// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CredentialsFile is the credential store file inside the Alca home.
const CredentialsFile = "credentials.json"

// CredentialsFormatVersion identifies the credentials.json layout.
const CredentialsFormatVersion = 1

// Credential is a saved registry account.
type Credential struct {
	Username  string `json:"username"`
	Token     string `json:"token"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Credentials stores one account token per registry node. The file lives in
// the Alca home and is created with owner-only permissions.
type Credentials struct {
	path    string
	entries map[string]Credential
}

type credentialFile struct {
	FormatVersion int                   `json:"format_version"`
	Registries    map[string]Credential `json:"registries"`
}

// CredentialsPath returns the credentials file path for an Alca home.
func CredentialsPath(home string) (string, error) {
	if strings.TrimSpace(home) == "" {
		h, err := Home()
		if err != nil {
			return "", err
		}
		home = h
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, CredentialsFile), nil
}

// LoadCredentials reads the credentials file. A missing file is not an error.
func LoadCredentials(home string) (*Credentials, error) {
	path, err := CredentialsPath(home)
	if err != nil {
		return nil, err
	}
	c := &Credentials{path: path, entries: map[string]Credential{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var doc credentialFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s is corrupt: %w", path, err)
	}
	if doc.FormatVersion > CredentialsFormatVersion {
		return nil, fmt.Errorf("%s format %d is newer than this toolchain supports (%d)", path, doc.FormatVersion, CredentialsFormatVersion)
	}
	if doc.Registries != nil {
		c.entries = doc.Registries
	}
	return c, nil
}

// Path returns the file backing this store.
func (c *Credentials) Path() string { return c.path }

// Get returns the saved credential of a registry node.
func (c *Credentials) Get(registryURL string) (Credential, bool) {
	cred, ok := c.entries[NormalizeRegistryURL(registryURL)]
	return cred, ok
}

// Set stores a token for a registry node, creating the file when needed.
func (c *Credentials) Set(registryURL, username, token string) error {
	key := NormalizeRegistryURL(registryURL)
	if key == "" {
		return fmt.Errorf("cannot save credentials: empty registry URL")
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("cannot save credentials: empty token")
	}
	if c.entries == nil {
		c.entries = map[string]Credential{}
	}
	c.entries[key] = Credential{Username: username, Token: token, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	return c.save()
}

// Delete forgets the credential of a registry node.
func (c *Credentials) Delete(registryURL string) error {
	key := NormalizeRegistryURL(registryURL)
	if _, ok := c.entries[key]; !ok {
		return nil
	}
	delete(c.entries, key)
	return c.save()
}

func (c *Credentials) save() error {
	doc := credentialFile{FormatVersion: CredentialsFormatVersion, Registries: c.entries}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWriteFile(c.path, data, 0o600)
}

// NormalizeRegistryURL canonicalizes a registry URL for use as a key: the
// scheme and host are lowercased and a trailing slash is dropped.
func NormalizeRegistryURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}
