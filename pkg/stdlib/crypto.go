// Package stdlib provides the Crypto & Hash standard library module for the Alca programming language.
// It supports cryptographic hashing (SHA-256, SHA-512, MD5), Base64, Hex, UUIDs, and secure random generation.
package stdlib

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"

	"github.com/alca-lang/alca/pkg/interpreter"
)

const (
	passwordCipherMagic      = "ALCA"
	passwordCipherVersion    = byte(1)
	passwordCipherIterations = 600_000
	passwordCipherMaxRounds  = 1_000_000
	passwordCipherSaltSize   = 16
)

// RegisterCryptoBuiltins registers the "std.crypto" module and its functions into the environment.
func RegisterCryptoBuiltins(env *interpreter.Environment) {
	cryptoModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		cryptoModule.Set(name, bf)
		env.Set(name, bf)
	}

	// sha256(data) -> hex string
	register("sha256", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "crypto.sha256 requires data argument")
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		sum := sha256.Sum256(data)
		return &interpreter.StringValue{Val: hex.EncodeToString(sum[:])}
	})

	// sha512(data) -> hex string
	register("sha512", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "crypto.sha512 requires data argument")
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		sum := sha512.Sum512(data)
		return &interpreter.StringValue{Val: hex.EncodeToString(sum[:])}
	})

	register("random_bytes", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 1 {
			return interpreter.NewError(400, "crypto.random_bytes requires a byte length")
		}
		length, ok := args[0].(*interpreter.IntegerValue)
		if !ok || length.Val < 0 || length.Val > 16<<20 {
			return interpreter.NewError(400, "random byte length must be an integer between 0 and 16777216")
		}
		data := make([]byte, int(length.Val))
		if _, err := io.ReadFull(rand.Reader, data); err != nil {
			return interpreter.NewError(500, "secure random generation failed: "+err.Error())
		}
		return &interpreter.BlobValue{Data: data}
	})

	register("constant_time_equal", func(args ...interpreter.Value) interpreter.Value {
		if len(args) != 2 {
			return interpreter.NewError(400, "crypto.constant_time_equal requires two strings")
		}
		a := []byte(interpreter.CoerceToString(args[0]))
		b := []byte(interpreter.CoerceToString(args[1]))
		if len(a) != len(b) {
			return interpreter.FALSE
		}
		if subtle.ConstantTimeCompare(a, b) == 1 {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// sha1(data) -> hex string
	register("sha1", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "crypto.sha1 requires data argument")
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		sum := sha1.Sum(data)
		return &interpreter.StringValue{Val: hex.EncodeToString(sum[:])}
	})

	// md5(data) -> hex string
	register("md5", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "crypto.md5 requires data argument")
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		sum := md5.Sum(data)
		return &interpreter.StringValue{Val: hex.EncodeToString(sum[:])}
	})

	// base64_encode(data) -> string
	register("base64_encode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		return &interpreter.StringValue{Val: base64.StdEncoding.EncodeToString(data)}
	})

	// base64_decode(data) -> string
	register("base64_decode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		str := interpreter.CoerceToString(args[0])
		decoded, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			return interpreter.NewError(400, fmt.Sprintf("invalid base64: %s", err.Error()))
		}
		return &interpreter.StringValue{Val: string(decoded)}
	})

	// hex_encode(data) -> string
	register("hex_encode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		return &interpreter.StringValue{Val: hex.EncodeToString(data)}
	})

	// hex_decode(data) -> string
	register("hex_decode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		str := interpreter.CoerceToString(args[0])
		decoded, err := hex.DecodeString(str)
		if err != nil {
			return interpreter.NewError(400, fmt.Sprintf("invalid hex string: %s", err.Error()))
		}
		return &interpreter.StringValue{Val: string(decoded)}
	})

	// uuid() -> RFC 4122 v4 UUID string
	register("uuid", func(args ...interpreter.Value) interpreter.Value {
		b := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, b); err != nil {
			return interpreter.NewError(500, "secure UUID generation failed: "+err.Error())
		}
		b[6] = (b[6] & 0x0f) | 0x40 // Version 4
		b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
		uuidStr := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
		return &interpreter.StringValue{Val: uuidStr}
	})

	// random_string(length, charset?) -> string
	register("random_string", func(args ...interpreter.Value) interpreter.Value {
		length := 16
		if len(args) >= 1 {
			if iv, ok := args[0].(*interpreter.IntegerValue); ok {
				if iv.Val < 0 || iv.Val > 1<<20 {
					return interpreter.NewError(400, "random string length must be between 0 and 1048576")
				}
				length = int(iv.Val)
			}
		}
		charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		if len(args) >= 2 {
			custom := interpreter.CoerceToString(args[1])
			if custom != "" {
				charset = custom
			}
		}
		if length < 0 || length > 1<<20 {
			return interpreter.NewError(400, "random string length must be between 0 and 1048576")
		}
		if len(charset) == 0 && length > 0 {
			return interpreter.NewError(400, "random string charset must not be empty")
		}

		result := make([]byte, length)
		charsetLen := big.NewInt(int64(len(charset)))
		for i := 0; i < length; i++ {
			n, err := rand.Int(rand.Reader, charsetLen)
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			result[i] = charset[n.Int64()]
		}
		return &interpreter.StringValue{Val: string(result)}
	})

	// random_int(min, max) -> int
	register("random_int", func(args ...interpreter.Value) interpreter.Value {
		min := int64(0)
		max := int64(100)
		if len(args) >= 1 {
			if iv, ok := args[0].(*interpreter.IntegerValue); ok {
				min = iv.Val
			}
		}
		if len(args) >= 2 {
			if iv, ok := args[1].(*interpreter.IntegerValue); ok {
				max = iv.Val
			}
		}
		if max < min {
			return interpreter.NewError(400, "random_int maximum must be greater than or equal to minimum")
		}
		delta := new(big.Int).Sub(big.NewInt(max), big.NewInt(min))
		delta.Add(delta, big.NewInt(1))
		n, err := rand.Int(rand.Reader, delta)
		if err != nil {
			return interpreter.NewError(500, "secure random generation failed: "+err.Error())
		}
		result := new(big.Int).Add(big.NewInt(min), n)
		if !result.IsInt64() {
			return interpreter.NewError(500, "random integer result is outside the supported range")
		}
		return &interpreter.IntegerValue{Val: result.Int64()}
	})

	// hmac_sha256(key, data) -> hex string
	register("hmac_sha256", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "crypto.hmac_sha256 requires (key, data) arguments")
		}
		key := []byte(interpreter.CoerceToString(args[0]))
		data := []byte(interpreter.CoerceToString(args[1]))
		h := hmac.New(sha256.New, key)
		h.Write(data)
		return &interpreter.StringValue{Val: hex.EncodeToString(h.Sum(nil))}
	})

	// hmac_sha512(key, data) -> hex string
	register("hmac_sha512", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "crypto.hmac_sha512 requires (key, data) arguments")
		}
		key := []byte(interpreter.CoerceToString(args[0]))
		data := []byte(interpreter.CoerceToString(args[1]))
		h := hmac.New(sha512.New, key)
		h.Write(data)
		return &interpreter.StringValue{Val: hex.EncodeToString(h.Sum(nil))}
	})

	// aes_encrypt(key, plaintext) -> base64 string; key must be 16, 24, or 32 bytes.
	register("aes_encrypt", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "crypto.aes_encrypt requires (key, plaintext) arguments")
		}
		key := []byte(interpreter.CoerceToString(args[0]))
		if len(key) != 16 && len(key) != 24 && len(key) != 32 {
			return interpreter.NewError(400, "AES key must be exactly 16, 24, or 32 bytes; use aes_encrypt_password for passwords")
		}
		plaintext := []byte(interpreter.CoerceToString(args[1]))
		block, err := aes.NewCipher(key)
		if err != nil {
			return interpreter.NewError(500, "aes cipher error: "+err.Error())
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return interpreter.NewError(500, "aes gcm error: "+err.Error())
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return interpreter.NewError(500, "aes nonce error: "+err.Error())
		}
		ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
		return &interpreter.StringValue{Val: base64.StdEncoding.EncodeToString(ciphertext)}
	})

	// aes_decrypt(key, ciphertextBase64) -> string; key must be 16, 24, or 32 bytes.
	register("aes_decrypt", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "crypto.aes_decrypt requires (key, ciphertext) arguments")
		}
		key := []byte(interpreter.CoerceToString(args[0]))
		if len(key) != 16 && len(key) != 24 && len(key) != 32 {
			return interpreter.NewError(400, "AES key must be exactly 16, 24, or 32 bytes; use aes_decrypt_password for passwords")
		}
		encoded := interpreter.CoerceToString(args[1])
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return interpreter.NewError(400, "invalid base64 ciphertext: "+err.Error())
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return interpreter.NewError(500, "aes cipher error: "+err.Error())
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return interpreter.NewError(500, "aes gcm error: "+err.Error())
		}
		nonceSize := gcm.NonceSize()
		if len(raw) < nonceSize {
			return interpreter.NewError(400, "ciphertext too short")
		}
		nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return interpreter.NewError(400, "decryption failed (wrong key or corrupted data): "+err.Error())
		}
		return &interpreter.StringValue{Val: string(plaintext)}
	})

	register("derive_key", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 4 {
			return interpreter.NewError(400, "crypto.derive_key requires (password, salt, iterations, key_len) arguments")
		}
		password := []byte(interpreter.CoerceToString(args[0]))
		salt := []byte(interpreter.CoerceToString(args[1]))
		iterations, ok := args[2].(*interpreter.IntegerValue)
		if !ok || iterations.Val < 1 || iterations.Val > 1_000_000 {
			return interpreter.NewError(400, "PBKDF2 iterations must be an integer from 1 to 1000000")
		}
		keyLen, ok := args[3].(*interpreter.IntegerValue)
		if !ok || (keyLen.Val != 16 && keyLen.Val != 24 && keyLen.Val != 32) {
			return interpreter.NewError(400, "derived AES key length must be 16, 24, or 32 bytes")
		}
		if len(salt) == 0 {
			return interpreter.NewError(400, "PBKDF2 salt must not be empty")
		}
		key, err := pbkdf2.Key(sha256.New, string(password), salt, int(iterations.Val), int(keyLen.Val))
		if err != nil {
			return interpreter.NewError(400, "key derivation failed: "+err.Error())
		}
		return &interpreter.StringValue{Val: string(key)}
	})

	register("aes_encrypt_password", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "crypto.aes_encrypt_password requires (password, plaintext) arguments")
		}
		password := []byte(interpreter.CoerceToString(args[0]))
		salt := make([]byte, passwordCipherSaltSize)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return interpreter.NewError(500, "AES password salt error: "+err.Error())
		}
		key, err := pbkdf2.Key(sha256.New, string(password), salt, passwordCipherIterations, 32)
		if err != nil {
			return interpreter.NewError(500, "AES password key derivation failed: "+err.Error())
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return interpreter.NewError(500, "aes cipher error: "+err.Error())
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return interpreter.NewError(500, "aes gcm error: "+err.Error())
		}
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return interpreter.NewError(500, "aes nonce error: "+err.Error())
		}
		plaintext := []byte(interpreter.CoerceToString(args[1]))
		header := make([]byte, 9, 9+len(salt)+len(nonce)+len(plaintext)+gcm.Overhead())
		copy(header, passwordCipherMagic)
		header[4] = passwordCipherVersion
		binary.BigEndian.PutUint32(header[5:9], uint32(passwordCipherIterations))
		header = append(header, salt...)
		sealed := gcm.Seal(nil, nonce, plaintext, header)
		payload := append(header, nonce...)
		payload = append(payload, sealed...)
		return &interpreter.StringValue{Val: base64.StdEncoding.EncodeToString(payload)}
	})

	register("aes_decrypt_password", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "crypto.aes_decrypt_password requires (password, ciphertext) arguments")
		}
		payload, err := base64.StdEncoding.DecodeString(interpreter.CoerceToString(args[1]))
		if err != nil {
			return interpreter.NewError(400, "invalid or truncated password ciphertext")
		}
		if !bytes.HasPrefix(payload, []byte(passwordCipherMagic)) {
			return decryptLegacyPasswordCiphertext([]byte(interpreter.CoerceToString(args[0])), payload)
		}
		const headerSize = 9
		if len(payload) < headerSize+passwordCipherSaltSize+12+16 {
			return interpreter.NewError(400, "invalid or truncated password ciphertext")
		}
		if payload[4] != passwordCipherVersion {
			return interpreter.NewError(400, fmt.Sprintf("unsupported password ciphertext version %d", payload[4]))
		}
		iterations := int(binary.BigEndian.Uint32(payload[5:9]))
		if iterations < 100_000 || iterations > passwordCipherMaxRounds {
			return interpreter.NewError(400, "password ciphertext iteration count is outside the supported range")
		}
		headerEnd := headerSize + passwordCipherSaltSize
		salt := payload[headerSize:headerEnd]
		key, err := pbkdf2.Key(sha256.New, interpreter.CoerceToString(args[0]), salt, iterations, 32)
		if err != nil {
			return interpreter.NewError(400, "password key derivation failed: "+err.Error())
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return interpreter.NewError(500, "aes cipher error: "+err.Error())
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return interpreter.NewError(500, "aes gcm error: "+err.Error())
		}
		nonceEnd := headerEnd + gcm.NonceSize()
		if len(payload) < nonceEnd+gcm.Overhead() {
			return interpreter.NewError(400, "password ciphertext is too short")
		}
		plaintext, err := gcm.Open(nil, payload[headerEnd:nonceEnd], payload[nonceEnd:], payload[:headerEnd])
		if err != nil {
			return interpreter.NewError(400, "decryption failed (wrong password or corrupted data)")
		}
		return &interpreter.StringValue{Val: string(plaintext)}
	})

	env.Set("crypto", cryptoModule)
	env.Set("std.crypto", cryptoModule)
}

func decryptLegacyPasswordCiphertext(password, payload []byte) interpreter.Value {
	blockKey := password
	if len(blockKey) != 16 && len(blockKey) != 24 && len(blockKey) != 32 {
		sum := sha256.Sum256(blockKey)
		blockKey = sum[:]
	}
	block, err := aes.NewCipher(blockKey)
	if err != nil {
		return interpreter.NewError(400, "invalid legacy password ciphertext")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return interpreter.NewError(500, "aes gcm error: "+err.Error())
	}
	if len(payload) < gcm.NonceSize()+gcm.Overhead() {
		return interpreter.NewError(400, "invalid or truncated legacy password ciphertext")
	}
	plaintext, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], nil)
	if err != nil {
		return interpreter.NewError(400, "decryption failed (wrong password or corrupted data)")
	}
	return &interpreter.StringValue{Val: string(plaintext)}
}
