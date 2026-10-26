// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package libtorch

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	cudaContext uintptr
)

func initCUDA() {
	cudaInitOnce.Do(func() {
		h, err := syscall.LoadLibrary("nvcuda.dll")
		if err != nil {
			cudaAvailable = false
			return
		}

		cuInit, err := syscall.GetProcAddress(h, "cuInit")
		if err != nil {
			cudaAvailable = false
			return
		}
		ret, _, _ := syscall.SyscallN(cuInit, 0)
		if ret != 0 {
			cudaAvailable = false
			return
		}

		cuDeviceGetCount, err := syscall.GetProcAddress(h, "cuDeviceGetCount")
		if err != nil {
			cudaAvailable = false
			return
		}
		var count int32
		ret, _, _ = syscall.SyscallN(cuDeviceGetCount, uintptr(unsafe.Pointer(&count)))
		if ret != 0 || count <= 0 {
			cudaAvailable = false
			return
		}

		cudaDevCount = int(count)
		cuDeviceGet, _ := syscall.GetProcAddress(h, "cuDeviceGet")
		cuDeviceGetName, _ := syscall.GetProcAddress(h, "cuDeviceGetName")

		for i := 0; i < int(count); i++ {
			var dev int32
			syscall.SyscallN(cuDeviceGet, uintptr(unsafe.Pointer(&dev)), uintptr(i))
			var nameBuf [256]byte
			syscall.SyscallN(cuDeviceGetName, uintptr(unsafe.Pointer(&nameBuf[0])), 256, uintptr(dev))
			name := byteSliceToString(nameBuf[:])
			cudaDevNames = append(cudaDevNames, name)
		}

		// Initialize primary context on device 0
		cuCtxCreate, err := syscall.GetProcAddress(h, "cuCtxCreate_v2")
		if err == nil {
			var dev0 int32
			syscall.SyscallN(cuDeviceGet, uintptr(unsafe.Pointer(&dev0)), 0)
			syscall.SyscallN(cuCtxCreate, uintptr(unsafe.Pointer(&cudaContext)), 0, uintptr(dev0))
		}

		cudaAvailable = true
	})
}

func byteSliceToString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func cudaAlloc(bytes int) (uintptr, error) {
	h, err := syscall.LoadLibrary("nvcuda.dll")
	if err != nil {
		return 0, err
	}
	cuMemAlloc, err := syscall.GetProcAddress(h, "cuMemAlloc_v2")
	if err != nil {
		return 0, err
	}
	var ptr uintptr
	ret, _, _ := syscall.SyscallN(cuMemAlloc, uintptr(unsafe.Pointer(&ptr)), uintptr(bytes))
	if ret != 0 {
		return 0, fmt.Errorf("cuMemAlloc failed with code %d", ret)
	}
	cudaTrackAlloc(bytes)
	return ptr, nil
}

func cudaFree(ptr uintptr, bytes int) error {
	if ptr == 0 {
		return nil
	}
	h, err := syscall.LoadLibrary("nvcuda.dll")
	if err != nil {
		return err
	}
	cuMemFree, err := syscall.GetProcAddress(h, "cuMemFree_v2")
	if err != nil {
		return err
	}
	ret, _, _ := syscall.SyscallN(cuMemFree, ptr)
	if ret != 0 {
		return fmt.Errorf("cuMemFree failed with code %d", ret)
	}
	cudaTrackFree(bytes)
	return nil
}

func cudaMemcpyHtoD(dst uintptr, src []float64) error {
	h, err := syscall.LoadLibrary("nvcuda.dll")
	if err != nil {
		return err
	}
	cuMemcpyHtoD, err := syscall.GetProcAddress(h, "cuMemcpyHtoD_v2")
	if err != nil {
		return err
	}
	bytes := uintptr(len(src) * 8)
	ret, _, _ := syscall.SyscallN(cuMemcpyHtoD, dst, uintptr(unsafe.Pointer(&src[0])), bytes)
	if ret != 0 {
		return fmt.Errorf("cuMemcpyHtoD failed with code %d", ret)
	}
	return nil
}

func cudaMemcpyDtoH(dst []float64, src uintptr) error {
	h, err := syscall.LoadLibrary("nvcuda.dll")
	if err != nil {
		return err
	}
	cuMemcpyDtoH, err := syscall.GetProcAddress(h, "cuMemcpyDtoH_v2")
	if err != nil {
		return err
	}
	bytes := uintptr(len(dst) * 8)
	ret, _, _ := syscall.SyscallN(cuMemcpyDtoH, uintptr(unsafe.Pointer(&dst[0])), src, bytes)
	if ret != 0 {
		return fmt.Errorf("cuMemcpyDtoH failed with code %d", ret)
	}
	return nil
}

func cudaSync() error {
	h, err := syscall.LoadLibrary("nvcuda.dll")
	if err != nil {
		return err
	}
	cuCtxSynchronize, err := syscall.GetProcAddress(h, "cuCtxSynchronize")
	if err != nil {
		return err
	}
	ret, _, _ := syscall.SyscallN(cuCtxSynchronize)
	if ret != 0 {
		return fmt.Errorf("cuCtxSynchronize failed with code %d", ret)
	}
	return nil
}
