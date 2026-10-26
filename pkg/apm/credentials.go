// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package apm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CredentialsFile is the credential store file inside the Alca home.
const CredentialsFile = "credentials.json"

// MasterKeyFile is the machine-local encryption key for credentials.
const MasterKeyFile = ".credentials.key"

// CredentialsFormatVersion identifies the credentials.json layout (v2: AES-256-GCM encrypted).
const CredentialsFormatVersion = 2

// Credential is a saved registry account.
type Credential struct {
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
	Token     string `json:"token"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Credentials stores account credentials (username, password, token) per registry node.
// All sensitive fields on disk are encrypted using AES-256-GCM with a machine-local key.
type Credentials struct {
	home    string
	path    string
	entries map[string]Credential
}

type credentialFile struct {
	FormatVersion int                   `json:"format_version"`
	Encrypted     bool                  `json:"encrypted,omitempty"`
	Nonce         string                `json:"nonce,omitempty"`
	Ciphertext    string                `json:"ciphertext,omitempty"`
	Registries    map[string]Credential `json:"registries,omitempty"`
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

func getMasterKey(home string) ([]byte, error) {
	keyPath := filepath.Join(home, MasterKeyFile)
	data, err := os.ReadFile(keyPath)
	if err == nil && len(data) >= 32 {
		return deriveAESKey(data, home), nil
	}

	// Generate 32 bytes of cryptographically secure random entropy
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return nil, fmt.Errorf("cannot generate entropy for credential store: %w", err)
	}
	if err := atomicWriteFile(keyPath, raw, 0o600); err != nil {
		return nil, fmt.Errorf("cannot write encryption key %s: %w", keyPath, err)
	}
	return deriveAESKey(raw, home), nil
}

func deriveAESKey(rawKey []byte, home string) []byte {
	h := sha256.New()
	h.Write(rawKey)
	h.Write([]byte(home))
	if host, err := os.Hostname(); err == nil {
		h.Write([]byte(host))
	}
	return h.Sum(nil)
}

// LoadCredentials reads the credentials file. A missing file is not an error.
// If the file is encrypted (FormatVersion >= 2), it is decrypted with the local machine key.
func LoadCredentials(home string) (*Credentials, error) {
	if strings.TrimSpace(home) == "" {
		h, err := Home()
		if err != nil {
			return nil, err
		}
		home = h
	}
	homeAbs, err := filepath.Abs(home)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(homeAbs, CredentialsFile)
	c := &Credentials{home: homeAbs, path: path, entries: map[string]Credential{}}
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

	if doc.Encrypted {
		if doc.Nonce == "" || doc.Ciphertext == "" {
			return nil, fmt.Errorf("%s: missing encrypted payload", path)
		}
		nonce, err := hex.DecodeString(doc.Nonce)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid nonce: %w", path, err)
		}
		ciphertext, err := hex.DecodeString(doc.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid ciphertext: %w", path, err)
		}

		key, err := getMasterKey(homeAbs)
		if err != nil {
			return nil, err
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("aes init: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("gcm init: %w", err)
		}
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("cannot decrypt %s (key mismatch or corrupted data): %w", path, err)
		}

		var decryptedRegistries map[string]Credential
		if err := json.Unmarshal(plaintext, &decryptedRegistries); err != nil {
			return nil, fmt.Errorf("%s: decrypted data is corrupt: %w", path, err)
		}
		c.entries = decryptedRegistries
	} else if doc.Registries != nil {
		c.entries = doc.Registries
	}

	if c.entries == nil {
		c.entries = map[string]Credential{}
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

// Set stores a token for a registry node, preserving any existing password.
func (c *Credentials) Set(registryURL, username, token string) error {
	key := NormalizeRegistryURL(registryURL)
	existingPassword := ""
	if existing, ok := c.entries[key]; ok {
		existingPassword = existing.Password
	}
	return c.SetWithPassword(registryURL, username, existingPassword, token)
}

// SetWithPassword stores account credentials (username, password, token) for a registry node.
func (c *Credentials) SetWithPassword(registryURL, username, password, token string) error {
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
	c.entries[key] = Credential{
		Username:  username,
		Password:  password,
		Token:     token,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
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
	key, err := getMasterKey(c.home)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("aes init: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm init: %w", err)
	}

	plainData, err := json.Marshal(c.entries)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("cannot generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plainData, nil)

	doc := credentialFile{
		FormatVersion: CredentialsFormatVersion,
		Encrypted:     true,
		Nonce:         hex.EncodeToString(nonce),
		Ciphertext:    hex.EncodeToString(ciphertext),
	}
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
