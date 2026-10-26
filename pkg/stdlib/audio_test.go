// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/stdlib"
)

func TestAudioModule(t *testing.T) {
	env := interpreter.NewEnvironment()
	stdlib.RegisterAudioBuiltins(env)

	val, ok := env.Get("audio")
	if !ok {
		t.Fatalf("expected 'audio' module to be registered")
	}
	mod := val.(*interpreter.MapValue)

	createBufFn, _ := mod.Get("create_buffer")
	sineFn, _ := mod.Get("sine")
	squareFn, _ := mod.Get("square")
	sawtoothFn, _ := mod.Get("sawtooth")
	triangleFn, _ := mod.Get("triangle")
	noiseFn, _ := mod.Get("noise")
	silenceFn, _ := mod.Get("silence")
	chirpFn, _ := mod.Get("chirp")
	gainFn, _ := mod.Get("gain")
	normalizeFn, _ := mod.Get("normalize")
	mixFn, _ := mod.Get("mix")
	panFn, _ := mod.Get("pan")
	delayFn, _ := mod.Get("delay")
	lowpassFn, _ := mod.Get("lowpass")
	highpassFn, _ := mod.Get("highpass")
	adsrFn, _ := mod.Get("adsr")
	fftFn, _ := mod.Get("fft")
	domFreqFn, _ := mod.Get("dominant_frequency")
	rmsFn, _ := mod.Get("rms")
	peakFn, _ := mod.Get("peak")
	zeroCrossFn, _ := mod.Get("zero_crossings")
	encodeWavFn, _ := mod.Get("encode_wav")
	decodeWavFn, _ := mod.Get("decode_wav")
	writeWavFn, _ := mod.Get("write_wav")
	readWavFn, _ := mod.Get("read_wav")

	// 1. Create buffer
	bufVal := createBufFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.IntegerValue{Val: 2},     // 2 channels
		&interpreter.IntegerValue{Val: 1000},  // 1000 samples
		&interpreter.IntegerValue{Val: 48000}, // 48kHz
	)
	mp, ok := bufVal.(interpreter.MethodProvider)
	if !ok {
		t.Fatalf("expected AudioBuffer to be MethodProvider, got %T", bufVal)
	}
	chVal, _ := mp.CallMethod("channels", nil)
	if chVal.(*interpreter.IntegerValue).Val != 2 {
		t.Fatalf("expected 2 channels, got %v", chVal)
	}

	// 2. Synthesize Sine 440Hz, 1.0s at 44100Hz
	sineBuf := sineFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.FloatValue{Val: 440.0},
		&interpreter.FloatValue{Val: 1.0},
		&interpreter.IntegerValue{Val: 44100},
		&interpreter.FloatValue{Val: 0.9},
	)
	smp, _ := sineBuf.(interpreter.MethodProvider)
	durVal, _ := smp.CallMethod("duration", nil)
	if math.Abs(durVal.(*interpreter.FloatValue).Val-1.0) > 0.001 {
		t.Fatalf("expected duration 1.0s, got %v", durVal)
	}

	// 3. FFT & Dominant Frequency (should detect ~440 Hz)
	domVal := domFreqFn.(*interpreter.BuiltinFunction).Fn(sineBuf, &interpreter.IntegerValue{Val: 0})
	domFreq := domVal.(*interpreter.FloatValue).Val
	if math.Abs(domFreq-440.0) > 15.0 {
		t.Fatalf("expected dominant frequency around 440Hz, got %.2f Hz", domFreq)
	}

	// 4. RMS & Peak
	peakVal := peakFn.(*interpreter.BuiltinFunction).Fn(sineBuf)
	if math.Abs(peakVal.(*interpreter.FloatValue).Val-0.9) > 0.05 {
		t.Fatalf("expected peak around 0.9, got %v", peakVal)
	}
	rmsVal := rmsFn.(*interpreter.BuiltinFunction).Fn(sineBuf)
	expectedRMS := 0.9 / math.Sqrt(2)
	if math.Abs(rmsVal.(*interpreter.FloatValue).Val-expectedRMS) > 0.05 {
		t.Fatalf("expected RMS ~%.3f, got %v", expectedRMS, rmsVal)
	}

	// 5. Zero Crossings (440Hz * 2 crossings * 1s = ~880 crossings)
	zcVal := zeroCrossFn.(*interpreter.BuiltinFunction).Fn(sineBuf)
	zc := zcVal.(*interpreter.IntegerValue).Val
	if math.Abs(float64(zc-880)) > 20 {
		t.Fatalf("expected ~880 zero crossings, got %d", zc)
	}

	// 6. ADSR envelope
	adsrBuf := adsrFn.(*interpreter.BuiltinFunction).Fn(
		sineBuf,
		&interpreter.FloatValue{Val: 0.1}, // attack
		&interpreter.FloatValue{Val: 0.1}, // decay
		&interpreter.FloatValue{Val: 0.7}, // sustain
		&interpreter.FloatValue{Val: 0.2}, // release
	)
	if adsrBuf.Type() != "AUDIO_BUFFER" {
		t.Fatalf("expected AUDIO_BUFFER from adsr, got %s", adsrBuf.Type())
	}

	// 7. Biquad Lowpass filter
	filteredBuf := lowpassFn.(*interpreter.BuiltinFunction).Fn(
		adsrBuf,
		&interpreter.FloatValue{Val: 1000.0},
		&interpreter.FloatValue{Val: 0.707},
	)
	if filteredBuf.Type() != "AUDIO_BUFFER" {
		t.Fatalf("expected AUDIO_BUFFER from lowpass")
	}

	// 8. Pan & Gain
	panned := panFn.(*interpreter.BuiltinFunction).Fn(filteredBuf, &interpreter.FloatValue{Val: 0.5})
	gained := gainFn.(*interpreter.BuiltinFunction).Fn(panned, &interpreter.FloatValue{Val: 0.8})

	// 9. WAV Encode & Decode Round-trip (16-bit PCM)
	encodedWAV := encodeWavFn.(*interpreter.BuiltinFunction).Fn(gained, &interpreter.IntegerValue{Val: 16})
	wavBytesVal, ok := encodedWAV.(*interpreter.StringValue)
	if !ok || len(wavBytesVal.Val) < 44 {
		t.Fatalf("encode_wav failed: %v", encodedWAV)
	}
	// Verify RIFF header
	if wavBytesVal.Val[:4] != "RIFF" || wavBytesVal.Val[8:12] != "WAVE" {
		t.Fatalf("invalid WAV header: %q", wavBytesVal.Val[:12])
	}

	decodedBuf := decodeWavFn.(*interpreter.BuiltinFunction).Fn(wavBytesVal)
	if decodedBuf.Type() != "AUDIO_BUFFER" {
		t.Fatalf("decode_wav failed: %v", decodedBuf)
	}
	decMP, _ := decodedBuf.(interpreter.MethodProvider)
	decCh, _ := decMP.CallMethod("channels", nil)
	if decCh.(*interpreter.IntegerValue).Val != 2 {
		t.Fatalf("expected 2 channels after decode, got %v", decCh)
	}

	// 10. Base64 Encode & Decode Round-trip
	encB64Fn, _ := mod.Get("encode_base64")
	decB64Fn, _ := mod.Get("decode_base64")
	playFn, _ := mod.Get("play")

	b64Val := encB64Fn.(*interpreter.BuiltinFunction).Fn(gained, &interpreter.IntegerValue{Val: 16})
	b64Str, ok := b64Val.(*interpreter.StringValue)
	if !ok || len(b64Str.Val) == 0 {
		t.Fatalf("encode_base64 failed: %v", b64Val)
	}

	// Direct decode
	b64Decoded := decB64Fn.(*interpreter.BuiltinFunction).Fn(b64Str)
	if b64Decoded.Type() != "AUDIO_BUFFER" {
		t.Fatalf("decode_base64 failed: %v", b64Decoded)
	}

	// Data URI decode
	dataURI := &interpreter.StringValue{Val: "data:audio/wav;base64," + b64Str.Val}
	b64URIDecoded := decB64Fn.(*interpreter.BuiltinFunction).Fn(dataURI)
	if b64URIDecoded.Type() != "AUDIO_BUFFER" {
		t.Fatalf("decode_base64 with data URI failed: %v", b64URIDecoded)
	}

	// Play from base64 string directly
	playRes := playFn.(*interpreter.BuiltinFunction).Fn(b64Str, interpreter.TRUE)
	if playRes != interpreter.TRUE {
		t.Fatalf("play with base64 failed: %v", playRes)
	}

	// 11. File write and read
	tmpWav := filepath.Join(t.TempDir(), "test.wav")
	wrRes := writeWavFn.(*interpreter.BuiltinFunction).Fn(
		&interpreter.StringValue{Val: tmpWav},
		gained,
		&interpreter.IntegerValue{Val: 16},
	)
	if wrRes != interpreter.TRUE {
		t.Fatalf("write_wav failed: %v", wrRes)
	}

	readBuf := readWavFn.(*interpreter.BuiltinFunction).Fn(&interpreter.StringValue{Val: tmpWav})
	if readBuf.Type() != "AUDIO_BUFFER" {
		t.Fatalf("read_wav failed: %v", readBuf)
	}

	// 11. Test other synthesis shapes
	squareFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 220}, &interpreter.FloatValue{Val: 0.1})
	sawtoothFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 220}, &interpreter.FloatValue{Val: 0.1})
	triangleFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 220}, &interpreter.FloatValue{Val: 0.1})
	noiseFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 0.1}, &interpreter.StringValue{Val: "pink"})
	silenceFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 0.1})
	chirpFn.(*interpreter.BuiltinFunction).Fn(&interpreter.FloatValue{Val: 100}, &interpreter.FloatValue{Val: 1000}, &interpreter.FloatValue{Val: 0.1})
	highpassFn.(*interpreter.BuiltinFunction).Fn(sineBuf, &interpreter.FloatValue{Val: 200})
	normalizeFn.(*interpreter.BuiltinFunction).Fn(sineBuf, &interpreter.FloatValue{Val: 0.99})
	delayFn.(*interpreter.BuiltinFunction).Fn(sineBuf, &interpreter.FloatValue{Val: 0.1}, &interpreter.FloatValue{Val: 0.3}, &interpreter.FloatValue{Val: 0.4})
	mixFn.(*interpreter.BuiltinFunction).Fn(sineBuf, sineBuf, &interpreter.FloatValue{Val: 0.5}, &interpreter.FloatValue{Val: 0.5})
	fftFn.(*interpreter.BuiltinFunction).Fn(sineBuf, &interpreter.IntegerValue{Val: 0}, &interpreter.IntegerValue{Val: 1024})
}
