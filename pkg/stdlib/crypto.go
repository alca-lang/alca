// Package stdlib provides the Crypto & Hash standard library module for the Alca programming language.
// It supports cryptographic hashing (SHA-256, SHA-512, MD5), Base64, Hex, UUIDs, and secure random generation.
package stdlib

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// RegisterCryptoBuiltins registers the "std.crypto" module and its functions into the environment.
func RegisterCryptoBuiltins(env *interpreter.Environment) {
	cryptoModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		cryptoModule.Set(name, bf)
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
		_, _ = rand.Read(b)
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
		if max <= min {
			return &interpreter.IntegerValue{Val: min}
		}
		delta := big.NewInt(max - min + 1)
		n, err := rand.Int(rand.Reader, delta)
		if err != nil {
			return &interpreter.IntegerValue{Val: min}
		}
		return &interpreter.IntegerValue{Val: min + n.Int64()}
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

	env.Set("crypto", cryptoModule)
	env.Set("std.crypto", cryptoModule)
}
