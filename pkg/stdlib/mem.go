// Package stdlib provides the low-level memory module for the Alca programming
// language.
//
// The module hands out raw memory to a program and lets it read and write
// that memory by address. The memory a program can reach is only the memory it
// allocated itself: every block is registered with its size, and every access
// is checked against the bounds of the block. A program therefore cannot use
// the module to read or write the memory of the process, which would make it
// a way around the capability policy of the runtime.
package stdlib

import (
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// float64FromBits reads a float from its bit pattern.
func float64FromBits(bits uint64) float64 { return math.Float64frombits(bits) }

// float64Bits returns the bit pattern of a float.
func float64Bits(value float64) uint64 { return math.Float64bits(value) }

// memoryBlock is a block of raw memory a program allocated.
type memoryBlock struct {
	data []byte
}

var (
	memoryMu     sync.Mutex
	memoryBlocks = make(map[uintptr]*memoryBlock)
)

// RegisterMemBuiltins registers "std.mem" and "mem" into the interpreter environment.
func RegisterMemBuiltins(env *interpreter.Environment) {
	memModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		memModule.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// mem.alloc(size) -> address
	register("alloc", func(args ...interpreter.Value) interpreter.Value {
		size := int64(0)
		if len(args) >= 1 {
			if iv, ok := args[0].(*interpreter.IntegerValue); ok && iv.Val > 0 {
				size = iv.Val
			}
		}
		if size <= 0 {
			return interpreter.NewError(400, "mem.alloc requires a positive size")
		}
		if size > 1<<30 {
			return interpreter.NewError(400, "mem.alloc: the block is larger than one gigabyte")
		}
		block := &memoryBlock{data: make([]byte, size)}
		addr := uintptr(unsafe.Pointer(&block.data[0]))
		memoryMu.Lock()
		memoryBlocks[addr] = block
		memoryMu.Unlock()
		return &interpreter.IntegerValue{Val: int64(addr)}
	})

	// mem.free(address) -> bool
	register("free", func(args ...interpreter.Value) interpreter.Value {
		addr, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.free requires an address")
		}
		memoryMu.Lock()
		block, found := memoryBlocks[addr]
		if found {
			delete(memoryBlocks, addr)
		}
		memoryMu.Unlock()
		if !found {
			return interpreter.NewError(400, "mem.free: the address was not allocated by mem.alloc")
		}
		// The block stays alive until the call returns, so that a program cannot
		// read the memory after it was released.
		_ = block
		return interpreter.TRUE
	})

	// mem.size(address) -> int
	register("size", func(args ...interpreter.Value) interpreter.Value {
		addr, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.size requires an address")
		}
		memoryMu.Lock()
		block, found := memoryBlocks[addr]
		memoryMu.Unlock()
		if !found {
			return interpreter.NewError(400, "mem.size: the address was not allocated by mem.alloc")
		}
		return &interpreter.IntegerValue{Val: int64(len(block.data))}
	})

	// mem.zero(address, size) -> bool
	register("zero", func(args ...interpreter.Value) interpreter.Value {
		addr, size, ok := blockArgs(args, 0, 1)
		if !ok {
			return interpreter.NewError(400, "mem.zero requires an address and a size")
		}
		if !withinBounds(addr, 0, size) {
			return interpreter.NewError(400, "mem.zero: the range is outside of the block")
		}
		memoryMu.Lock()
		block := memoryBlocks[addr]
		memoryMu.Unlock()
		for i := int64(0); i < size; i++ {
			block.data[i] = 0
		}
		return interpreter.TRUE
	})

	// mem.copy(destination, source, size) -> bool
	register("copy", func(args ...interpreter.Value) interpreter.Value {
		dst, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.copy requires a destination address")
		}
		src, ok := addressArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mem.copy requires a source address")
		}
		size, ok := intArg(args, 2)
		if !ok || size < 0 {
			return interpreter.NewError(400, "mem.copy requires a size")
		}
		if !withinBounds(dst, 0, size) || !withinBounds(src, 0, size) {
			return interpreter.NewError(400, "mem.copy: the range is outside of the block")
		}
		memoryMu.Lock()
		dstBlock := memoryBlocks[dst]
		srcBlock := memoryBlocks[src]
		memoryMu.Unlock()
		copy(dstBlock.data[:size], srcBlock.data[:size])
		return interpreter.TRUE
	})

	// mem.fill(address, value, size) -> bool
	register("fill", func(args ...interpreter.Value) interpreter.Value {
		addr, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.fill requires an address")
		}
		value, ok := intArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mem.fill requires a value")
		}
		size, ok := intArg(args, 2)
		if !ok || size < 0 {
			return interpreter.NewError(400, "mem.fill requires a size")
		}
		if !withinBounds(addr, 0, size) {
			return interpreter.NewError(400, "mem.fill: the range is outside of the block")
		}
		memoryMu.Lock()
		block := memoryBlocks[addr]
		memoryMu.Unlock()
		for i := int64(0); i < size; i++ {
			block.data[i] = byte(value)
		}
		return interpreter.TRUE
	})

	// The fixed width accessors read and write a number at an offset of a
	// block. The offset is optional and defaults to zero.
	accessors := []struct {
		name  string
		size  int
		read  func(block *memoryBlock, offset int64) int64
		write func(block *memoryBlock, offset int64, value int64)
	}{
		{name: "u8", size: 1,
			read:  func(b *memoryBlock, o int64) int64 { return int64(b.data[o]) },
			write: func(b *memoryBlock, o int64, v int64) { b.data[o] = byte(v) }},
		{name: "i8", size: 1,
			read:  func(b *memoryBlock, o int64) int64 { return int64(int8(b.data[o])) },
			write: func(b *memoryBlock, o int64, v int64) { b.data[o] = byte(v) }},
		{name: "u16", size: 2,
			read: func(b *memoryBlock, o int64) int64 {
				return int64(uint16(b.data[o]) | uint16(b.data[o+1])<<8)
			},
			write: func(b *memoryBlock, o int64, v int64) {
				b.data[o] = byte(v)
				b.data[o+1] = byte(v >> 8)
			}},
		{name: "i16", size: 2,
			read: func(b *memoryBlock, o int64) int64 {
				return int64(int16(uint16(b.data[o]) | uint16(b.data[o+1])<<8))
			},
			write: func(b *memoryBlock, o int64, v int64) {
				b.data[o] = byte(v)
				b.data[o+1] = byte(v >> 8)
			}},
		{name: "u32", size: 4,
			read: func(b *memoryBlock, o int64) int64 {
				var value uint32
				for i := int64(3); i >= 0; i-- {
					value = value<<8 | uint32(b.data[o+i])
				}
				return int64(value)
			},
			write: func(b *memoryBlock, o int64, v int64) {
				for i := int64(0); i < 4; i++ {
					b.data[o+i] = byte(v >> (8 * i))
				}
			}},
		{name: "i32", size: 4,
			read: func(b *memoryBlock, o int64) int64 {
				var value uint32
				for i := int64(3); i >= 0; i-- {
					value = value<<8 | uint32(b.data[o+i])
				}
				return int64(int32(value))
			},
			write: func(b *memoryBlock, o int64, v int64) {
				for i := int64(0); i < 4; i++ {
					b.data[o+i] = byte(v >> (8 * i))
				}
			}},
		{name: "u64", size: 8,
			read: func(b *memoryBlock, o int64) int64 {
				var value uint64
				for i := int64(7); i >= 0; i-- {
					value = value<<8 | uint64(b.data[o+i])
				}
				return int64(value)
			},
			write: func(b *memoryBlock, o int64, v int64) {
				for i := int64(0); i < 8; i++ {
					b.data[o+i] = byte(v >> (8 * i))
				}
			}},
		{name: "i64", size: 8,
			read: func(b *memoryBlock, o int64) int64 {
				var value uint64
				for i := int64(7); i >= 0; i-- {
					value = value<<8 | uint64(b.data[o+i])
				}
				return int64(value)
			},
			write: func(b *memoryBlock, o int64, v int64) {
				for i := int64(0); i < 8; i++ {
					b.data[o+i] = byte(v >> (8 * i))
				}
			}},
	}

	for _, accessor := range accessors {
		size := int64(accessor.size)
		read := accessor.read
		write := accessor.write
		register("read_"+accessor.name, func(args ...interpreter.Value) interpreter.Value {
			addr, offset, ok := addressAndOffset(args)
			if !ok {
				return interpreter.NewError(400, fmt.Sprintf("mem.read_%s requires an address", accessor.name))
			}
			if !withinBounds(addr, offset, size) {
				return interpreter.NewError(400, fmt.Sprintf("mem.read_%s: the address is outside of the block", accessor.name))
			}
			memoryMu.Lock()
			block := memoryBlocks[addr]
			memoryMu.Unlock()
			return &interpreter.IntegerValue{Val: read(block, offset)}
		})
		register("write_"+accessor.name, func(args ...interpreter.Value) interpreter.Value {
			addr, offset, value, ok := addressValueAndOffset(args)
			if !ok {
				return interpreter.NewError(400, fmt.Sprintf("mem.write_%s requires an address and a value", accessor.name))
			}
			if !withinBounds(addr, offset, size) {
				return interpreter.NewError(400, fmt.Sprintf("mem.write_%s: the address is outside of the block", accessor.name))
			}
			memoryMu.Lock()
			block := memoryBlocks[addr]
			memoryMu.Unlock()
			write(block, offset, value)
			return interpreter.TRUE
		})
	}

	// mem.read_f64(address, offset?) -> float
	register("read_f64", func(args ...interpreter.Value) interpreter.Value {
		addr, offset, ok := addressAndOffset(args)
		if !ok {
			return interpreter.NewError(400, "mem.read_f64 requires an address")
		}
		if !withinBounds(addr, offset, 8) {
			return interpreter.NewError(400, "mem.read_f64: the address is outside of the block")
		}
		memoryMu.Lock()
		block := memoryBlocks[addr]
		memoryMu.Unlock()
		bits := uint64(0)
		for i := int64(7); i >= 0; i-- {
			bits = bits<<8 | uint64(block.data[offset+i])
		}
		return &interpreter.FloatValue{Val: float64FromBits(bits)}
	})

	// mem.write_f64(address, value, offset?) -> bool
	register("write_f64", func(args ...interpreter.Value) interpreter.Value {
		addr, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.write_f64 requires an address")
		}
		value, ok := floatArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mem.write_f64 requires a value")
		}
		offset := int64(0)
		if len(args) > 2 {
			raw, ok := intArg(args, 2)
			if !ok || raw < 0 {
				return interpreter.NewError(400, "mem.write_f64: the offset must not be negative")
			}
			offset = raw
		}
		if !withinBounds(addr, offset, 8) {
			return interpreter.NewError(400, "mem.write_f64: the address is outside of the block")
		}
		memoryMu.Lock()
		block := memoryBlocks[addr]
		memoryMu.Unlock()
		bits := float64Bits(value)
		for i := int64(0); i < 8; i++ {
			block.data[offset+i] = byte(bits >> (8 * i))
		}
		return interpreter.TRUE
	})

	// mem.read_bytes(address, offset?, size) -> blob
	register("read_bytes", func(args ...interpreter.Value) interpreter.Value {
		addr, offset, ok := addressAndOffset(args)
		if !ok {
			return interpreter.NewError(400, "mem.read_bytes requires an address")
		}
		size, ok := intArg(args, 2)
		if !ok || size < 0 {
			return interpreter.NewError(400, "mem.read_bytes requires a size")
		}
		if !withinBounds(addr, offset, size) {
			return interpreter.NewError(400, "mem.read_bytes: the range is outside of the block")
		}
		memoryMu.Lock()
		block := memoryBlocks[addr]
		memoryMu.Unlock()
		return &interpreter.BlobValue{Data: append([]byte(nil), block.data[offset:offset+size]...)}
	})

	// mem.write_bytes(address, data, offset?) -> bool
	register("write_bytes", func(args ...interpreter.Value) interpreter.Value {
		addr, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.write_bytes requires an address")
		}
		data, ok := blobArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mem.write_bytes requires data")
		}
		offset := int64(0)
		if len(args) > 2 {
			raw, ok := intArg(args, 2)
			if !ok || raw < 0 {
				return interpreter.NewError(400, "mem.write_bytes: the offset must not be negative")
			}
			offset = raw
		}
		if !withinBounds(addr, offset, int64(len(data))) {
			return interpreter.NewError(400, "mem.write_bytes: the range is outside of the block")
		}
		memoryMu.Lock()
		block := memoryBlocks[addr]
		memoryMu.Unlock()
		copy(block.data[offset:offset+int64(len(data))], data)
		return interpreter.TRUE
	})

	// mem.add(address, delta) -> address
	register("add", func(args ...interpreter.Value) interpreter.Value {
		addr, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.add requires an address")
		}
		delta, ok := intArg(args, 1)
		if !ok {
			return interpreter.NewError(400, "mem.add requires a delta")
		}
		return &interpreter.IntegerValue{Val: int64(addr) + delta}
	})

	// mem.is_null(address) -> bool
	register("is_null", func(args ...interpreter.Value) interpreter.Value {
		addr, ok := addressArg(args, 0)
		if !ok {
			return interpreter.NewError(400, "mem.is_null requires an address")
		}
		if addr == 0 {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	// mem.size_of(type_name) -> int
	register("size_of", func(args ...interpreter.Value) interpreter.Value {
		return &interpreter.IntegerValue{Val: memoryTypeSize(interpreter.CoerceToString(args[0]))}
	})

	env.Set("mem", memModule)
	env.Set("std.mem", memModule)
}

// addressArg reads an address argument.
func addressArg(args []interpreter.Value, index int) (uintptr, bool) {
	if index >= len(args) {
		return 0, false
	}
	iv, ok := args[index].(*interpreter.IntegerValue)
	if !ok || iv.Val <= 0 {
		return 0, false
	}
	return uintptr(iv.Val), true
}

// intArg reads an integer argument.
func intArg(args []interpreter.Value, index int) (int64, bool) {
	if index >= len(args) {
		return 0, false
	}
	iv, ok := args[index].(*interpreter.IntegerValue)
	if !ok {
		return 0, false
	}
	return iv.Val, true
}

// floatArg reads a floating point argument.
func floatArg(args []interpreter.Value, index int) (float64, bool) {
	if index >= len(args) {
		return 0, false
	}
	switch value := args[index].(type) {
	case *interpreter.FloatValue:
		return value.Val, true
	case *interpreter.IntegerValue:
		return float64(value.Val), true
	default:
		return 0, false
	}
}

// blobArg reads a byte string argument.
func blobArg(args []interpreter.Value, index int) ([]byte, bool) {
	if index >= len(args) {
		return nil, false
	}
	switch value := args[index].(type) {
	case *interpreter.BlobValue:
		return value.Data, true
	case *interpreter.StringValue:
		return []byte(value.Val), true
	default:
		return nil, false
	}
}

// addressAndOffset reads an address and an optional offset.
func addressAndOffset(args []interpreter.Value) (uintptr, int64, bool) {
	addr, ok := addressArg(args, 0)
	if !ok {
		return 0, 0, false
	}
	offset := int64(0)
	if len(args) > 1 {
		value, ok := intArg(args, 1)
		if !ok || value < 0 {
			return 0, 0, false
		}
		offset = value
	}
	return addr, offset, true
}

// addressValueAndOffset reads an address, a value and an optional offset.
func addressValueAndOffset(args []interpreter.Value) (uintptr, int64, int64, bool) {
	addr, ok := addressArg(args, 0)
	if !ok {
		return 0, 0, 0, false
	}
	value, ok := intArg(args, 1)
	if !ok {
		return 0, 0, 0, false
	}
	offset := int64(0)
	if len(args) > 2 {
		raw, ok := intArg(args, 2)
		if !ok || raw < 0 {
			return 0, 0, 0, false
		}
		offset = raw
	}
	return addr, offset, value, true
}

// blockArgs reads an address and a size.
func blockArgs(args []interpreter.Value, addrIndex, sizeIndex int) (uintptr, int64, bool) {
	addr, ok := addressArg(args, addrIndex)
	if !ok {
		return 0, 0, false
	}
	size, ok := intArg(args, sizeIndex)
	if !ok || size < 0 {
		return 0, 0, false
	}
	return addr, size, true
}

// withinBounds reports whether a range lies inside the block of an address.
func withinBounds(addr uintptr, offset, size int64) bool {
	memoryMu.Lock()
	block, found := memoryBlocks[addr]
	memoryMu.Unlock()
	if !found {
		return false
	}
	if offset < 0 || size < 0 {
		return false
	}
	return offset+size <= int64(len(block.data))
}

// memoryTypeSize returns the size of a C type in bytes.
func memoryTypeSize(name string) int64 {
	switch name {
	case "byte", "char", "int8", "uint8", "bool":
		return 1
	case "short", "int16", "uint16":
		return 2
	case "int", "int32", "uint32", "float":
		return 4
	default:
		return 8
	}
}
