// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

var (
	user32Img = syscall.NewLazyDLL("user32.dll")
	gdi32Img  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC               = user32Img.NewProc("GetDC")
	procReleaseDC           = user32Img.NewProc("ReleaseDC")
	procGetSystemMetricsImg = user32Img.NewProc("GetSystemMetrics")
	procCreateCompatibleDC  = gdi32Img.NewProc("CreateCompatibleDC")
	procCreateDIBSection    = gdi32Img.NewProc("CreateDIBSection")
	procSelectObject        = gdi32Img.NewProc("SelectObject")
	procBitBlt              = gdi32Img.NewProc("BitBlt")
	procDeleteDC            = gdi32Img.NewProc("DeleteDC")
	procDeleteObject        = gdi32Img.NewProc("DeleteObject")
)

const (
	smCxScreen = 0
	smCyScreen = 1
	srccopy    = 0x00CC0020
	biRgb      = 0
)

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

func captureScreen(x, y, w, h int) (*image.RGBA, error) {
	if procGetDC.Find() != nil || procBitBlt.Find() != nil || procCreateDIBSection.Find() != nil {
		return nil, fmt.Errorf("GDI functions not available")
	}

	rW, _, _ := procGetSystemMetricsImg.Call(smCxScreen)
	rH, _, _ := procGetSystemMetricsImg.Call(smCyScreen)
	screenWidth := int(rW)
	screenHeight := int(rH)

	if w <= 0 {
		w = screenWidth - x
	}
	if h <= 0 {
		h = screenHeight - y
	}
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid capture dimensions (%dx%d)", w, h)
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, hdcScreen)

	rMem, _, _ := procCreateCompatibleDC.Call(0)
	if rMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	hdcMem := uintptr(uint32(rMem))
	defer procDeleteDC.Call(hdcMem)

	var bi bitmapInfoHeader
	bi.biSize = uint32(unsafe.Sizeof(bi))
	bi.biWidth = int32(w)
	bi.biHeight = -int32(h) // Top-down DIB
	bi.biPlanes = 1
	bi.biBitCount = 32
	bi.biCompression = biRgb

	var bitsPtr unsafe.Pointer
	hbm, _, err := procCreateDIBSection.Call(
		hdcMem,
		uintptr(unsafe.Pointer(&bi)),
		0,
		uintptr(unsafe.Pointer(&bitsPtr)),
		0,
		0,
	)
	if hbm == 0 || bitsPtr == nil {
		return nil, fmt.Errorf("CreateDIBSection failed: %v", err)
	}
	hbm = uintptr(uint32(hbm))
	defer procDeleteObject.Call(hbm)

	oldBmp, _, _ := procSelectObject.Call(hdcMem, hbm)
	defer procSelectObject.Call(hdcMem, oldBmp)

	hdcScreenVal := uintptr(uint32(hdcScreen))
	ret, _, err := procBitBlt.Call(
		hdcMem, 0, 0, uintptr(w), uintptr(h),
		hdcScreenVal, uintptr(x), uintptr(y),
		srccopy,
	)
	if ret == 0 {
		return nil, fmt.Errorf("screenshot capture requires an interactive desktop session (%v)", err)
	}

	// Copy pixels into standard Go image.RGBA
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	totalBytes := w * h * 4
	rawSlice := unsafe.Slice((*byte)(bitsPtr), totalBytes)

	// BGRA -> RGBA conversion
	for i := 0; i < totalBytes; i += 4 {
		b := rawSlice[i]
		g := rawSlice[i+1]
		r := rawSlice[i+2]
		img.Pix[i] = r
		img.Pix[i+1] = g
		img.Pix[i+2] = b
		img.Pix[i+3] = 255
	}

	return img, nil
}
