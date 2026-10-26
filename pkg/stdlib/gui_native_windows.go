// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Native Win32 widget window backend. This renders the generic gui.window
// widgets (labels, buttons, inputs, checkboxes and progress bars) with real
// USER32/COMCTL32 controls, so no PowerShell or child process is involved.

const (
	nativeControlIDBase = 2000

	bsAutoCheckbox = 0x0003
	wsExClientEdge = 0x00000200

	emSetCueBanner = 0x1501
	bmSetCheck     = 0x00F1
	bmGetCheck     = 0x00F0
	bstChecked     = 1

	pbmSetPos     = 0x0402
	pbmSetRange32 = 0x0406

	iccProgressClass = 0x00000020
	wmSetText        = 0x000C
	wmSetFont        = 0x0030
	defaultGUIFont   = 17
)

type nativeWindowState struct {
	win     *NativeWindow
	hwnd    uintptr
	handles []nativeWidgetHandle
}

type nativeWidgetHandle struct {
	widget *Widget
	hwnd   uintptr
}

// nativeActiveWindow is accessed only from the window's UI thread. The message
// loop and every window procedure callback run on that same locked thread.
var nativeActiveWindow *nativeWindowState

func nativeWindowProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	state := nativeActiveWindow
	switch uint32(msg) {
	case wmCreate:
		return 0

	case wmSize:
		if state != nil {
			layoutNativeWidgets(state, hwnd)
		}
		return 0

	case wmCommand:
		if state != nil {
			id := int(wParam & 0xFFFF)
			notification := uint16((wParam >> 16) & 0xFFFF)
			slot := id - nativeControlIDBase
			if slot >= 0 && slot < len(state.handles) {
				handle := state.handles[slot]
				switch handle.widget.Type {
				case "button":
					if notification == 0 { // BN_CLICKED
						state.win.invokeCallback(handle.widget.Index)
					}
				case "checkbox":
					if notification == 0 { // BN_CLICKED
						checked, _, _ := procSendMessageW.Call(handle.hwnd, bmGetCheck, 0, 0)
						value := "false"
						if checked == bstChecked {
							value = "true"
						}
						state.win.mu.Lock()
						handle.widget.Value = value
						state.win.mu.Unlock()
						state.win.invokeCallback(handle.widget.Index)
					}
				}
			}
			return 0
		}

	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func fixedGUIFont() uintptr {
	font, _, _ := procGetStockObject.Call(defaultGUIFont)
	return font
}

func layoutNativeWidgets(state *nativeWindowState, hwnd uintptr) {
	var rc rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	width := int(rc.right - rc.left)
	height := int(rc.bottom - rc.top)
	if width <= 0 || height <= 0 {
		return
	}

	margin := 16
	if width < 2*margin+80 {
		margin = 8
	}
	controlWidth := width - 2*margin
	if controlWidth < 40 {
		controlWidth = 40
	}

	y := margin
	font := fixedGUIFont()
	for _, handle := range state.handles {
		h := 24
		switch handle.widget.Type {
		case "label":
			h = 22
		case "button":
			h = 32
		case "input":
			h = 26
		case "checkbox":
			h = 24
		case "progress":
			h = 20
		}
		// Controls below the visible area are still created and laid out; the
		// window simply clips them, which keeps callbacks and state consistent
		// even for very small windows.
		procMoveWindow.Call(handle.hwnd, uintptr(margin), uintptr(y), uintptr(controlWidth), uintptr(h), 1)
		if font != 0 {
			procSendMessageW.Call(handle.hwnd, wmSetFont, font, 1)
		}
		y += h + 8
	}
}

// runNativeWindows creates a real Win32 window and runs its message loop. It
// blocks until the user closes the window.
func (w *NativeWindow) runNativeWindows() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	attachToDefaultDesktop()

	var icc initCommonControlsEx
	icc.dwSize = uint32(unsafe.Sizeof(icc))
	icc.dwICC = iccProgressClass
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))

	instance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("AlcaNativeWidgetWindow")
	title, _ := syscall.UTF16PtrFromString(w.Title)

	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	icon, _, _ := procLoadIconW.Call(0, 32512)

	wndProc := syscall.NewCallback(nativeWindowProc)

	var wc wndClassExW
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	wc.style = 0x0003 // CS_HREDRAW | CS_VREDRAW
	wc.lpfnWndProc = wndProc
	wc.hInstance = instance
	wc.hIcon = icon
	wc.hCursor = cursor
	wc.hbrBackground = colorBtnFace + 1
	wc.lpszClassName = className
	wc.hIconSm = icon
	// Registration can legitimately fail when a previous window already
	// registered the class in this process; CreateWindowEx still works.
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	screenWidth, _, _ := procGetSystemMetrics.Call(0)
	screenHeight, _, _ := procGetSystemMetrics.Call(1)
	posX := 80
	posY := 80
	if screenWidth > uintptr(w.Width) {
		posX = int(screenWidth-uintptr(w.Width)) / 2
	}
	if screenHeight > uintptr(w.Height) {
		posY = int(screenHeight-uintptr(w.Height)) / 2
	}

	state := &nativeWindowState{win: w}
	nativeActiveWindow = state
	defer func() { nativeActiveWindow = nil }()

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow|wsVisible,
		uintptr(posX), uintptr(posY),
		uintptr(w.Width), uintptr(w.Height),
		0, 0, instance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("failed to create native window: %v", err)
	}
	state.hwnd = hwnd

	staticClass, _ := syscall.UTF16PtrFromString("STATIC")
	editClass, _ := syscall.UTF16PtrFromString("EDIT")
	buttonClass, _ := syscall.UTF16PtrFromString("BUTTON")
	progressClass, _ := syscall.UTF16PtrFromString("msctls_progress32")

	w.mu.Lock()
	widgets := make([]*Widget, len(w.Widgets))
	copy(widgets, w.Widgets)
	w.mu.Unlock()

	font := fixedGUIFont()
	state.handles = make([]nativeWidgetHandle, 0, len(widgets))
	for _, widget := range widgets {
		id := uintptr(nativeControlIDBase + len(state.handles))
		text, _ := syscall.UTF16PtrFromString(widget.Text)

		var class *uint16
		var style uintptr
		var exStyle uintptr
		switch widget.Type {
		case "label":
			class = staticClass
			style = wsChild | wsVisible
		case "input":
			class = editClass
			style = wsChild | wsVisible | wsBorder | esAutoHScroll
			exStyle = wsExClientEdge
		case "button":
			class = buttonClass
			style = wsChild | wsVisible | bsPushButton
		case "checkbox":
			class = buttonClass
			style = wsChild | wsVisible | bsAutoCheckbox
		case "progress":
			class = progressClass
			style = wsChild | wsVisible
		default:
			continue
		}

		hwndControl, _, _ := procCreateWindowExW.Call(
			exStyle,
			uintptr(unsafe.Pointer(class)),
			uintptr(unsafe.Pointer(text)),
			style,
			16, 16, 200, 24,
			hwnd, id, instance, 0,
		)
		if hwndControl == 0 {
			continue
		}
		state.handles = append(state.handles, nativeWidgetHandle{widget: widget, hwnd: hwndControl})

		if font != 0 {
			procSendMessageW.Call(hwndControl, wmSetFont, font, 1)
		}

		switch widget.Type {
		case "input":
			if widget.Text != "" {
				cue, _ := syscall.UTF16PtrFromString(widget.Text)
				procSendMessageW.Call(hwndControl, emSetCueBanner, 1, uintptr(unsafe.Pointer(cue)))
			}
		case "checkbox":
			checked := uintptr(0)
			if widget.Value == "true" {
				checked = bstChecked
			}
			procSendMessageW.Call(hwndControl, bmSetCheck, checked, 0)
		case "progress":
			value := atoiDefault(widget.Value, 50)
			if value < 0 {
				value = 0
			}
			if value > 100 {
				value = 100
			}
			procSendMessageW.Call(hwndControl, pbmSetRange32, 0, uintptr(100<<16))
			procSendMessageW.Call(hwndControl, pbmSetPos, uintptr(value), 0)
		}
	}

	layoutNativeWidgets(state, hwnd)
	procShowWindow.Call(hwnd, swShowNormal)
	procUpdateWindow.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)

	var msg winMsg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if r == 0 || int32(r) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	return nil
}

// widgetText reads the current text of an EDIT control.
func widgetText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
