// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"testing"
	"time"
)

func TestNativeWin32TaskManagerWindow(t *testing.T) {
	requireGUIE2E(t)

	closeDone := make(chan bool, 1)

	go func() {
		defer close(closeDone)
		// Poll until the native window is created
		for i := 0; i < 50; i++ {
			time.Sleep(50 * time.Millisecond)
			if gHwndMain != 0 && gHwndList != 0 {
				break
			}
		}

		if gHwndMain == 0 {
			t.Errorf("expected native window handle to be non-zero")
			return
		}

		if gHwndList == 0 {
			t.Errorf("expected native listview handle to be non-zero")
			return
		}

		// Wait another 150ms for initial population
		time.Sleep(150 * time.Millisecond)

		// Post WM_CLOSE to gracefully close the window
		procPostMessageW.Call(gHwndMain, wmClose, 0, 0)
	}()

	err := RunTrueNativeWin32TaskManager("Alca Native Win32 Test", 800, 600)
	if err != nil {
		t.Fatalf("RunTrueNativeWin32TaskManager failed: %v", err)
	}

	<-closeDone
}
