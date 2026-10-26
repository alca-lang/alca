// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package mtproto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"math/big"
	"testing"
	"time"
)

func TestIGERoundTrip(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(iv); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 64)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	encrypted, err := EncryptIGE(key, data, iv)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Equal(encrypted, data) {
		t.Fatal("the encrypted data must differ from the plain data")
	}
	decrypted, err := DecryptIGE(key, encrypted, iv)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, data) {
		t.Fatalf("decrypted = %x, want %x", decrypted, data)
	}
}

func TestIGERejectsBadInput(t *testing.T) {
	key := make([]byte, 32)
	iv := make([]byte, 32)
	if _, err := EncryptIGE(key, []byte{1, 2, 3}, iv); err == nil {
		t.Fatal("the data must be a multiple of the block size")
	}
	if _, err := EncryptIGE(key[:16], make([]byte, 16), iv); err == nil {
		t.Fatal("the key must be 32 bytes")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	key, err := NewAuthKey(make([]byte, AuthKeySize))
	if err != nil {
		t.Fatal(err)
	}
	plain := PlainMessage{
		Salt:      0x1122334455667788,
		SessionID: -0x6644332211aabbcc,
		MsgID:     NewMsgID(16, time.Unix(1700000000, 0)),
		SeqNo:     7,
		Payload:   []byte("the payload of the request"),
	}
	encrypted, err := EncryptMessage(key, plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted.AuthKeyID != key.ID {
		t.Fatal("the message must carry the identifier of the key")
	}
	wire := EncodeMessage(encrypted)
	decoded, err := DecodeMessage(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, err := DecryptMessage(key, ToServer, decoded)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got.Salt != plain.Salt || got.SessionID != plain.SessionID || got.SeqNo != plain.SeqNo {
		t.Fatalf("header = %+v, want %+v", got, plain)
	}
	if !bytes.Equal(got.Payload, plain.Payload) {
		t.Fatalf("payload = %q, want %q", got.Payload, plain.Payload)
	}
}

func TestMessageRejectsForeignKey(t *testing.T) {
	key, err := NewAuthKey(make([]byte, AuthKeySize))
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewAuthKey(bytes.Repeat([]byte{0xAB}, AuthKeySize))
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := EncryptMessage(key, PlainMessage{Payload: []byte("data")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptMessage(other, ToServer, encrypted); err == nil {
		t.Fatal("a message must not decrypt with a foreign key")
	}
}

func TestAuthKeyIdentifier(t *testing.T) {
	value := bytes.Repeat([]byte{0x11}, AuthKeySize)
	key, err := NewAuthKey(value)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := key.Fingerprint()
	if !bytes.Equal(fingerprint[:12], key.ID[:]) && !bytes.Equal(fingerprint[12:], key.ID[:]) {
		t.Fatal("the identifier must be part of the fingerprint")
	}
	id, err := AuthKeyID(value)
	if err != nil {
		t.Fatal(err)
	}
	if id != key.ID {
		t.Fatalf("identifier = %x, want %x", id, key.ID)
	}
	if _, err := NewAuthKey(value[:10]); err == nil {
		t.Fatal("a key of the wrong size must be rejected")
	}
}

func TestAuthKeyExchange(t *testing.T) {
	// The server has an RSA key; its modulus is prime and serves as the
	// modulus of the key exchange.
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewDHExchange(serverKey.N, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	request := client.Request()
	if request.Prime.Cmp(serverKey.N) != 0 {
		t.Fatal("the request must use the modulus of the server")
	}
	if request.Generator.Cmp(big.NewInt(2)) != 0 {
		t.Fatal("the generator must be 2")
	}

	// The server answers with its own public value and the nonces.
	serverSecret, err := rand.Int(rand.Reader, new(big.Int).Sub(serverKey.N, big.NewInt(2)))
	if err != nil {
		t.Fatal(err)
	}
	serverSecret.Add(serverSecret, big.NewInt(1))
	gb := new(big.Int).Exp(big.NewInt(2), serverSecret, serverKey.N)
	serverNonce := make([]byte, 16)
	newNonce := make([]byte, 16)
	if _, err := rand.Read(serverNonce); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(newNonce); err != nil {
		t.Fatal(err)
	}

	// The server computes the auxiliary hash and the confirmation of the key
	// the same way the client does: the shared secret is g_a^b mod p, which is
	// the same value the client computes as g_b^a mod p.
	gc := new(big.Int).Exp(request.GA, serverSecret, serverKey.N)
	auxHash := AuxHash(newNonce, gb, gc)
	clientKey, err := client.Derive(DHResponse{
		ServerNonce: serverNonce,
		GB:          gb,
	}, newNonce, auxHash)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	// The server derives the same key from its own secret and the public
	// value of the client.
	serverKeyValue, err := DeriveAuthKey(serverSecret, request.GA, gb, serverKey.N, serverNonce, newNonce, auxHash)
	if err != nil {
		t.Fatal(err)
	}
	if clientKey.Hex() != serverKeyValue.Hex() {
		t.Fatal("the client and the server must derive the same key")
	}
	if !ConfirmKey(clientKey, newNonce, nil) {
		t.Fatal("the key must be confirmed")
	}
	if len(clientKey.Value) != AuthKeySize {
		t.Fatalf("the key is %d bytes, want %d", len(clientKey.Value), AuthKeySize)
	}
}

func TestDeriveRejectsWrongAuxHash(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewDHExchange(key.N, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Derive(DHResponse{
		ServerNonce: make([]byte, 16),
		GB:          big.NewInt(3),
	}, make([]byte, 16), make([]byte, 32))
	if err == nil {
		t.Fatal("a wrong auxiliary hash must be rejected")
	}
}

func TestObfuscatedHeaderRoundTrip(t *testing.T) {
	header, err := NewObfuscatedHeader(TagIntermediate, 0x0102030405060708, 0x1112131415161718, 42, 3, 128)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := header.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != ObfuscatedHeaderSize {
		t.Fatalf("the header is %d bytes, want %d", len(encoded), ObfuscatedHeaderSize)
	}
	decoded, err := DecodeObfuscatedHeader(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Salt != header.Salt || decoded.SessionID != header.SessionID {
		t.Fatalf("header = %+v, want %+v", decoded, header)
	}
	if decoded.Tag != TagIntermediate {
		t.Fatalf("tag = %x, want %x", decoded.Tag, TagIntermediate)
	}
}

func TestTLWriterAndReader(t *testing.T) {
	writer := NewWriter()
	writer.WriteInt32(-7)
	writer.WriteInt64(1 << 40)
	writer.WriteBool(true)
	writer.WriteString("hello")
	writer.WriteBytes([]byte{1, 2, 3})

	reader := NewReader(writer.Bytes())
	if value, err := reader.ReadInt32(); err != nil || value != -7 {
		t.Fatalf("int32 = %v (%v), want -7", value, err)
	}
	if value, err := reader.ReadInt64(); err != nil || value != 1<<40 {
		t.Fatalf("int64 = %v (%v), want %d", value, err, 1<<40)
	}
	if value, err := reader.ReadBool(); err != nil || !value {
		t.Fatalf("bool = %v (%v), want true", value, err)
	}
	if value, err := reader.ReadString(); err != nil || value != "hello" {
		t.Fatalf("string = %q (%v), want hello", value, err)
	}
	if value, err := reader.ReadBytes(); err != nil || !bytes.Equal(value, []byte{1, 2, 3}) {
		t.Fatalf("bytes = %v (%v), want [1 2 3]", value, err)
	}
}

func TestTLReaderRejectsTruncatedData(t *testing.T) {
	reader := NewReader([]byte{1, 2})
	if _, err := reader.ReadInt32(); err == nil {
		t.Fatal("truncated data must be rejected")
	}
	reader = NewReader([]byte{10, 0, 0, 0, 1, 2})
	if _, err := reader.ReadBytes(); err == nil {
		t.Fatal("a byte string that is longer than the data must be rejected")
	}
}

func TestKnownEndpoints(t *testing.T) {
	endpoints := KnownEndpoints()
	if len(endpoints) == 0 {
		t.Fatal("the network must have data centers")
	}
	for _, endpoint := range endpoints {
		if endpoint.IP == "" || endpoint.Port == 0 {
			t.Fatalf("endpoint %+v is incomplete", endpoint)
		}
	}
}
