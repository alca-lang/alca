// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

var (
	winmmDLLAudio      = syscall.NewLazyDLL("winmm.dll")
	kernel32Audio      = syscall.NewLazyDLL("kernel32.dll")
	procPlay           = winmmDLLAudio.NewProc("PlaySoundW")
	procBeep           = kernel32Audio.NewProc("Beep")
	procMCISendStringW = winmmDLLAudio.NewProc("mciSendStringW")

	audioMu        sync.Mutex
	lastAudioBytes []byte
	mciTempFile    string
	mciActive      bool
)

const (
	sndSync      = 0x00000000
	sndAsync     = 0x00000001
	sndNodefault = 0x00000002
	sndMemory    = 0x00000004
	sndPurge     = 0x00000040
)

func mciSend(cmd string) error {
	if procMCISendStringW.Find() != nil {
		return fmt.Errorf("mciSendStringW not found")
	}
	p, err := syscall.UTF16PtrFromString(cmd)
	if err != nil {
		return err
	}
	ret, _, _ := procMCISendStringW.Call(uintptr(unsafe.Pointer(p)), 0, 0, 0)
	if ret != 0 {
		return fmt.Errorf("mci command %q failed with code %d", cmd, ret)
	}
	return nil
}

func playWAVBytes(data []byte, async bool) error {
	if procPlay.Find() != nil {
		return fmt.Errorf("PlaySoundW not available in winmm.dll")
	}

	flags := uintptr(sndMemory | sndNodefault)
	if async {
		flags |= uintptr(sndAsync)
	} else {
		flags |= uintptr(sndSync)
	}

	audioMu.Lock()
	// Stop any active MCI playback
	if mciActive {
		_ = mciSend("close alca_audio")
		mciActive = false
	}
	lastAudioBytes = data
	ptr := uintptr(0)
	if len(data) > 0 {
		ptr = uintptr(unsafe.Pointer(&data[0]))
	}
	audioMu.Unlock()

	ret, _, err := procPlay.Call(ptr, 0, flags)
	if ret == 0 {
		return fmt.Errorf("PlaySoundW failed: %v", err)
	}
	return nil
}

func playAudioSource(data []byte, async bool, isWAV bool) error {
	if isWAV {
		return playWAVBytes(data, async)
	}

	// For MP3, MIDI, etc. use Windows MCI
	audioMu.Lock()
	defer audioMu.Unlock()

	// Stop previous sound
	if procPlay.Find() == nil {
		procPlay.Call(0, 0, uintptr(sndPurge))
	}
	if mciActive {
		_ = mciSend("close alca_audio")
		mciActive = false
	}

	tmpDir := os.TempDir()
	ext := ".mp3"
	if len(data) >= 4 && string(data[:4]) == "MThd" {
		ext = ".mid"
	}
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("alca_audio_%d%s", os.Getpid(), ext))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("audio: cannot write temporary file for playback: %w", err)
	}
	mciTempFile = tmpFile

	openCmd := fmt.Sprintf(`open "%s" alias alca_audio`, tmpFile)
	if err := mciSend(openCmd); err != nil {
		return err
	}
	mciActive = true

	playCmd := "play alca_audio"
	if !async {
		playCmd += " wait"
	}
	return mciSend(playCmd)
}

func pauseAudio() {
	audioMu.Lock()
	defer audioMu.Unlock()
	if mciActive {
		_ = mciSend("pause alca_audio")
	}
}

func resumeAudio() {
	audioMu.Lock()
	defer audioMu.Unlock()
	if mciActive {
		_ = mciSend("resume alca_audio")
	}
}

func setAudioVolume(vol int) {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	audioMu.Lock()
	defer audioMu.Unlock()
	if mciActive {
		_ = mciSend(fmt.Sprintf("setaudio alca_audio volume to %d", vol*10))
	}
}

func stopAudio() {
	audioMu.Lock()
	defer audioMu.Unlock()

	if procPlay.Find() == nil {
		procPlay.Call(0, 0, uintptr(sndPurge))
	}
	lastAudioBytes = nil

	if mciActive {
		_ = mciSend("stop alca_audio")
		_ = mciSend("close alca_audio")
		mciActive = false
		if mciTempFile != "" {
			_ = os.Remove(mciTempFile)
			mciTempFile = ""
		}
	}
}

func systemBeep(freq, dur int) {
	if procBeep.Find() == nil {
		if freq < 37 {
			freq = 37
		}
		if freq > 32767 {
			freq = 32767
		}
		if dur < 1 {
			dur = 1
		}
		procBeep.Call(uintptr(freq), uintptr(dur))
	}
}
