// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package libtorch_test

import (
	"strings"
	"testing"

	"github.com/alca-lang/alca/pkg/libtorch"
)

func TestCUDADeviceQueries(t *testing.T) {
	avail := libtorch.CUDAAvailable()
	count := libtorch.CUDADeviceCount()

	if !avail && count > 0 {
		t.Fatalf("CUDA not available but device count is %d", count)
	}

	tensor := libtorch.Zeros(2, 3)
	if tensor.Device() != libtorch.DeviceCPU {
		t.Fatalf("expected tensor default device to be cpu, got %s", tensor.Device())
	}
	if tensor.IsCUDA() {
		t.Fatal("expected IsCUDA to be false for CPU tensor")
	}

	cpuTensor := tensor.CPU()
	if cpuTensor.Device() != libtorch.DeviceCPU {
		t.Fatalf("expected CPU() to return cpu device, got %s", cpuTensor.Device())
	}

	toCPUTensor, err := tensor.To("cpu")
	if err != nil {
		t.Fatalf("To('cpu') failed: %v", err)
	}
	if toCPUTensor.Device() != libtorch.DeviceCPU {
		t.Fatalf("expected To('cpu') to return cpu device, got %s", toCPUTensor.Device())
	}

	if !avail {
		_, err := tensor.CUDA()
		if err == nil {
			t.Fatal("expected CUDA() to fail when CUDA is not available")
		}
		if !strings.Contains(err.Error(), "CUDA is not available") {
			t.Fatalf("unexpected error message: %v", err)
		}

		_, err = tensor.To("cuda")
		if err == nil {
			t.Fatal("expected To('cuda') to fail when CUDA is not available")
		}
	} else {
		cudaTensor, err := tensor.CUDA()
		if err != nil {
			t.Fatalf("CUDA() failed on available device: %v", err)
		}
		if !cudaTensor.IsCUDA() {
			t.Fatal("expected IsCUDA to be true on CUDA tensor")
		}
		backToCPU := cudaTensor.CPU()
		if backToCPU.IsCUDA() {
			t.Fatal("expected CPU() to return non-CUDA tensor")
		}
	}
}
