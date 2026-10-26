// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package mtproto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// The identifiers of the primitive types of the protocol language.
const (
	TLBoolFalse uint32 = 0xbc799737
	TLBoolTrue  uint32 = 0x997275b5
	TLInt32     uint32 = 0x4cb45f26
	TLInt64     uint32 = 0x8af93e40
	TLInt128    uint32 = 0xcb7f1d6f
	TLBytes     uint32 = 0x5be0e191
	TLString    uint32 = 0x2e2a1a1d
	TLVector    uint32 = 0x1cb5c415
	TLNull      uint32 = 0x5678b1b6
	TLBool      uint32 = 0xc4b9f9bb
)

// ErrTLFormat is returned when data does not follow the protocol language.
var ErrTLFormat = errors.New("mtproto: the data does not follow the protocol language")

// Writer serializes values of the protocol language.
type Writer struct {
	buf []byte
}

// NewWriter returns a writer for the protocol language.
func NewWriter() *Writer { return &Writer{} }

// Bytes returns the serialized data.
func (w *Writer) Bytes() []byte { return w.buf }

// Len returns the number of serialized bytes.
func (w *Writer) Len() int { return len(w.buf) }

// WriteInt32 writes a 32-bit integer.
func (w *Writer) WriteInt32(value int32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(value))
	w.buf = append(w.buf, buf[:]...)
}

// WriteInt64 writes a 64-bit integer.
func (w *Writer) WriteInt64(value int64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(value))
	w.buf = append(w.buf, buf[:]...)
}

// WriteInt128 writes a 128-bit integer.
func (w *Writer) WriteInt128(hi, lo int64) {
	w.WriteInt64(lo)
	w.WriteInt64(hi)
}

// WriteUInt32 writes an unsigned 32-bit integer.
func (w *Writer) WriteUInt32(value uint32) { w.WriteInt32(int32(value)) }

// WriteBool writes a boolean.
func (w *Writer) WriteBool(value bool) {
	if value {
		w.WriteUInt32(TLBoolTrue)
		return
	}
	w.WriteUInt32(TLBoolFalse)
}

// WriteBytes writes a byte string with its length prefix and the padding the
// protocol requires.
func (w *Writer) WriteBytes(data []byte) {
	w.WriteInt32(int32(len(data)))
	w.buf = append(w.buf, data...)
	for w.Len()%4 != 0 {
		w.buf = append(w.buf, 0)
	}
}

// WriteString writes a string, which the protocol encodes like a byte string.
func (w *Writer) WriteString(value string) { w.WriteBytes([]byte(value)) }

// WriteRaw writes bytes without a length prefix.
func (w *Writer) WriteRaw(data []byte) { w.buf = append(w.buf, data...) }

// WriteVector writes the length of a vector followed by its elements.
func (w *Writer) WriteVector(count int) { w.WriteInt32(int32(count)) }

// Reader reads values of the protocol language.
type Reader struct {
	buf []byte
	pos int
}

// NewReader returns a reader for the protocol language.
func NewReader(data []byte) *Reader { return &Reader{buf: data} }

// Remaining returns the bytes that were not read yet.
func (r *Reader) Remaining() []byte {
	if r.pos >= len(r.buf) {
		return nil
	}
	return r.buf[r.pos:]
}

// ReadInt32 reads a 32-bit integer.
func (r *Reader) ReadInt32() (int32, error) {
	if r.pos+4 > len(r.buf) {
		return 0, ErrTLFormat
	}
	value := int32(binary.LittleEndian.Uint32(r.buf[r.pos:]))
	r.pos += 4
	return value, nil
}

// ReadInt64 reads a 64-bit integer.
func (r *Reader) ReadInt64() (int64, error) {
	if r.pos+8 > len(r.buf) {
		return 0, ErrTLFormat
	}
	value := int64(binary.LittleEndian.Uint64(r.buf[r.pos:]))
	r.pos += 8
	return value, nil
}

// ReadInt128 reads a 128-bit integer.
func (r *Reader) ReadInt128() (hi, lo int64, err error) {
	lo, err = r.ReadInt64()
	if err != nil {
		return 0, 0, err
	}
	hi, err = r.ReadInt64()
	if err != nil {
		return 0, 0, err
	}
	return hi, lo, nil
}

// ReadUInt32 reads an unsigned 32-bit integer.
func (r *Reader) ReadUInt32() (uint32, error) {
	value, err := r.ReadInt32()
	return uint32(value), err
}

// ReadBool reads a boolean.
func (r *Reader) ReadBool() (bool, error) {
	value, err := r.ReadUInt32()
	if err != nil {
		return false, err
	}
	switch value {
	case TLBoolTrue:
		return true, nil
	case TLBoolFalse:
		return false, nil
	default:
		return false, fmt.Errorf("%w: unknown boolean value %d", ErrTLFormat, value)
	}
}

// ReadBytes reads a byte string.
func (r *Reader) ReadBytes() ([]byte, error) {
	length, err := r.ReadInt32()
	if err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, ErrTLFormat
	}
	if r.pos+int(length) > len(r.buf) {
		return nil, ErrTLFormat
	}
	data := append([]byte(nil), r.buf[r.pos:r.pos+int(length)]...)
	r.pos += int(length)
	// The padding aligns the next value to a multiple of four.
	for r.pos%4 != 0 {
		if r.pos >= len(r.buf) {
			return nil, ErrTLFormat
		}
		r.pos++
	}
	return data, nil
}

// ReadString reads a string.
func (r *Reader) ReadString() (string, error) {
	data, err := r.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadVectorLength reads the length of a vector.
func (r *Reader) ReadVectorLength() (int, error) {
	value, err := r.ReadInt32()
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, ErrTLFormat
	}
	return int(value), nil
}

// ReadTypeID reads the identifier of a type.
func (r *Reader) ReadTypeID() (uint32, error) { return r.ReadUInt32() }

// MaxVectorLength bounds the length of a vector, so that a malformed message
// cannot make the reader allocate without limit.
const MaxVectorLength = 1 << 20

// CheckVectorLength reports whether a vector length is within the bound.
func CheckVectorLength(length int) error {
	if length < 0 || length > MaxVectorLength {
		return fmt.Errorf("%w: the vector length %d is out of range", ErrTLFormat, length)
	}
	return nil
}

// Abs returns the absolute value of an integer.
func Abs(value int64) int64 {
	if value < 0 {
		if value == math.MinInt64 {
			return math.MaxInt64
		}
		return -value
	}
	return value
}
