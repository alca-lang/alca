// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

func TestCryptoRequiresExplicitAESKeyLengthsAndSupportsPasswordEncryption(t *testing.T) {
	env := interpreter.NewEnvironment()
	RegisterCryptoBuiltins(env)
	cryptoModVal, _ := env.Get("crypto")
	cryptoMod := cryptoModVal.(*interpreter.MapValue)
	aesEncrypt, _ := cryptoMod.Get("aes_encrypt")
	aesDecrypt, _ := cryptoMod.Get("aes_decrypt")
	for _, key := range []string{"", "123456789012345", "12345678901234567"} {
		got := aesEncrypt.(*interpreter.BuiltinFunction).Fn(
			&interpreter.StringValue{Val: key},
			&interpreter.StringValue{Val: "data"},
		)
		if _, ok := got.(*interpreter.ErrorValue); !ok {
			t.Fatalf("aes_encrypt accepted %d-byte key: %T", len(key), got)
		}
	}

	validKey := "0123456789abcdef0123456789abcdef"
	ciphertext := aesEncrypt.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: validKey},
		&interpreter.StringValue{Val: "raw-key data"},
	)
	plaintext := aesDecrypt.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: validKey}, ciphertext)
	if got, ok := plaintext.(*interpreter.StringValue); !ok || got.Val != "raw-key data" {
		t.Fatalf("raw AES key round trip = %v", plaintext)
	}

	encryptPassword, _ := cryptoMod.Get("aes_encrypt_password")
	decryptPassword, _ := cryptoMod.Get("aes_decrypt_password")
	password := &interpreter.StringValue{Val: "correct horse battery staple"}
	passwordCiphertext := encryptPassword.(*interpreter.BuiltinFunction).Fn(password, &interpreter.StringValue{Val: "password protected"})
	passwordPlaintext := decryptPassword.(*interpreter.BuiltinFunction).Fn(password, passwordCiphertext)
	if got, ok := passwordPlaintext.(*interpreter.StringValue); !ok || got.Val != "password protected" {
		t.Fatalf("password AES round trip = %v", passwordPlaintext)
	}
	wrongPassword := decryptPassword.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: "wrong"}, passwordCiphertext)
	if _, ok := wrongPassword.(*interpreter.ErrorValue); !ok {
		t.Fatalf("password decryption with wrong password succeeded: %v", wrongPassword)
	}
	raw, err := base64.StdEncoding.DecodeString(passwordCiphertext.(*interpreter.StringValue).Val)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw[:4]) != passwordCipherMagic || raw[4] != passwordCipherVersion ||
		binary.BigEndian.Uint32(raw[5:9]) != passwordCipherIterations {
		t.Fatalf("password ciphertext header does not contain the expected format version and iteration count")
	}

	legacyKey := sha256.Sum256([]byte(password.Val))
	block, err := aes.NewCipher(legacyKey[:])
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatal(err)
	}
	legacyPayload := gcm.Seal(nonce, nonce, []byte("legacy password data"), nil)
	legacyCiphertext := &interpreter.StringValue{Val: base64.StdEncoding.EncodeToString(legacyPayload)}
	legacyPlaintext := decryptPassword.(*interpreter.BuiltinFunction).Fn(password, legacyCiphertext)
	if got, ok := legacyPlaintext.(*interpreter.StringValue); !ok || got.Val != "legacy password data" {
		t.Fatalf("legacy password ciphertext decryption = %v", legacyPlaintext)
	}
}

func TestPBKDF2SHA256KnownVector(t *testing.T) {
	key, err := pbkdf2.Key(sha256.New, "password", []byte("salt"), 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(key)
	const want = "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"
	if got != want {
		t.Fatalf("PBKDF2 result = %s, want %s", got, want)
	}
}

func TestHTTPBodyLimitRepeatedHeadersAndTimeoutOption(t *testing.T) {
	prevPolicy := sandbox.CurrentPolicy()
	sandbox.SetPolicy(sandbox.AllowAll())
	defer sandbox.SetPolicy(prevPolicy)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/headers":
			w.Header().Add("Set-Cookie", "first=1")
			w.Header().Add("Set-Cookie", "second=2")
			w.Header().Set("X-Single", "one")
			_, _ = w.Write([]byte("ok"))
		case "/large":
			_, _ = w.Write(make([]byte, maxHTTPBodyBytes+1))
		case "/slow":
			time.Sleep(200 * time.Millisecond)
			_, _ = w.Write([]byte("late"))
		}
	}))
	defer server.Close()

	env := interpreter.NewEnvironment()
	RegisterHTTPBuiltins(env)
	httpModuleValue, _ := env.Get("http")
	httpModule := httpModuleValue.(*interpreter.MapValue)
	requestValue, _ := httpModule.Get("request")
	request := requestValue.(*interpreter.BuiltinFunction).Fn
	call := request

	headers := request(&interpreter.StringValue{Val: "GET"}, &interpreter.StringValue{Val: server.URL + "/headers"})
	headersMap, ok := headers.(*interpreter.MapValue)
	if !ok {
		t.Fatalf("headers response = %v", headers)
	}
	headerVal, ok := headersMap.Get("headers")
	if !ok {
		t.Fatal("HTTP response did not include headers")
	}
	headerMap := headerVal.(*interpreter.MapValue)
	cookies, ok := headerMap.Get("Set-Cookie")
	if !ok {
		t.Fatal("Set-Cookie missing from response")
	}
	cookieValues, ok := cookies.(*interpreter.ListValue)
	if !ok || len(cookieValues.ElementsSlice()) != 2 {
		t.Fatalf("Set-Cookie = %T %v, want a two-item list", cookies, cookies)
	}
	single, ok := headerMap.Get("X-Single")
	if !ok {
		t.Fatal("single-value header missing from response")
	}
	singleValues, ok := single.(*interpreter.ListValue)
	if !ok || len(singleValues.ElementsSlice()) != 1 {
		t.Fatalf("X-Single = %T %v, want a one-item list", single, single)
	}

	large := call(&interpreter.StringValue{Val: "GET"}, &interpreter.StringValue{Val: server.URL + "/large"})
	if err, ok := large.(*interpreter.ErrorValue); !ok || err.Code != 413 {
		t.Fatalf("oversized response = %v, want HTTP 413 error", large)
	}

	options := interpreter.NewMap()
	options.Set("timeout", &interpreter.FloatValue{Val: 0.05})
	getValue, _ := httpModule.Get("get")
	timed := getValue.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: server.URL + "/slow"}, nil, &interpreter.StringValue{Val: ""}, options)
	if _, ok := timed.(*interpreter.ErrorValue); !ok {
		t.Fatalf("request with short timeout returned %T, want an error", timed)
	}
}

func TestHTTPServerTimeoutConfiguration(t *testing.T) {
	server := newHTTPServer(http.NewServeMux())
	if server.ReadHeaderTimeout != 10*time.Second ||
		server.ReadTimeout != 30*time.Second ||
		server.WriteTimeout != 60*time.Second ||
		server.IdleTimeout != 120*time.Second {
		t.Fatalf("unexpected HTTP server timeouts: %+v", server)
	}
}
