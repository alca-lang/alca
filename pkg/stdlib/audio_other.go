// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package stdlib

import (
	"fmt"
)

func playWAVBytes(data []byte, async bool) error {
	return fmt.Errorf("audio playback is natively supported on Windows; on Unix use external player")
}

func playAudioSource(data []byte, async bool, isWAV bool) error {
	return fmt.Errorf("audio playback is natively supported on Windows; on Unix use external player")
}

func pauseAudio() {}

func resumeAudio() {}

func setAudioVolume(vol int) {}

func stopAudio() {}

func systemBeep(freq, dur int) {
	fmt.Print("\a")
}
