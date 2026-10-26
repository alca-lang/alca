// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch

import (
	"fmt"
	"strings"
	"sync"
)

// Device represents the hardware compute device for a tensor ("cpu" or "cuda" / "cuda:0").
type Device string

const (
	DeviceCPU  Device = "cpu"
	DeviceCUDA Device = "cuda"
)

// NormalizeDevice parses a device string into a canonical Device.
func NormalizeDevice(d string) (Device, error) {
	d = strings.ToLower(strings.TrimSpace(d))
	if d == "" || d == "cpu" {
		return DeviceCPU, nil
	}
	if d == "cuda" || strings.HasPrefix(d, "cuda:") {
		return Device(d), nil
	}
	return "", fmt.Errorf("libtorch: unknown device %q (supported: 'cpu', 'cuda', 'cuda:X')", d)
}

// Device returns the tensor's current location ("cpu" or "cuda").
func (t *Tensor) Device() Device {
	if t.device == "" {
		return DeviceCPU
	}
	return t.device
}

// IsCUDA returns true if the tensor data resides on an NVIDIA CUDA device.
func (t *Tensor) IsCUDA() bool {
	return strings.HasPrefix(string(t.Device()), "cuda")
}

// To transfers the tensor to the requested device ("cpu" or "cuda").
func (t *Tensor) To(target Device) (*Tensor, error) {
	target, err := NormalizeDevice(string(target))
	if err != nil {
		return nil, err
	}
	if t.Device() == target {
		return t, nil
	}
	if target == DeviceCPU {
		return t.CPU(), nil
	}
	return t.CUDA()
}

// CPU transfers or returns the tensor on the host CPU.
func (t *Tensor) CPU() *Tensor {
	if !t.IsCUDA() {
		return t
	}
	// Copy device memory back to host slice
	hostData := make([]float64, len(t.data))
	if t.gpuPtr != 0 {
		if err := cudaMemcpyDtoH(hostData, t.gpuPtr); err != nil {
			panic(fmt.Sprintf("libtorch: failed to copy CUDA tensor to CPU: %v", err))
		}
	} else {
		copy(hostData, t.data)
	}
	out := &Tensor{
		data:         hostData,
		shape:        cloneInts(t.shape),
		device:       DeviceCPU,
		requiresGrad: t.requiresGrad,
	}
	return out
}

// CUDA transfers the tensor to NVIDIA GPU memory.
func (t *Tensor) CUDA() (*Tensor, error) {
	if !CUDAAvailable() {
		return nil, fmt.Errorf("libtorch: CUDA is not available on this system (requires NVIDIA GPU and CUDA driver)")
	}
	if t.IsCUDA() && t.gpuPtr != 0 {
		return t, nil
	}

	bytes := len(t.data) * 8
	devPtr, err := cudaAlloc(bytes)
	if err != nil {
		return nil, fmt.Errorf("libtorch: CUDA out of memory or allocation failed: %w", err)
	}
	if err := cudaMemcpyHtoD(devPtr, t.data); err != nil {
		cudaFree(devPtr, bytes)
		return nil, fmt.Errorf("libtorch: CUDA host-to-device copy failed: %w", err)
	}

	out := &Tensor{
		data:         slicesClone(t.data), // keep shadow copy for quick read
		shape:        cloneInts(t.shape),
		device:       DeviceCUDA,
		gpuPtr:       devPtr,
		requiresGrad: t.requiresGrad,
	}
	return out, nil
}

var (
	cudaInitOnce  sync.Once
	cudaAvailable bool
	cudaDevCount  int
	cudaDevNames  []string
	cudaMemUsed   int64
	cudaMemMu     sync.Mutex
)

// CUDAAvailable returns true if an NVIDIA GPU and driver are present and initialized.
func CUDAAvailable() bool {
	initCUDA()
	return cudaAvailable
}

// CUDADeviceCount returns the number of accessible CUDA-capable GPUs.
func CUDADeviceCount() int {
	initCUDA()
	return cudaDevCount
}

// CUDADeviceName returns the marketing name of the specified CUDA device.
func CUDADeviceName(deviceIndex int) (string, error) {
	initCUDA()
	if !cudaAvailable || deviceIndex < 0 || deviceIndex >= len(cudaDevNames) {
		return "", fmt.Errorf("libtorch: invalid CUDA device index %d (available: %d)", deviceIndex, cudaDevCount)
	}
	return cudaDevNames[deviceIndex], nil
}

// CUDAMemoryAllocated returns the total bytes of GPU memory currently allocated by libtorch.
func CUDAMemoryAllocated() int64 {
	cudaMemMu.Lock()
	defer cudaMemMu.Unlock()
	return cudaMemUsed
}

// CUDASynchronize waits for all concurrent CUDA operations to complete.
func CUDASynchronize() error {
	if !CUDAAvailable() {
		return nil
	}
	return cudaSync()
}

func cudaTrackAlloc(bytes int) {
	cudaMemMu.Lock()
	cudaMemUsed += int64(bytes)
	cudaMemMu.Unlock()
}

func cudaTrackFree(bytes int) {
	cudaMemMu.Lock()
	cudaMemUsed -= int64(bytes)
	if cudaMemUsed < 0 {
		cudaMemUsed = 0
	}
	cudaMemMu.Unlock()
}
