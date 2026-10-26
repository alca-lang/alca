// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package mtproto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// AuthKeySize is the size of an authorization key in bytes.
const AuthKeySize = 256

// AuthKey is an authorization key of a user with a data center.
type AuthKey struct {
	// Value is the 256-byte key.
	Value []byte
	// ID is the key identifier: the lower 64 bits of the key fingerprint.
	ID [8]byte
}

// Fingerprint returns the SHA-1 hash of the key.
func (k AuthKey) Fingerprint() [20]byte {
	return sha1.Sum(k.Value)
}

// NewAuthKey builds an authorization key from its 256 bytes and computes the
// key identifier.
func NewAuthKey(value []byte) (AuthKey, error) {
	if len(value) != AuthKeySize {
		return AuthKey{}, fmt.Errorf("mtproto: the authorization key must be %d bytes, got %d", AuthKeySize, len(value))
	}
	key := AuthKey{Value: append([]byte(nil), value...)}
	fingerprint := key.Fingerprint()
	copy(key.ID[:], fingerprint[12:20])
	return key, nil
}

// ParseAuthKey reads an authorization key from its hexadecimal form.
func ParseAuthKey(text string) (AuthKey, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return AuthKey{}, fmt.Errorf("mtproto: the authorization key is not hexadecimal: %w", err)
	}
	return NewAuthKey(raw)
}

// Hex returns the hexadecimal form of the key.
func (k AuthKey) Hex() string { return hex.EncodeToString(k.Value) }

// AuthKeyID is the identifier of a key: the lower 64 bits of its fingerprint.
func AuthKeyID(value []byte) ([8]byte, error) {
	if len(value) != AuthKeySize {
		return [8]byte{}, fmt.Errorf("mtproto: the authorization key must be %d bytes, got %d", AuthKeySize, len(value))
	}
	fingerprint := sha1.Sum(value)
	var id [8]byte
	copy(id[:], fingerprint[12:20])
	return id, nil
}

// ErrKeyCheckFailed is returned when the server does not confirm the derived
// authorization key.
var ErrKeyCheckFailed = errors.New("mtproto: the server rejected the derived authorization key")

// RSAFingerprint returns the fingerprint of an RSA public key: the SHA-1 hash
// of its modulus.
func RSAFingerprint(pub *rsa.PublicKey) [20]byte {
	return sha1.Sum(pub.N.Bytes())
}

// ParseRSAPublicKey reads a PEM encoded RSA public key.
func ParseRSAPublicKey(text string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, errors.New("mtproto: the public key is not a PEM block")
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaPub, ok := pub.(*rsa.PublicKey); ok {
			return rsaPub, nil
		}
		return nil, errors.New("mtproto: the public key is not an RSA key")
	}
	if pub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return pub, nil
	}
	return nil, errors.New("mtproto: the public key could not be parsed")
}

// DHRequest is the first step of the authorization key generation: the client
// sends its public value and the server answers with its own.
type DHRequest struct {
	// Prime is the modulus of the server's RSA key. It is prime and serves as
	// the modulus of the key exchange.
	Prime *big.Int
	// Generator is the group generator, always 2.
	Generator *big.Int
	// GA is the client's public value.
	GA *big.Int
	// TimeA is the time the request was created at.
	TimeA int64
}

// DHResponse is the server's answer to a DHRequest.
type DHResponse struct {
	// ServerNonce is the nonce of the server.
	ServerNonce []byte
	// GB is the server's public value.
	GB *big.Int
	// NewNonceHash is the hash the client checks the derived key against.
	NewNonceHash []byte
}

// DHExchange is the state of an authorization key exchange. The client keeps
// the secret exponent, the server only ever sees the public value.
type DHExchange struct {
	prime    *big.Int
	exponent *big.Int
	request  DHRequest
}

// NewDHExchange starts a key exchange with the modulus of the server's RSA
// key.
func NewDHExchange(modulus *big.Int, timeA int64) (*DHExchange, error) {
	if modulus == nil || modulus.Sign() <= 0 {
		return nil, errors.New("mtproto: the key exchange needs a modulus")
	}
	exponent, err := rand.Int(rand.Reader, new(big.Int).Sub(modulus, big.NewInt(2)))
	if err != nil {
		return nil, err
	}
	exponent.Add(exponent, big.NewInt(1))
	ga := new(big.Int).Exp(big.NewInt(2), exponent, modulus)
	return &DHExchange{
		prime:    modulus,
		exponent: exponent,
		request: DHRequest{
			Prime:     modulus,
			Generator: big.NewInt(2),
			GA:        ga,
			TimeA:     timeA,
		},
	}, nil
}

// Request returns the request the client sends to the server.
func (x *DHExchange) Request() DHRequest { return x.request }

// AuxHash returns the auxiliary hash of a key exchange. It binds the new
// nonce and the public values of both sides, so that a man in the middle
// cannot substitute them.
func AuxHash(newNonce []byte, gb, gc *big.Int) []byte {
	hasher := sha256.New()
	hasher.Write(newNonce)
	if gb != nil {
		hasher.Write(gb.Bytes())
	}
	if gc != nil {
		hasher.Write(gc.Bytes())
	}
	return hasher.Sum(nil)
}

// DeriveAuthKey derives the authorization key from the shared secret of a key
// exchange. The client passes its own secret exponent and the public value of
// the server, the server its secret and the public value of the client; both
// derive the same key.
//
// peerPublic is the public value of the other side and is raised to the secret
// to get the shared secret. serverPublic is the public value of the server: it
// is the same on both sides and is part of the auxiliary hash.
//
// The auxiliary hash is checked, and the key is confirmed against the hash of
// the server when it is supplied.
func DeriveAuthKey(secret, peerPublic, serverPublic, modulus *big.Int, serverNonce, newNonce, auxHash []byte) (AuthKey, error) {
	if secret == nil {
		return AuthKey{}, errors.New("mtproto: the key exchange needs the shared secret")
	}
	if peerPublic == nil || serverPublic == nil || modulus == nil {
		return AuthKey{}, errors.New("mtproto: the key exchange is incomplete")
	}
	if len(newNonce) != 16 {
		return AuthKey{}, errors.New("mtproto: the new nonce must be 16 bytes")
	}
	if len(auxHash) != 32 {
		return AuthKey{}, errors.New("mtproto: the auxiliary hash must be 32 bytes")
	}
	if len(serverNonce) != 16 {
		return AuthKey{}, errors.New("mtproto: the server nonce must be 16 bytes")
	}

	// The shared secret is g_c = peer^secret mod p.
	gc := new(big.Int).Exp(peerPublic, secret, modulus)
	gcBytes := gc.Bytes()

	// The auxiliary hash binds the nonces and the public values.
	computedAux := AuxHash(newNonce, serverPublic, gc)
	if !equalBytes(computedAux, auxHash) {
		return AuthKey{}, errors.New("mtproto: the auxiliary hash does not match")
	}

	// The key is derived from the shared secret and the nonces.
	msgKeyLarge := sha1.Sum(gcBytes)
	key := make([]byte, 0, 40)
	first := sha1.New()
	first.Write(msgKeyLarge[8:20])
	first.Write(newNonce)
	first.Write(computedAux)
	key = append(key, first.Sum(nil)...)
	second := sha1.New()
	second.Write(msgKeyLarge[0:8])
	second.Write(serverNonce)
	second.Write(newNonce)
	second.Write(computedAux)
	key = append(key, second.Sum(nil)...)

	authKey, err := NewAuthKey(padTo256(key))
	if err != nil {
		return AuthKey{}, err
	}
	return authKey, nil
}

// ConfirmKey checks the hash the server sends with its answer: it is the hash
// of the derived key and the new nonce.
func ConfirmKey(key AuthKey, newNonce, hash []byte) bool {
	if len(hash) == 0 {
		return true
	}
	confirm := sha1.New()
	confirm.Write(key.Value)
	confirm.Write(newNonce)
	return equalBytes(confirm.Sum(nil)[:len(hash)], hash)
}

// Derive completes the key exchange: it computes the shared secret, derives
// the authorization key and checks it against the hash of the server.
//
// The auxiliary hash binds the nonces and the public values. When it is
// supplied it is checked, and the derived key is confirmed against the hash of
// the server.
func (x *DHExchange) Derive(resp DHResponse, newNonce, auxHash []byte) (AuthKey, error) {
	if x == nil || x.prime == nil {
		return AuthKey{}, errors.New("mtproto: the key exchange is not started")
	}
	authKey, err := DeriveAuthKey(x.exponent, resp.GB, resp.GB, x.prime, resp.ServerNonce, newNonce, auxHash)
	if err != nil {
		return AuthKey{}, err
	}
	if !ConfirmKey(authKey, newNonce, resp.NewNonceHash) {
		return AuthKey{}, ErrKeyCheckFailed
	}
	return authKey, nil
}

// padTo256 pads a key to the 256 bytes an authorization key occupies.
func padTo256(key []byte) []byte {
	padded := make([]byte, AuthKeySize)
	copy(padded, key)
	return padded
}

// equalBytes compares two byte slices in constant time.
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// EncryptRSAData encrypts the payload of an authorization request with the
// server's RSA key, as the protocol requires: the payload is XORed with the
// nonce and then encrypted with PKCS#1 v1.5.
func EncryptRSAData(pub *rsa.PublicKey, nonce, data []byte) ([]byte, error) {
	if pub == nil {
		return nil, errors.New("mtproto: an RSA public key is required")
	}
	if len(nonce) != 16 {
		return nil, errors.New("mtproto: the nonce must be 16 bytes")
	}
	if len(data) > 192 {
		return nil, errors.New("mtproto: the authorization payload is too long")
	}
	// The padding is random; the low bits of the first byte carry the flags
	// the server checks.
	padding := make([]byte, 192)
	if _, err := rand.Read(padding); err != nil {
		return nil, err
	}
	padding[0] = (padding[0] & 0xF0) | 0x01
	payload := append(append([]byte{}, padding...), data...)
	for i := range payload {
		payload[i] ^= nonce[i%16]
	}
	return rsa.EncryptPKCS1v15(rand.Reader, pub, payload)
}

// DecodeBase64 reads base64 text, ignoring the line breaks.
func DecodeBase64(text string) ([]byte, error) {
	cleaned := strings.NewReplacer("\n", "", "\r", "", " ", "").Replace(text)
	return base64.StdEncoding.DecodeString(cleaned)
}
