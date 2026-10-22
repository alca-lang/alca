// Package stdlib provides the compression standard library module for the Alca programming language.
// It supports Gzip and Zlib compression and decompression.
package stdlib

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/alca-lang/alca/pkg/interpreter"
)

const defaultMaxDecompressedBytes = 16 << 20

// GzipCompress compresses data with gzip and returns base64-encoded string.
func GzipCompress(data []byte) (string, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// GzipDecompress decompresses base64-encoded gzip data.
func GzipDecompress(encoded string) (string, error) {
	return GzipDecompressLimit(encoded, defaultMaxDecompressedBytes)
}

// GzipDecompressLimit decompresses a gzip payload, stopping at maxBytes.
func GzipDecompressLimit(encoded string, maxBytes int64) (string, error) {
	if maxBytes < 0 {
		return "", fmt.Errorf("decompressed size limit must not be negative")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Fallback: try raw bytes
		raw = []byte(encoded)
	}
	r, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(out)) > maxBytes {
		return "", fmt.Errorf("decompressed data exceeds the %d-byte limit", maxBytes)
	}
	return string(out), nil
}

// ZlibCompress compresses data with zlib and returns base64-encoded string.
func ZlibCompress(data []byte) (string, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ZlibDecompress decompresses base64-encoded zlib data.
func ZlibDecompress(encoded string) (string, error) {
	return ZlibDecompressLimit(encoded, defaultMaxDecompressedBytes)
}

// ZlibDecompressLimit decompresses a zlib payload, stopping at maxBytes.
func ZlibDecompressLimit(encoded string, maxBytes int64) (string, error) {
	if maxBytes < 0 {
		return "", fmt.Errorf("decompressed size limit must not be negative")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		raw = []byte(encoded)
	}
	r, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(out)) > maxBytes {
		return "", fmt.Errorf("decompressed data exceeds the %d-byte limit", maxBytes)
	}
	return string(out), nil
}

// RegisterCompressBuiltins registers the "std.compress" and "compress" modules into the environment.
func RegisterCompressBuiltins(env *interpreter.Environment) {
	mod := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		bf := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		mod.Set(name, bf)
		env.Set(name, bf)
	}

	register("gzip_compress", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "gzip_compress requires data argument")
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		res, err := GzipCompress(data)
		if err != nil {
			return interpreter.NewError(500, "gzip_compress error: "+err.Error())
		}
		return &interpreter.StringValue{Val: res}
	})

	register("gzip_decompress", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "gzip_decompress requires data argument")
		}
		encoded := interpreter.CoerceToString(args[0])
		limit, err := decompressionLimit(args)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		res, err := GzipDecompressLimit(encoded, limit)
		if err != nil {
			return interpreter.NewError(500, "gzip_decompress error: "+err.Error())
		}
		return &interpreter.StringValue{Val: res}
	})

	register("zlib_compress", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "zlib_compress requires data argument")
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		res, err := ZlibCompress(data)
		if err != nil {
			return interpreter.NewError(500, "zlib_compress error: "+err.Error())
		}
		return &interpreter.StringValue{Val: res}
	})

	register("zlib_decompress", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "zlib_decompress requires data argument")
		}
		encoded := interpreter.CoerceToString(args[0])
		limit, err := decompressionLimit(args)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		res, err := ZlibDecompressLimit(encoded, limit)
		if err != nil {
			return interpreter.NewError(500, "zlib_decompress error: "+err.Error())
		}
		return &interpreter.StringValue{Val: res}
	})

	env.Set("compress", mod)
	env.Set("std.compress", mod)
}

func decompressionLimit(args []interpreter.Value) (int64, error) {
	if len(args) < 2 {
		return defaultMaxDecompressedBytes, nil
	}
	value, ok := args[1].(*interpreter.IntegerValue)
	if !ok || value.Val < 0 || value.Val > 1<<30 {
		return 0, fmt.Errorf("decompression limit must be an integer between 0 and 1073741824")
	}
	return value.Val, nil
}
