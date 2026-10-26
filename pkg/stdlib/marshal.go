// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/alca-lang/alca/pkg/interpreter"
)

const (
	tagNull   byte = 0
	tagBool   byte = 1
	tagInt    byte = 2
	tagFloat  byte = 3
	tagString byte = 4
	tagBlob   byte = 5
	tagList   byte = 6
	tagMap    byte = 7
	tagError  byte = 8
)

// serializeValue writes an Alca Value into a binary stream.
func serializeValue(w io.Writer, val interpreter.Value) error {
	if val == nil || val == interpreter.NULL {
		_, err := w.Write([]byte{tagNull})
		return err
	}

	switch v := val.(type) {
	case *interpreter.BoolValue:
		b := byte(0)
		if v.Val {
			b = 1
		}
		_, err := w.Write([]byte{tagBool, b})
		return err

	case *interpreter.IntegerValue:
		if _, err := w.Write([]byte{tagInt}); err != nil {
			return err
		}
		var buf [10]byte
		n := binary.PutVarint(buf[:], v.Val)
		_, err := w.Write(buf[:n])
		return err

	case *interpreter.FloatValue:
		if _, err := w.Write([]byte{tagFloat}); err != nil {
			return err
		}
		bits := math.Float64bits(v.Val)
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], bits)
		_, err := w.Write(buf[:])
		return err

	case *interpreter.StringValue:
		if _, err := w.Write([]byte{tagString}); err != nil {
			return err
		}
		data := []byte(v.Val)
		var buf [10]byte
		n := binary.PutUvarint(buf[:], uint64(len(data)))
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		_, err := w.Write(data)
		return err

	case *interpreter.BlobValue:
		if _, err := w.Write([]byte{tagBlob}); err != nil {
			return err
		}
		data := v.Data
		var buf [10]byte
		n := binary.PutUvarint(buf[:], uint64(len(data)))
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		_, err := w.Write(data)
		return err

	case *interpreter.ListValue:
		if _, err := w.Write([]byte{tagList}); err != nil {
			return err
		}
		elems := v.ElementsSlice()
		var buf [10]byte
		n := binary.PutUvarint(buf[:], uint64(len(elems)))
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		for _, elem := range elems {
			if err := serializeValue(w, elem); err != nil {
				return err
			}
		}
		return nil

	case *interpreter.MapValue:
		if _, err := w.Write([]byte{tagMap}); err != nil {
			return err
		}
		keys := v.Keys()
		var buf [10]byte
		n := binary.PutUvarint(buf[:], uint64(len(keys)))
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		for _, key := range keys {
			// write key
			keyBytes := []byte(key)
			kn := binary.PutUvarint(buf[:], uint64(len(keyBytes)))
			if _, err := w.Write(buf[:kn]); err != nil {
				return err
			}
			if _, err := w.Write(keyBytes); err != nil {
				return err
			}
			// write value
			val, _ := v.Get(key)
			if err := serializeValue(w, val); err != nil {
				return err
			}
		}
		return nil

	case *interpreter.ErrorValue:
		if _, err := w.Write([]byte{tagError}); err != nil {
			return err
		}
		var buf [10]byte
		n := binary.PutVarint(buf[:], int64(v.Code))
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		msgBytes := []byte(v.Message)
		mn := binary.PutUvarint(buf[:], uint64(len(msgBytes)))
		if _, err := w.Write(buf[:mn]); err != nil {
			return err
		}
		_, err := w.Write(msgBytes)
		return err

	default:
		// Fallback: serialize as string representation
		if _, err := w.Write([]byte{tagString}); err != nil {
			return err
		}
		s := interpreter.CoerceToString(val)
		data := []byte(s)
		var buf [10]byte
		n := binary.PutUvarint(buf[:], uint64(len(data)))
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		_, err := w.Write(data)
		return err
	}
}

// deserializeValue reads an Alca Value from a binary stream.
func deserializeValue(r *bytes.Reader) (interpreter.Value, error) {
	tag, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch tag {
	case tagNull:
		return interpreter.NULL, nil

	case tagBool:
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != 0 {
			return interpreter.TRUE, nil
		}
		return interpreter.FALSE, nil

	case tagInt:
		val, err := binary.ReadVarint(r)
		if err != nil {
			return nil, err
		}
		return &interpreter.IntegerValue{Val: val}, nil

	case tagFloat:
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		bits := binary.LittleEndian.Uint64(buf[:])
		return &interpreter.FloatValue{Val: math.Float64frombits(bits)}, nil

	case tagString:
		length, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}
		data := make([]byte, length)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		return &interpreter.StringValue{Val: string(data)}, nil

	case tagBlob:
		length, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}
		data := make([]byte, length)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		return &interpreter.BlobValue{Data: data}, nil

	case tagList:
		count, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}
		elems := make([]interpreter.Value, count)
		for i := 0; i < int(count); i++ {
			elem, err := deserializeValue(r)
			if err != nil {
				return nil, err
			}
			elems[i] = elem
		}
		return interpreter.NewList(elems...), nil

	case tagMap:
		count, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}
		m := interpreter.NewMap()
		for i := 0; i < int(count); i++ {
			klen, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, err
			}
			kdata := make([]byte, klen)
			if _, err := io.ReadFull(r, kdata); err != nil {
				return nil, err
			}
			val, err := deserializeValue(r)
			if err != nil {
				return nil, err
			}
			m.Set(string(kdata), val)
		}
		return m, nil

	case tagError:
		code, err := binary.ReadVarint(r)
		if err != nil {
			return nil, err
		}
		mlen, err := binary.ReadUvarint(r)
		if err != nil {
			return nil, err
		}
		mdata := make([]byte, mlen)
		if _, err := io.ReadFull(r, mdata); err != nil {
			return nil, err
		}
		return interpreter.NewError(code, string(mdata)), nil

	default:
		return nil, fmt.Errorf("marshal: unknown binary tag 0x%02x", tag)
	}
}

// RegisterMarshalBuiltins registers "std.marshal" and "marshal" into environment.
func RegisterMarshalBuiltins(env *interpreter.Environment) {
	mod := interpreter.NewMap()

	reg := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		mod.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// marshal.dumps(val) -> string (binary byte string)
	reg("dumps", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "marshal.dumps requires value")
		}
		var buf bytes.Buffer
		if err := serializeValue(&buf, args[0]); err != nil {
			return interpreter.NewError(500, "marshal: serialization failed: "+err.Error())
		}
		return &interpreter.StringValue{Val: buf.String()}
	})

	// marshal.loads(bytes: string) -> value
	reg("loads", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "marshal.loads requires byte string")
		}
		data := []byte(interpreter.CoerceToString(args[0]))
		reader := bytes.NewReader(data)
		val, err := deserializeValue(reader)
		if err != nil {
			return interpreter.NewError(400, "marshal: deserialization failed: "+err.Error())
		}
		return val
	})

	// marshal.dump(val, path: string) -> bool
	reg("dump", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "marshal.dump requires (val, path)")
		}
		path := interpreter.CoerceToString(args[1])
		var buf bytes.Buffer
		if err := serializeValue(&buf, args[0]); err != nil {
			return interpreter.NewError(500, "marshal: serialization failed: "+err.Error())
		}
		if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
			return interpreter.NewError(500, fmt.Sprintf("marshal: failed to write file %q: %v", path, err))
		}
		return interpreter.TRUE
	})

	// marshal.load(path: string) -> value
	reg("load", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "marshal.load requires path")
		}
		path := interpreter.CoerceToString(args[0])
		data, err := os.ReadFile(path)
		if err != nil {
			return interpreter.NewError(500, fmt.Sprintf("marshal: failed to read file %q: %v", path, err))
		}
		reader := bytes.NewReader(data)
		val, err := deserializeValue(reader)
		if err != nil {
			return interpreter.NewError(400, "marshal: deserialization failed: "+err.Error())
		}
		return val
	})

	env.Set("marshal", mod)
	env.Set("std.marshal", mod)
}
