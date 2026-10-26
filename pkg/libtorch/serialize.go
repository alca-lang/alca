// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

const serializeMagic = "ALCALT"
const serializeVersion = byte(1)

// Save writes a named map of tensors to path in a compact little-endian binary
// format. The result can be read back with Load.
func Save(path string, tensors map[string]*Tensor) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Encode(f, tensors)
}

// Encode serializes a named map of tensors to w.
func Encode(w io.Writer, tensors map[string]*Tensor) error {
	bw := binaryWriter{w: w}
	bw.writeString(serializeMagic)
	bw.writeByte(serializeVersion)
	bw.writeUint32(uint32(len(tensors)))
	for name, t := range tensors {
		bw.writeString(name)
		bw.writeUint32(uint32(t.Dim()))
		for _, d := range t.shape {
			bw.writeUint32(uint32(d))
		}
		bw.writeUint32(uint32(len(t.data)))
		for _, v := range t.data {
			bw.writeFloat64(v)
		}
	}
	return bw.err
}

// Load reads tensors serialized by Save from path.
func Load(path string) (map[string]*Tensor, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f)
}

// Decode deserializes a named map of tensors from r.
func Decode(r io.Reader) (map[string]*Tensor, error) {
	br := binaryReader{r: r}
	magic := br.readString()
	if magic != serializeMagic {
		return nil, fmt.Errorf("libtorch: not a libtorch tensor file (bad magic %q)", magic)
	}
	version := br.readByte()
	if version != serializeVersion {
		return nil, fmt.Errorf("libtorch: unsupported tensor file version %d", version)
	}
	count := int(br.readUint32())
	out := make(map[string]*Tensor, count)
	for i := 0; i < count; i++ {
		name := br.readString()
		rank := int(br.readUint32())
		shape := make([]int, rank)
		for d := 0; d < rank; d++ {
			shape[d] = int(br.readUint32())
		}
		n := int(br.readUint32())
		data := make([]float64, n)
		for j := 0; j < n; j++ {
			data[j] = br.readFloat64()
		}
		if br.err != nil {
			return nil, br.err
		}
		out[name] = &Tensor{data: data, shape: shape}
	}
	return out, br.err
}

// StateDict builds a named map from a module's parameters, naming them
// "prefix.0", "prefix.1", ... (or "p0", "p1" when prefix is empty).
func StateDict(prefix string, params []*Tensor) map[string]*Tensor {
	out := make(map[string]*Tensor, len(params))
	for i, p := range params {
		name := fmt.Sprintf("p%d", i)
		if prefix != "" {
			name = fmt.Sprintf("%s.%d", prefix, i)
		}
		out[name] = p
	}
	return out
}

// LoadInto copies loaded tensor values into the destination parameters by
// matching names, resizing where necessary. It is a convenience for restoring a
// saved state dict into an existing model whose parameter order is stable.
func LoadInto(dst map[string]*Tensor, src map[string]*Tensor) error {
	for name, d := range dst {
		s, ok := src[name]
		if !ok {
			return fmt.Errorf("libtorch: missing tensor %q in state dict", name)
		}
		if !equalInts(d.shape, s.shape) {
			return fmt.Errorf("libtorch: shape mismatch for %q: dst %v vs src %v", name, d.shape, s.shape)
		}
		copy(d.data, s.data)
	}
	return nil
}

// binaryWriter wraps an io.Writer with little-endian primitives, recording the
// first error encountered.
type binaryWriter struct {
	w   io.Writer
	err error
	buf [8]byte
}

func (b *binaryWriter) writeByte(v byte) {
	if b.err != nil {
		return
	}
	b.buf[0] = v
	_, b.err = b.w.Write(b.buf[:1])
}

func (b *binaryWriter) writeUint32(v uint32) {
	if b.err != nil {
		return
	}
	binary.LittleEndian.PutUint32(b.buf[:4], v)
	_, b.err = b.w.Write(b.buf[:4])
}

func (b *binaryWriter) writeFloat64(v float64) {
	if b.err != nil {
		return
	}
	binary.LittleEndian.PutUint64(b.buf[:8], math.Float64bits(v))
	_, b.err = b.w.Write(b.buf[:8])
}

func (b *binaryWriter) writeString(s string) {
	if b.err != nil {
		return
	}
	b.writeUint32(uint32(len(s)))
	_, b.err = b.w.Write([]byte(s))
}

// binaryReader wraps an io.Reader with little-endian primitives, recording the
// first error encountered.
type binaryReader struct {
	r   io.Reader
	err error
	buf [8]byte
}

func (b *binaryReader) readByte() byte {
	if b.err != nil {
		return 0
	}
	_, b.err = io.ReadFull(b.r, b.buf[:1])
	return b.buf[0]
}

func (b *binaryReader) readUint32() uint32 {
	if b.err != nil {
		return 0
	}
	_, b.err = io.ReadFull(b.r, b.buf[:4])
	return binary.LittleEndian.Uint32(b.buf[:4])
}

func (b *binaryReader) readFloat64() float64 {
	if b.err != nil {
		return 0
	}
	_, b.err = io.ReadFull(b.r, b.buf[:8])
	return math.Float64frombits(binary.LittleEndian.Uint64(b.buf[:8]))
}

func (b *binaryReader) readString() string {
	if b.err != nil {
		return ""
	}
	n := int(b.readUint32())
	if b.err != nil {
		return ""
	}
	s := make([]byte, n)
	_, b.err = io.ReadFull(b.r, s)
	return string(s)
}
