package registry

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// AccountsFormatVersion identifies the accounts.json layout.
const AccountsFormatVersion = 1

// Password hashing parameters (PBKDF2-HMAC-SHA256). They are stored per
// account, so these defaults can change without invalidating existing records.
const (
	// PasswordAlgo names the password hashing function.
	PasswordAlgo = "pbkdf2-sha256"
	// PasswordIterations is the default PBKDF2 iteration count.
	PasswordIterations = 210_000
	passwordSaltBytes  = 16
	passwordKeyBytes   = 32
)

// Token shape: a recognizable prefix plus 32 random bytes in base64url.
const (
	// TokenPrefix starts every issued token.
	TokenPrefix = "alca_"
	tokenBytes  = 32
	// tokenDisplayChars is how much of a token is kept for identification.
	tokenDisplayChars = 13 // "alca_" plus 8 characters
)

// Account errors. The HTTP layer maps them to status codes.
var (
	// ErrRegistrationClosed means self-service registration is disabled.
	ErrRegistrationClosed = errors.New("registration is disabled on this node")
	// ErrUsernameTaken means the username is already registered.
	ErrUsernameTaken = errors.New("username is already taken")
	// ErrEmailTaken means the email address is already registered.
	ErrEmailTaken = errors.New("email is already registered")
	// ErrInvalidInput wraps registration validation failures.
	ErrInvalidInput = errors.New("invalid registration details")
	// ErrInvalidCredentials means the username or password did not match.
	ErrInvalidCredentials = errors.New("invalid username or password")
	// ErrAccountNotFound means no account has that username.
	ErrAccountNotFound = errors.New("account not found")
	// ErrTokenNotFound means the presented token is not issued by this node.
	ErrTokenNotFound = errors.New("token not found")
)

var (
	usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,31}$`)
	emailRe    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// dummyPasswordInfo makes a failed lookup spend the same time as a real
// verification, so usernames cannot be enumerated by timing.
var dummyPasswordInfo = PasswordInfo{
	Algo:       PasswordAlgo,
	Iterations: PasswordIterations,
	Salt:       base64.StdEncoding.EncodeToString(make([]byte, passwordSaltBytes)),
	Hash:       base64.StdEncoding.EncodeToString(make([]byte, passwordKeyBytes)),
}

// PasswordInfo is a salted PBKDF2 password hash.
type PasswordInfo struct {
	Algo       string `json:"algo"`
	Iterations int    `json:"iterations"`
	Salt       string `json:"salt"`
	Hash       string `json:"hash"`
}

// AccountToken is an issued API token. Only its SHA-256 hash is persisted: the
// plaintext value is returned to its owner exactly once, when it is created.
type AccountToken struct {
	ID         string `json:"id"`
	Prefix     string `json:"prefix"`
	Hash       string `json:"hash"`
	Label      string `json:"label,omitempty"`
	CreatedAt  string `json:"created_at"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

// Account is a registered publisher.
type Account struct {
	Username  string         `json:"username"`
	Email     string         `json:"email"`
	CreatedAt string         `json:"created_at"`
	Password  PasswordInfo   `json:"password"`
	Tokens    []AccountToken `json:"tokens,omitempty"`
}

// AccountFile is the accounts.json document.
type AccountFile struct {
	FormatVersion int                 `json:"format_version"`
	UpdatedAt     string              `json:"updated_at,omitempty"`
	Accounts      map[string]*Account `json:"accounts"`
}

// AccountStore is a filesystem-backed account store. It is safe for concurrent
// use; every mutation is written atomically before it is acknowledged.
type AccountStore struct {
	path     string
	mu       sync.Mutex
	accounts map[string]*Account // canonical (lowercase) username -> account
}

// NewAccountStore opens (creating if needed) the accounts file at path.
func NewAccountStore(path string) (*AccountStore, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	s := &AccountStore{path: abs, accounts: map[string]*Account{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Path returns the accounts file path.
func (s *AccountStore) Path() string { return s.path }

// Count returns the number of registered accounts.
func (s *AccountStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.accounts)
}

func (s *AccountStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read %s: %w", s.path, err)
	}
	doc := AccountFile{FormatVersion: AccountsFormatVersion, Accounts: map[string]*Account{}}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s is corrupt: %w", s.path, err)
	}
	if doc.FormatVersion > AccountsFormatVersion {
		return fmt.Errorf("%s format %d is newer than this build supports (%d); upgrade apm-registry", s.path, doc.FormatVersion, AccountsFormatVersion)
	}
	if doc.Accounts != nil {
		s.accounts = doc.Accounts
	}
	return nil
}

// saveLocked writes the store. The caller must hold s.mu. The file holds
// password and token hashes, so it is created with owner-only permissions.
func (s *AccountStore) saveLocked() error {
	doc := AccountFile{
		FormatVersion: AccountsFormatVersion,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Accounts:      s.accounts,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return atomicWrite(s.path, data, 0o600)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// CanonicalUsername validates a username and returns its canonical form.
// Usernames are case-insensitive: they are stored and compared in lowercase.
func CanonicalUsername(name string) (string, error) {
	canon := strings.ToLower(strings.TrimSpace(name))
	if !usernameRe.MatchString(canon) {
		return "", fmt.Errorf("%w: username must be 3-32 characters, start with a letter or digit, and use only a-z, 0-9, '.', '_' or '-'", ErrInvalidInput)
	}
	return canon, nil
}

// ValidateEmail checks the shape of an email address. The node does not send
// mail: the address is contact information for the operator.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" || len(email) > 254 || !emailRe.MatchString(email) {
		return fmt.Errorf("%w: %q is not a valid email address", ErrInvalidInput, strings.TrimSpace(email))
	}
	return nil
}

// ValidatePassword enforces the password policy.
func ValidatePassword(password string) error {
	switch {
	case len(password) < 8:
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	case len(password) > 256:
		return fmt.Errorf("%w: password must be at most 256 characters", ErrInvalidInput)
	case strings.TrimSpace(password) == "":
		return fmt.Errorf("%w: password must not be blank", ErrInvalidInput)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Passwords and tokens
// ---------------------------------------------------------------------------

func hashPassword(password string) (PasswordInfo, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return PasswordInfo{}, err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, PasswordIterations, passwordKeyBytes)
	if err != nil {
		return PasswordInfo{}, err
	}
	return PasswordInfo{
		Algo:       PasswordAlgo,
		Iterations: PasswordIterations,
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Hash:       base64.StdEncoding.EncodeToString(key),
	}, nil
}

func verifyPassword(info PasswordInfo, password string) bool {
	if info.Algo != PasswordAlgo || info.Iterations <= 0 {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(info.Salt)
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := base64.StdEncoding.DecodeString(info.Hash)
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, info.Iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// newTokenValue returns a fresh token with its storage form: the SHA-256 hash,
// the display prefix and the revocation id.
func newTokenValue() (token, hash, prefix, id string, err error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", "", "", err
	}
	token = TokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	hash = sha256HexBytes([]byte(token))
	prefix = token
	if len(prefix) > tokenDisplayChars {
		prefix = prefix[:tokenDisplayChars]
	}
	idRaw := make([]byte, 8)
	if _, err := rand.Read(idRaw); err != nil {
		return "", "", "", "", err
	}
	return token, hash, prefix, hex.EncodeToString(idRaw), nil
}

func issueTokenLocked(acct *Account, label, now string) (string, *AccountToken, error) {
	token, hash, prefix, id, err := newTokenValue()
	if err != nil {
		return "", nil, err
	}
	acct.Tokens = append(acct.Tokens, AccountToken{
		ID: id, Prefix: prefix, Hash: hash, Label: label, CreatedAt: now,
	})
	return token, &acct.Tokens[len(acct.Tokens)-1], nil
}

func cloneAccount(a *Account) *Account {
	cp := *a
	if a.Tokens != nil {
		cp.Tokens = append([]AccountToken(nil), a.Tokens...)
	}
	return &cp
}

// ---------------------------------------------------------------------------
// Operations
// ---------------------------------------------------------------------------

// Register creates an account and returns its first token, which is shown to
// the caller exactly once.
func (s *AccountStore) Register(username, email, password string) (*Account, string, error) {
	canon, err := CanonicalUsername(username)
	if err != nil {
		return nil, "", err
	}
	email = strings.TrimSpace(email)
	if err := ValidateEmail(email); err != nil {
		return nil, "", err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, "", err
	}
	pw, err := hashPassword(password)
	if err != nil {
		return nil, "", err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[canon]; ok {
		return nil, "", ErrUsernameTaken
	}
	for _, existing := range s.accounts {
		if strings.EqualFold(existing.Email, email) {
			return nil, "", ErrEmailTaken
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	acct := &Account{Username: canon, Email: email, CreatedAt: now, Password: pw}
	token, _, err := issueTokenLocked(acct, "registration", now)
	if err != nil {
		return nil, "", err
	}
	s.accounts[canon] = acct
	if err := s.saveLocked(); err != nil {
		delete(s.accounts, canon)
		return nil, "", err
	}
	return cloneAccount(acct), token, nil
}

// Authenticate verifies a username and password.
func (s *AccountStore) Authenticate(username, password string) (*Account, error) {
	canon := strings.ToLower(strings.TrimSpace(username))
	s.mu.Lock()
	acct, ok := s.accounts[canon]
	s.mu.Unlock()
	if !ok {
		verifyPassword(dummyPasswordInfo, password)
		return nil, ErrInvalidCredentials
	}
	if !verifyPassword(acct.Password, password) {
		return nil, ErrInvalidCredentials
	}
	return cloneAccount(acct), nil
}

// IssueToken creates an extra token for an account and returns its plaintext
// value, shown exactly once.
func (s *AccountStore) IssueToken(username, label string) (string, *AccountToken, error) {
	canon := strings.ToLower(strings.TrimSpace(username))
	s.mu.Lock()
	defer s.mu.Unlock()
	acct, ok := s.accounts[canon]
	if !ok {
		return "", nil, ErrAccountNotFound
	}
	now := time.Now().UTC().Format(time.RFC3339)
	token, tok, err := issueTokenLocked(acct, label, now)
	if err != nil {
		return "", nil, err
	}
	if err := s.saveLocked(); err != nil {
		acct.Tokens = acct.Tokens[:len(acct.Tokens)-1]
		return "", nil, err
	}
	cp := *tok
	return token, &cp, nil
}

// FindToken resolves a plaintext token to its account and recorded token. The
// comparison is constant time; plaintext tokens are never stored.
func (s *AccountStore) FindToken(token string) (*Account, *AccountToken, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil, ErrTokenNotFound
	}
	sum := sha256HexBytes([]byte(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, acct := range s.accounts {
		for i := range acct.Tokens {
			if subtle.ConstantTimeCompare([]byte(acct.Tokens[i].Hash), []byte(sum)) == 1 {
				cp := acct.Tokens[i]
				return cloneAccount(acct), &cp, nil
			}
		}
	}
	return nil, nil, ErrTokenNotFound
}

// RevokeToken deletes the token with the given plaintext value.
func (s *AccountStore) RevokeToken(token string) (*Account, error) {
	sum := sha256HexBytes([]byte(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, acct := range s.accounts {
		for i := range acct.Tokens {
			if subtle.ConstantTimeCompare([]byte(acct.Tokens[i].Hash), []byte(sum)) == 1 {
				acct.Tokens = append(acct.Tokens[:i], acct.Tokens[i+1:]...)
				if err := s.saveLocked(); err != nil {
					return nil, err
				}
				return cloneAccount(acct), nil
			}
		}
	}
	return nil, ErrTokenNotFound
}

// TouchToken records that a token was used. Callers may log a failure but
// should let the request proceed: the timestamp is informational.
func (s *AccountStore) TouchToken(username, tokenID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	defer s.mu.Unlock()
	acct, ok := s.accounts[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return ErrAccountNotFound
	}
	for i := range acct.Tokens {
		if acct.Tokens[i].ID == tokenID {
			acct.Tokens[i].LastUsedAt = now
			return s.saveLocked()
		}
	}
	return ErrTokenNotFound
}

// Snapshot returns a copy of an account that is safe to read without the lock.
func (s *AccountStore) Snapshot(username string) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	acct, ok := s.accounts[strings.ToLower(strings.TrimSpace(username))]
	if !ok {
		return nil, ErrAccountNotFound
	}
	return cloneAccount(acct), nil
}
