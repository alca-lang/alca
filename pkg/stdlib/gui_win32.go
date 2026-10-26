// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	win32User32   = syscall.NewLazyDLL("user32.dll")
	win32Kernel32 = syscall.NewLazyDLL("kernel32.dll")
	win32ComCtl32 = syscall.NewLazyDLL("comctl32.dll")
	win32Gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procInitCommonControlsEx = win32ComCtl32.NewProc("InitCommonControlsEx")
	procGetModuleHandleW     = win32Kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW     = win32User32.NewProc("RegisterClassExW")
	procCreateWindowExW      = win32User32.NewProc("CreateWindowExW")
	procDestroyWindow        = win32User32.NewProc("DestroyWindow")
	procDefWindowProcW       = win32User32.NewProc("DefWindowProcW")
	procShowWindow           = win32User32.NewProc("ShowWindow")
	procUpdateWindow         = win32User32.NewProc("UpdateWindow")
	procGetMessageW          = win32User32.NewProc("GetMessageW")
	procTranslateMessage     = win32User32.NewProc("TranslateMessage")
	procDispatchMessageW     = win32User32.NewProc("DispatchMessageW")
	procPostQuitMessage      = win32User32.NewProc("PostQuitMessage")
	procPostMessageW         = win32User32.NewProc("PostMessageW")
	procSendMessageW         = win32User32.NewProc("SendMessageW")
	procSetTimer             = win32User32.NewProc("SetTimer")
	procKillTimer            = win32User32.NewProc("KillTimer")
	procGetClientRect        = win32User32.NewProc("GetClientRect")
	procMoveWindow           = win32User32.NewProc("MoveWindow")
	procLoadCursorW          = win32User32.NewProc("LoadCursorW")
	procLoadIconW            = win32User32.NewProc("LoadIconW")
	procGetWindowTextW       = win32User32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = win32User32.NewProc("GetWindowTextLengthW")
	procWin32MessageBoxW     = win32User32.NewProc("MessageBoxW")
	procSetForegroundWindow  = win32User32.NewProc("SetForegroundWindow")
	procGetStockObject       = win32Gdi32.NewProc("GetStockObject")
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct {
	x, y int32
}

type winMsg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type rect struct {
	left, top, right, bottom int32
}

type initCommonControlsEx struct {
	dwSize uint32
	dwICC  uint32
}

type lvColumnW struct {
	mask       uint32
	fmt        int32
	cx         int32
	pszText    *uint16
	cchTextMax int32
	iSubItem   int32
	iImage     int32
	iOrder     int32
}

type lvItemW struct {
	mask       uint32
	iItem      int32
	iSubItem   int32
	state      uint32
	stateMask  uint32
	pszText    *uint16
	cchTextMax int32
	iImage     int32
	lParam     uintptr
	iIndent    int32
	iGroupId   int32
	cColumns   uint32
	puColumns  uintptr
}

const (
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	wsBorder           = 0x00800000
	wsVScroll          = 0x00200000

	esAutoHScroll = 0x0080
	bsPushButton  = 0x0000

	lvsReport        = 0x0001
	lvsSingleSel     = 0x0004
	lvsShowSelAlways = 0x0008

	lvsExFullRowSelect = 0x00000020
	lvsExGridLines     = 0x00000001
	lvsExDoubleBuffer  = 0x00010000

	lvmFirst                    = 0x1000
	lvmInsertColumnW            = lvmFirst + 97
	lvmInsertItemW              = lvmFirst + 77
	lvmSetItemTextW             = lvmFirst + 116
	lvmDeleteAllItems           = lvmFirst + 9
	lvmGetNextItem              = lvmFirst + 12
	lvmGetItemTextW             = lvmFirst + 115
	lvmSetExtendedListViewStyle = lvmFirst + 54

	lvcfFmt     = 0x0001
	lvcfWidth   = 0x0002
	lvcfText    = 0x0004
	lvcfSubItem = 0x0008

	lvifText  = 0x0001
	lvifParam = 0x0004

	lvniSelected = 0x0002

	wmCreate  = 0x0001
	wmDestroy = 0x0002
	wmSize    = 0x0005
	wmClose   = 0x0010
	wmCommand = 0x0111
	wmTimer   = 0x0113

	iccListViewClasses = 0x00000001
	colorBtnFace       = 15
	colorWindow        = 5
	idcArrow           = 32512

	cmdSearchBox  = 101
	cmdBtnRefresh = 102
	cmdBtnEndTask = 103
	timerRefresh  = 1
)

var (
	gHwndMain     uintptr
	gHwndList     uintptr
	gHwndSearch   uintptr
	gHwndBtnRef   uintptr
	gHwndBtnKill  uintptr
	gHwndStatus   uintptr
	gSearchFilter string
)

func taskMgrWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case wmCreate:
		return 0

	case wmSize:
		var rc rect
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		width := rc.right - rc.left
		height := rc.bottom - rc.top

		// Top controls layout
		// Search box: x=10, y=10, w=220, h=24
		procMoveWindow.Call(gHwndSearch, 10, 10, 220, 24, 1)
		// Refresh button: x=240, y=9, w=80, h=26
		procMoveWindow.Call(gHwndBtnRef, 240, 9, 85, 26, 1)
		// End Task button: x=335, y=9, w=95, h=26
		procMoveWindow.Call(gHwndBtnKill, 335, 9, 95, 26, 1)
		// Status label: x=445, y=13, w=width-455, h=20
		if width > 460 {
			procMoveWindow.Call(gHwndStatus, 445, 13, uintptr(width-455), 20, 1)
		}

		// ListView: x=10, y=44, w=width-20, h=height-54
		if width > 20 && height > 54 {
			procMoveWindow.Call(gHwndList, 10, 44, uintptr(width-20), uintptr(height-54), 1)
		}
		return 0

	case wmTimer:
		if wParam == timerRefresh {
			refreshProcessList()
		}
		return 0

	case wmCommand:
		ctlID := uint16(wParam & 0xFFFF)
		notif := uint16((wParam >> 16) & 0xFFFF)

		if ctlID == cmdSearchBox && notif == 0x0300 { // EN_CHANGE
			length, _, _ := procGetWindowTextLengthW.Call(gHwndSearch)
			buf := make([]uint16, length+1)
			procGetWindowTextW.Call(gHwndSearch, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
			gSearchFilter = strings.ToLower(strings.TrimSpace(syscall.UTF16ToString(buf)))
			refreshProcessList()
			return 0
		}

		if ctlID == cmdBtnRefresh {
			refreshProcessList()
			return 0
		}

		if ctlID == cmdBtnEndTask {
			onEndTaskClicked(hwnd)
			return 0
		}

	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		procKillTimer.Call(hwnd, timerRefresh)
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func onEndTaskClicked(hwnd uintptr) {
	// Find selected row
	selIdx, _, _ := procSendMessageW.Call(gHwndList, lvmGetNextItem, ^uintptr(0), lvniSelected)
	if int(selIdx) < 0 {
		return
	}

	// Read PID from column 0
	buf := make([]uint16, 64)
	var item lvItemW
	item.iSubItem = 0
	item.cchTextMax = int32(len(buf))
	item.pszText = &buf[0]
	procSendMessageW.Call(gHwndList, lvmGetItemTextW, selIdx, uintptr(unsafe.Pointer(&item)))
	pidStr := syscall.UTF16ToString(buf)
	pid, _ := strconv.Atoi(pidStr)
	if pid <= 0 {
		return
	}

	// Read Process Name from column 1
	nameBuf := make([]uint16, 128)
	item.iSubItem = 1
	item.cchTextMax = int32(len(nameBuf))
	item.pszText = &nameBuf[0]
	procSendMessageW.Call(gHwndList, lvmGetItemTextW, selIdx, uintptr(unsafe.Pointer(&nameBuf[0])))
	procName := syscall.UTF16ToString(nameBuf)

	confirmMsg, _ := syscall.UTF16PtrFromString(fmt.Sprintf("Are you sure you want to end process '%s' (PID %d)?", procName, pid))
	title, _ := syscall.UTF16PtrFromString("End Task Confirmation")
	// MB_YESNO (0x4) | MB_ICONWARNING (0x30)
	res, _, _ := procWin32MessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(confirmMsg)), uintptr(unsafe.Pointer(title)), 0x4|0x30)
	if res == 6 { // IDYES
		err := KillProcess(pid)
		if err != nil {
			errMsg, _ := syscall.UTF16PtrFromString("Failed to terminate process: " + err.Error())
			errTitle, _ := syscall.UTF16PtrFromString("Access Denied")
			procWin32MessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(errMsg)), uintptr(unsafe.Pointer(errTitle)), 0x10) // MB_ICONERROR
		} else {
			refreshProcessList()
		}
	}
}

func refreshProcessList() {
	if gHwndList == 0 {
		return
	}

	procs, err := SystemProcesses()
	if err != nil {
		return
	}

	// Remember selection
	selPID := -1
	selIdx, _, _ := procSendMessageW.Call(gHwndList, lvmGetNextItem, ^uintptr(0), lvniSelected)
	if int(selIdx) >= 0 {
		buf := make([]uint16, 32)
		var item lvItemW
		item.iSubItem = 0
		item.cchTextMax = int32(len(buf))
		item.pszText = &buf[0]
		procSendMessageW.Call(gHwndList, lvmGetItemTextW, selIdx, uintptr(unsafe.Pointer(&item)))
		selPID, _ = strconv.Atoi(syscall.UTF16ToString(buf))
	}

	procSendMessageW.Call(gHwndList, lvmDeleteAllItems, 0, 0)

	filter := gSearchFilter
	insertedCount := 0
	newSelIdx := -1

	for _, p := range procs {
		pid := p["pid"].(int64)
		name := p["name"].(string)
		ppid := p["ppid"].(int64)

		if filter != "" {
			if !strings.Contains(strings.ToLower(name), filter) && !strings.Contains(strconv.FormatInt(pid, 10), filter) {
				continue
			}
		}

		pidStr := strconv.FormatInt(pid, 10)
		pidPtr, _ := syscall.UTF16PtrFromString(pidStr)

		var item lvItemW
		item.mask = lvifText | lvifParam
		item.iItem = int32(insertedCount)
		item.iSubItem = 0
		item.pszText = pidPtr
		item.lParam = uintptr(pid)

		r, _, _ := procSendMessageW.Call(gHwndList, lvmInsertItemW, 0, uintptr(unsafe.Pointer(&item)))
		idx := int32(r)

		namePtr, _ := syscall.UTF16PtrFromString(name)
		procSendMessageW.Call(gHwndList, lvmSetItemTextW, uintptr(idx), uintptr(unsafe.Pointer(&lvItemW{
			iSubItem: 1,
			pszText:  namePtr,
		})))

		ppidPtr, _ := syscall.UTF16PtrFromString(strconv.FormatInt(ppid, 10))
		procSendMessageW.Call(gHwndList, lvmSetItemTextW, uintptr(idx), uintptr(unsafe.Pointer(&lvItemW{
			iSubItem: 2,
			pszText:  ppidPtr,
		})))

		statusPtr, _ := syscall.UTF16PtrFromString("Running")
		procSendMessageW.Call(gHwndList, lvmSetItemTextW, uintptr(idx), uintptr(unsafe.Pointer(&lvItemW{
			iSubItem: 3,
			pszText:  statusPtr,
		})))

		if int(pid) == selPID {
			newSelIdx = int(idx)
		}

		insertedCount++
	}

	// Restore selection
	if newSelIdx >= 0 {
		var selItem lvItemW
		selItem.state = 0x0002 // LVIS_SELECTED
		selItem.stateMask = 0x0002
		procSendMessageW.Call(gHwndList, lvmFirst+43, uintptr(newSelIdx), uintptr(unsafe.Pointer(&selItem))) // LVM_SETITEMSTATE
	}

	// Update status label
	tot, _, used, pct, err := RamUsageStats()
	if err != nil {
		statusText := fmt.Sprintf("Tasks: %d | RAM metrics unavailable: %s | CPU Cores: %d",
			insertedCount, err, runtime.NumCPU())
		statusPtr, _ := syscall.UTF16PtrFromString(statusText)
		procSendMessageW.Call(gHwndStatus, 0x000C, 0, uintptr(unsafe.Pointer(statusPtr)))
		return
	}
	usedGB := float64(used) / (1024 * 1024 * 1024)
	totGB := float64(tot) / (1024 * 1024 * 1024)
	statusText := fmt.Sprintf("Tasks: %d | RAM: %.1f / %.1f GB (%.0f%%) | CPU Cores: %d",
		insertedCount, usedGB, totGB, pct, runtime.NumCPU())
	statusPtr, _ := syscall.UTF16PtrFromString(statusText)
	procSendMessageW.Call(gHwndStatus, 0x000C, 0, uintptr(unsafe.Pointer(statusPtr))) // WM_SETTEXT
}

// RunTrueNativeWin32TaskManager launches a 100% native Win32 Task Manager window
// directly inside the process without calling PowerShell, WebView, or any child processes.
func RunTrueNativeWin32TaskManager(title string, width, height int) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if width <= 0 {
		width = 980
	}
	if height <= 0 {
		height = 640
	}

	// Center window on primary monitor
	screenWidth, _, _ := procGetSystemMetrics.Call(0)  // SM_CXSCREEN
	screenHeight, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
	posX := 80
	posY := 80
	if screenWidth > uintptr(width) {
		posX = int(screenWidth-uintptr(width)) / 2
	}
	if screenHeight > uintptr(height) {
		posY = int(screenHeight-uintptr(height)) / 2
	}

	// 1. Initialize Common Controls
	var icc initCommonControlsEx
	icc.dwSize = uint32(unsafe.Sizeof(icc))
	icc.dwICC = iccListViewClasses
	procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("AlcaNativeTaskMgr")

	wndProcCallback := syscall.NewCallback(taskMgrWndProc)

	hCursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	hIcon, _, _ := procLoadIconW.Call(0, 32512)

	var wc wndClassExW
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	wc.style = 0x0003 // CS_HREDRAW | CS_VREDRAW
	wc.lpfnWndProc = wndProcCallback
	wc.hInstance = hInst
	wc.hIcon = hIcon
	wc.hCursor = hCursor
	wc.hbrBackground = colorBtnFace + 1
	wc.lpszClassName = className
	wc.hIconSm = hIcon

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	titlePtr, _ := syscall.UTF16PtrFromString(title)

	// Create main window at centered coordinates
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(titlePtr)),
		wsOverlappedWindow|wsVisible,
		uintptr(posX), uintptr(posY),
		uintptr(width), uintptr(height),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("failed to create native window: %v", err)
	}
	gHwndMain = hwnd

	editClass, _ := syscall.UTF16PtrFromString("EDIT")
	buttonClass, _ := syscall.UTF16PtrFromString("BUTTON")
	staticClass, _ := syscall.UTF16PtrFromString("STATIC")
	listViewClass, _ := syscall.UTF16PtrFromString("SysListView32")

	// Search box
	searchPlaceholder, _ := syscall.UTF16PtrFromString("")
	gHwndSearch, _, _ = procCreateWindowExW.Call(
		0x200, // WS_EX_CLIENTEDGE
		uintptr(unsafe.Pointer(editClass)),
		uintptr(unsafe.Pointer(searchPlaceholder)),
		wsChild|wsVisible|wsBorder|esAutoHScroll,
		10, 10, 220, 24,
		hwnd, cmdSearchBox, hInst, 0,
	)
	// EM_SETCUEBANNER (placeholder text)
	cueText, _ := syscall.UTF16PtrFromString("Search process name or PID...")
	procSendMessageW.Call(gHwndSearch, 0x1501, 1, uintptr(unsafe.Pointer(cueText)))

	// Refresh button
	refText, _ := syscall.UTF16PtrFromString("Refresh")
	gHwndBtnRef, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(refText)),
		wsChild|wsVisible|bsPushButton,
		240, 9, 85, 26,
		hwnd, cmdBtnRefresh, hInst, 0,
	)

	// End Task button
	killText, _ := syscall.UTF16PtrFromString("End Task")
	gHwndBtnKill, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(killText)),
		wsChild|wsVisible|bsPushButton,
		335, 9, 95, 26,
		hwnd, cmdBtnEndTask, hInst, 0,
	)

	// Status label
	initStatus, _ := syscall.UTF16PtrFromString("Loading...")
	gHwndStatus, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		uintptr(unsafe.Pointer(initStatus)),
		wsChild|wsVisible,
		445, 13, uintptr(width-455), 20,
		hwnd, 0, hInst, 0,
	)

	// ListView
	gHwndList, _, _ = procCreateWindowExW.Call(
		0x200, // WS_EX_CLIENTEDGE
		uintptr(unsafe.Pointer(listViewClass)),
		0,
		wsChild|wsVisible|lvsReport|lvsSingleSel|lvsShowSelAlways|wsBorder|wsVScroll,
		10, 44, uintptr(width-20), uintptr(height-54),
		hwnd, 0, hInst, 0,
	)

	// Extended styles: FullRowSelect, GridLines, DoubleBuffer
	procSendMessageW.Call(gHwndList, lvmSetExtendedListViewStyle, 0,
		lvsExFullRowSelect|lvsExGridLines|lvsExDoubleBuffer)

	// Apply modern font to controls
	hFont, _, _ := procGetStockObject.Call(17) // DEFAULT_GUI_FONT
	if hFont != 0 {
		procSendMessageW.Call(gHwndSearch, 0x0030, hFont, 1)
		procSendMessageW.Call(gHwndBtnRef, 0x0030, hFont, 1)
		procSendMessageW.Call(gHwndBtnKill, 0x0030, hFont, 1)
		procSendMessageW.Call(gHwndStatus, 0x0030, hFont, 1)
		procSendMessageW.Call(gHwndList, 0x0030, hFont, 1)
	}

	// Add Columns
	addCol := func(subItem int, text string, width int) {
		tPtr, _ := syscall.UTF16PtrFromString(text)
		var col lvColumnW
		col.mask = lvcfFmt | lvcfWidth | lvcfText | lvcfSubItem
		col.fmt = 0 // LVCFMT_LEFT
		col.cx = int32(width)
		col.pszText = tPtr
		col.iSubItem = int32(subItem)
		procSendMessageW.Call(gHwndList, lvmInsertColumnW, uintptr(subItem), uintptr(unsafe.Pointer(&col)))
	}

	addCol(0, "PID", 85)
	addCol(1, "Process Name", 320)
	addCol(2, "Parent PID", 95)
	addCol(3, "Status", 110)

	// Initial population
	refreshProcessList()

	// Set 2-second auto-refresh timer
	procSetTimer.Call(hwnd, timerRefresh, 2000, 0)

	procShowWindow.Call(hwnd, swShowNormal)
	procUpdateWindow.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)

	// True Win32 Message Loop
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
