// Package compiler packages Alca programs into standalone executables.
//
// An executable is: [runtime stub for the target OS/arch] + [payload] + [footer].
// The payload is either serialized VM bytecode (default; no source code is shipped) or,
// for the few programs the VM cannot compile, the source plus imported modules. Explicit
// non-source resources are packaged alongside either payload kind.
// The stub is the Alca runtime built for the *target* platform, which is what makes
// cross-compilation possible without any toolchain on the build machine.
package compiler

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/alca-lang/alca/pkg/ast"
	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/token"
	"github.com/alca-lang/alca/pkg/vm"
)

// Kind is the payload flavour.
type Kind byte

const (
	KindBytecode Kind = 'B'
	KindSource   Kind = 'S'
	KindNative   Kind = 'N' // Ahead-of-Time native machine code
)

// Bundle is the decoded content of an executable's payload.
type Bundle struct {
	Kind      Kind
	Name      string            // entry file name (for diagnostics)
	Source    string            // KindSource: entry program text
	Files     map[string]string // KindSource: imported modules (relative path -> text)
	Resources map[string][]byte // Embedded non-source files (relative path -> contents)
	// Modules maps a `use` module path to its relative path inside Files, so
	// the standalone runtime resolves `use pkg` from the embedded sources
	// instead of the machine's registry.
	Modules      map[string]string
	Code         *vm.Bytecode // KindBytecode or KindNative constants/fallback
	NativeCode   []byte       // KindNative: AOT compiled AMD64 machine code
	LabelOffsets map[int]int  // KindNative: Bytecode IP -> machine code offset
}

const (
	tagInt byte = iota + 1
	tagFloat
	tagString
	tagBool
	tagNull
	tagFunc
	tagStruct
	tagUse
)

type writer struct{ bytes.Buffer }

func (w *writer) u8(v byte) { w.WriteByte(v) }
func (w *writer) u32(v int) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	w.Write(b[:])
}
func (w *writer) u64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	w.Write(b[:])
}
func (w *writer) str(s string) { w.u32(len(s)); w.WriteString(s) }
func (w *writer) bytes(b []byte) {
	w.u32(len(b))
	w.Write(b)
}

type reader struct {
	b   []byte
	pos int
	err error
}

func (r *reader) need(n int) bool {
	if r.err != nil || r.pos+n > len(r.b) || n < 0 {
		if r.err == nil {
			r.err = fmt.Errorf("corrupted payload: unexpected end of data")
		}
		return false
	}
	return true
}
func (r *reader) u8() byte {
	if !r.need(1) {
		return 0
	}
	v := r.b[r.pos]
	r.pos++
	return v
}
func (r *reader) u32() int {
	if !r.need(4) {
		return 0
	}
	v := binary.LittleEndian.Uint32(r.b[r.pos:])
	r.pos += 4
	return int(v)
}
func (r *reader) u64() uint64 {
	if !r.need(8) {
		return 0
	}
	v := binary.LittleEndian.Uint64(r.b[r.pos:])
	r.pos += 8
	return v
}
func (r *reader) str() string {
	n := r.u32()
	if !r.need(n) {
		return ""
	}
	s := string(r.b[r.pos : r.pos+n])
	r.pos += n
	return s
}
func (r *reader) rawBytes() []byte {
	n := r.u32()
	if !r.need(n) {
		return nil
	}
	out := make([]byte, n)
	copy(out, r.b[r.pos:r.pos+n])
	r.pos += n
	return out
}

// ---------------------------------------------------------------------------
// Bytecode (de)serialization
// ---------------------------------------------------------------------------

func encodeLiteral(w *writer, e ast.Expression) error {
	switch x := e.(type) {
	case nil, *ast.NullLiteral:
		w.u8(tagNull)
	case *ast.IntegerLiteral:
		w.u8(tagInt)
		w.u64(uint64(x.Value))
	case *ast.FloatLiteral:
		w.u8(tagFloat)
		w.u64(math.Float64bits(x.Value))
	case *ast.StringLiteral:
		w.u8(tagString)
		w.str(x.Value)
	case *ast.BooleanLiteral:
		w.u8(tagBool)
		if x.Value {
			w.u8(1)
		} else {
			w.u8(0)
		}
	case *ast.PrefixExpression:
		switch lit := x.Right.(type) {
		case *ast.IntegerLiteral:
			if x.Operator == "-" {
				w.u8(tagInt)
				w.u64(uint64(-lit.Value))
				return nil
			}
		case *ast.FloatLiteral:
			if x.Operator == "-" {
				w.u8(tagFloat)
				w.u64(math.Float64bits(-lit.Value))
				return nil
			}
		}
		return fmt.Errorf("struct field default is not a literal")
	default:
		return fmt.Errorf("struct field default is not a literal")
	}
	return nil
}

func decodeLiteral(r *reader) ast.Expression {
	switch r.u8() {
	case tagInt:
		return &ast.IntegerLiteral{Value: int64(r.u64())}
	case tagFloat:
		return &ast.FloatLiteral{Value: math.Float64frombits(r.u64())}
	case tagString:
		return &ast.StringLiteral{Value: r.str()}
	case tagBool:
		return &ast.BooleanLiteral{Value: r.u8() == 1}
	}
	return nil
}

func encodeConstant(w *writer, v interpreter.Value) error {
	switch c := v.(type) {
	case *interpreter.IntegerValue:
		w.u8(tagInt)
		w.u64(uint64(c.Val))
	case *interpreter.FloatValue:
		w.u8(tagFloat)
		w.u64(math.Float64bits(c.Val))
	case *interpreter.StringValue:
		w.u8(tagString)
		w.str(c.Val)
	case *interpreter.BoolValue:
		w.u8(tagBool)
		if c.Val {
			w.u8(1)
		} else {
			w.u8(0)
		}
	case *interpreter.NullValue:
		w.u8(tagNull)
	case *vm.CompiledFunction:
		w.u8(tagFunc)
		w.str(c.Name)
		w.u32(c.NumLocals)
		w.u32(c.NumParameters)
		w.u32(c.NumFree)
		for _, d := range c.UpvalueDescs {
			if d.IsLocal {
				w.u8(1)
			} else {
				w.u8(0)
			}
			w.u32(d.Index)
		}
		w.bytes(c.Instructions)
	case *interpreter.StructDef:
		w.u8(tagStruct)
		w.str(c.Name)
		w.u32(len(c.Fields))
		for _, f := range c.Fields {
			w.str(f.Name)
			var types []string
			if f.TypeAnnotation != nil {
				types = f.TypeAnnotation.Types
			}
			w.u32(len(types))
			for _, t := range types {
				w.str(t)
			}
			if err := encodeLiteral(w, f.DefaultValue); err != nil {
				return fmt.Errorf("struct %s field %s: %w", c.Name, f.Name, err)
			}
		}
	case *vm.UseStatementValue:
		w.u8(tagUse)
		w.str(c.Stmt.ModulePath)
		w.str(c.Stmt.Alias)
		w.u32(len(c.Stmt.Imports))
		for _, im := range c.Stmt.Imports {
			w.str(im)
		}
	default:
		return fmt.Errorf("constant of type %T cannot be serialized", v)
	}
	return nil
}

func decodeConstant(r *reader) interpreter.Value {
	switch r.u8() {
	case tagInt:
		return &interpreter.IntegerValue{Val: int64(r.u64())}
	case tagFloat:
		return &interpreter.FloatValue{Val: math.Float64frombits(r.u64())}
	case tagString:
		return &interpreter.StringValue{Val: r.str()}
	case tagBool:
		if r.u8() == 1 {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	case tagNull:
		return interpreter.NULL
	case tagFunc:
		name := r.str()
		locals := r.u32()
		params := r.u32()
		numFree := r.u32()
		var descs []vm.UpvalueDesc
		for i := 0; i < numFree && r.err == nil; i++ {
			isLocal := r.u8() == 1
			descs = append(descs, vm.UpvalueDesc{IsLocal: isLocal, Index: r.u32()})
		}
		return &vm.CompiledFunction{
			Name:          name,
			NumLocals:     locals,
			NumParameters: params,
			NumFree:       numFree,
			UpvalueDescs:  descs,
			Instructions:  r.rawBytes(),
		}
	case tagStruct:
		def := &interpreter.StructDef{Name: r.str()}
		n := r.u32()
		for i := 0; i < n && r.err == nil; i++ {
			f := &ast.StructField{Name: r.str()}
			nt := r.u32()
			if nt > 0 {
				ta := &ast.TypeAnnotation{Token: token.Token{Literal: ""}}
				for j := 0; j < nt && r.err == nil; j++ {
					ta.Types = append(ta.Types, r.str())
				}
				f.TypeAnnotation = ta
			}
			f.DefaultValue = decodeLiteral(r)
			def.Fields = append(def.Fields, f)
		}
		return def
	case tagUse:
		u := &ast.UseStatement{ModulePath: r.str(), Alias: r.str()}
		n := r.u32()
		for i := 0; i < n && r.err == nil; i++ {
			u.Imports = append(u.Imports, r.str())
		}
		return &vm.UseStatementValue{Stmt: u}
	}
	if r.err == nil {
		r.err = fmt.Errorf("corrupted payload: unknown constant tag")
	}
	return interpreter.NULL
}

// ---------------------------------------------------------------------------
// Payload envelope
// ---------------------------------------------------------------------------

// Encode serializes the bundle into the (compressed, checksummed) payload envelope.
func (b *Bundle) Encode() ([]byte, error) {
	var raw writer
	raw.u8(byte(b.Kind))
	raw.str(b.Name)
	switch b.Kind {
	case KindNative:
		raw.bytes(b.NativeCode)
		raw.u32(len(b.LabelOffsets))
		for ip, off := range b.LabelOffsets {
			raw.u32(ip)
			raw.u32(off)
		}
		raw.bytes(b.Code.Instructions)
		raw.u32(len(b.Code.Constants))
		for _, c := range b.Code.Constants {
			if err := encodeConstant(&raw, c); err != nil {
				return nil, err
			}
		}
	case KindBytecode:
		raw.bytes(b.Code.Instructions)
		raw.u32(len(b.Code.Constants))
		for _, c := range b.Code.Constants {
			if err := encodeConstant(&raw, c); err != nil {
				return nil, err
			}
		}
	case KindSource:
		raw.str(b.Source)
		raw.u32(len(b.Files))
		for _, k := range sortedKeys(b.Files) {
			raw.str(k)
			raw.str(b.Files[k])
		}
		raw.u32(len(b.Modules))
		for _, k := range sortedKeys(b.Modules) {
			raw.str(k)
			raw.str(b.Modules[k])
		}
	default:
		return nil, fmt.Errorf("unknown bundle kind %q", b.Kind)
	}
	raw.u32(len(b.Resources))
	for _, k := range sortedByteKeys(b.Resources) {
		raw.str(k)
		raw.bytes(b.Resources[k])
	}

	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		return nil, fmt.Errorf("failed to compress payload: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize compression: %w", err)
	}
	sum := sha256.Sum256(compressed.Bytes())

	var out writer
	var ver [4]byte
	binary.LittleEndian.PutUint32(ver[:], CurrentPayloadVersion)
	out.Write(ver[:])
	out.Write(sum[:])
	out.u64(uint64(raw.Len()))
	out.Write(compressed.Bytes())
	return out.Bytes(), nil
}

// DecodeBundle validates and decodes a payload envelope.
func DecodeBundle(data []byte) (*Bundle, error) {
	const header = 4 + 32 + 8
	if len(data) < header {
		return nil, fmt.Errorf("corrupted payload: too short (%d bytes)", len(data))
	}
	if v := binary.LittleEndian.Uint32(data[:4]); v != CurrentPayloadVersion {
		return nil, fmt.Errorf("unsupported payload version %d (this runtime supports %d); rebuild the program with a matching alca", v, CurrentPayloadVersion)
	}
	var want [32]byte
	copy(want[:], data[4:36])
	comp := data[header:]
	if got := sha256.Sum256(comp); got != want {
		return nil, fmt.Errorf("payload integrity check failed (checksum mismatch)")
	}
	zr, err := gzip.NewReader(bytes.NewReader(comp))
	if err != nil {
		return nil, fmt.Errorf("failed to decompress payload: %w", err)
	}
	defer zr.Close()
	rawBytes, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("decompression error: %w", err)
	}

	r := &reader{b: rawBytes}
	b := &Bundle{Kind: Kind(r.u8()), Name: r.str()}
	switch b.Kind {
	case KindNative:
		b.NativeCode = r.rawBytes()
		numOffsets := r.u32()
		b.LabelOffsets = make(map[int]int, numOffsets)
		for i := 0; i < numOffsets && r.err == nil; i++ {
			ip := r.u32()
			off := r.u32()
			b.LabelOffsets[ip] = off
		}
		code := &vm.Bytecode{Instructions: r.rawBytes()}
		n := r.u32()
		for i := 0; i < n && r.err == nil; i++ {
			code.Constants = append(code.Constants, decodeConstant(r))
		}
		b.Code = code
	case KindBytecode:
		code := &vm.Bytecode{Instructions: r.rawBytes()}
		n := r.u32()
		for i := 0; i < n && r.err == nil; i++ {
			code.Constants = append(code.Constants, decodeConstant(r))
		}
		b.Code = code
	case KindSource:
		b.Source = r.str()
		n := r.u32()
		b.Files = map[string]string{}
		for i := 0; i < n && r.err == nil; i++ {
			k := r.str()
			b.Files[k] = r.str()
		}
		m := r.u32()
		b.Modules = map[string]string{}
		for i := 0; i < m && r.err == nil; i++ {
			k := r.str()
			b.Modules[k] = r.str()
		}
	default:
		return nil, fmt.Errorf("corrupted payload: unknown kind %q", b.Kind)
	}
	nResources := r.u32()
	b.Resources = make(map[string][]byte, nResources)
	for i := 0; i < nResources && r.err == nil; i++ {
		k := r.str()
		b.Resources[k] = r.rawBytes()
	}
	if r.err != nil {
		return nil, r.err
	}
	return b, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func sortedByteKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
