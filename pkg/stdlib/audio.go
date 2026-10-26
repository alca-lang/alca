// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hajimehoshi/go-mp3"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// audioBufferValue wraps an audio buffer as an opaque interpreter value.
type audioBufferValue struct {
	channels   int
	sampleRate int
	samples    [][]float64 // [channel][sample_index] in range [-1.0, 1.0]
}

func (v *audioBufferValue) Type() interpreter.ValueType { return "AUDIO_BUFFER" }
func (v *audioBufferValue) Inspect() string {
	dur := float64(v.numSamples()) / float64(v.sampleRate)
	return fmt.Sprintf("<audio buffer channels=%d rate=%d samples=%d duration=%.3fs>", v.channels, v.sampleRate, v.numSamples(), dur)
}
func (v *audioBufferValue) Clone() interpreter.Value {
	cp := &audioBufferValue{
		channels:   v.channels,
		sampleRate: v.sampleRate,
		samples:    make([][]float64, v.channels),
	}
	for ch := 0; ch < v.channels; ch++ {
		cp.samples[ch] = make([]float64, len(v.samples[ch]))
		copy(cp.samples[ch], v.samples[ch])
	}
	return cp
}

func (v *audioBufferValue) numSamples() int {
	if len(v.samples) == 0 {
		return 0
	}
	return len(v.samples[0])
}

// CallMethod implements interpreter.MethodProvider.
func (v *audioBufferValue) CallMethod(name string, args []interpreter.Value) (interpreter.Value, error) {
	switch name {
	case "channels":
		return &interpreter.IntegerValue{Val: int64(v.channels)}, nil
	case "sample_rate":
		return &interpreter.IntegerValue{Val: int64(v.sampleRate)}, nil
	case "num_samples", "length":
		return &interpreter.IntegerValue{Val: int64(v.numSamples())}, nil
	case "duration":
		return &interpreter.FloatValue{Val: float64(v.numSamples()) / float64(v.sampleRate)}, nil

	case "sample":
		if len(args) < 2 {
			return nil, fmt.Errorf("sample requires (channel, index)")
		}
		ch := int(args[0].(*interpreter.IntegerValue).Val)
		idx := int(args[1].(*interpreter.IntegerValue).Val)
		if ch < 0 || ch >= v.channels || idx < 0 || idx >= v.numSamples() {
			return nil, fmt.Errorf("sample index out of bounds: ch=%d, idx=%d", ch, idx)
		}
		return &interpreter.FloatValue{Val: v.samples[ch][idx]}, nil

	case "set_sample":
		if len(args) < 3 {
			return nil, fmt.Errorf("set_sample requires (channel, index, value)")
		}
		ch := int(args[0].(*interpreter.IntegerValue).Val)
		idx := int(args[1].(*interpreter.IntegerValue).Val)
		val := ltScalarFloat(args[2])
		if ch < 0 || ch >= v.channels || idx < 0 || idx >= v.numSamples() {
			return nil, fmt.Errorf("sample index out of bounds: ch=%d, idx=%d", ch, idx)
		}
		v.samples[ch][idx] = math.Max(-1.0, math.Min(1.0, val))
		return interpreter.NULL, nil

	case "channel_data":
		ch := 0
		if len(args) >= 1 {
			ch = int(args[0].(*interpreter.IntegerValue).Val)
		}
		if ch < 0 || ch >= v.channels {
			return nil, fmt.Errorf("channel %d out of bounds (channels=%d)", ch, v.channels)
		}
		elems := make([]interpreter.Value, len(v.samples[ch]))
		for i, s := range v.samples[ch] {
			elems[i] = &interpreter.FloatValue{Val: s}
		}
		return interpreter.NewList(elems...), nil

	case "clone":
		return v.Clone(), nil

	case "to_list":
		rows := make([]interpreter.Value, v.channels)
		for ch := 0; ch < v.channels; ch++ {
			cols := make([]interpreter.Value, len(v.samples[ch]))
			for i, s := range v.samples[ch] {
				cols[i] = &interpreter.FloatValue{Val: s}
			}
			rows[ch] = interpreter.NewList(cols...)
		}
		return interpreter.NewList(rows...), nil

	default:
		return nil, fmt.Errorf("audio buffer has no method %q", name)
	}
}

func asAudioBuffer(v interpreter.Value) (*audioBufferValue, error) {
	if ab, ok := v.(*audioBufferValue); ok {
		return ab, nil
	}
	return nil, fmt.Errorf("expected AudioBuffer")
}

// -----------------------------------------------------------------------------
// WAV Decoding & Encoding (Pure Go RIFF Parser)
// -----------------------------------------------------------------------------

func decodeWAV(r io.Reader) (*audioBufferValue, error) {
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, fmt.Errorf("invalid WAV header: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid RIFF/WAVE file")
	}

	var channels, sampleRate, bitsPerSample uint16
	var audioFormat uint16
	var rawData []byte

	for {
		var chunkHeader [8]byte
		if _, err := io.ReadFull(r, chunkHeader[:]); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		if chunkID == "fmt " {
			fmtBuf := make([]byte, chunkSize)
			if _, err := io.ReadFull(r, fmtBuf); err != nil {
				return nil, err
			}
			audioFormat = binary.LittleEndian.Uint16(fmtBuf[0:2])
			channels = binary.LittleEndian.Uint16(fmtBuf[2:4])
			sampleRate = uint16(binary.LittleEndian.Uint32(fmtBuf[4:8]))
			bitsPerSample = binary.LittleEndian.Uint16(fmtBuf[14:16])
		} else if chunkID == "data" {
			rawData = make([]byte, chunkSize)
			if _, err := io.ReadFull(r, rawData); err != nil {
				return nil, err
			}
			break
		} else {
			// skip unknown chunk
			if _, err := io.CopyN(io.Discard, r, int64(chunkSize)); err != nil {
				break
			}
		}
	}

	if len(rawData) == 0 || channels == 0 || sampleRate == 0 {
		return nil, fmt.Errorf("incomplete or empty WAV data")
	}

	bytesPerSample := int(bitsPerSample) / 8
	if bytesPerSample == 0 {
		bytesPerSample = 2
	}
	totalSamples := len(rawData) / (int(channels) * bytesPerSample)

	buf := &audioBufferValue{
		channels:   int(channels),
		sampleRate: int(sampleRate),
		samples:    make([][]float64, channels),
	}
	for ch := 0; ch < int(channels); ch++ {
		buf.samples[ch] = make([]float64, totalSamples)
	}

	reader := bytes.NewReader(rawData)
	for i := 0; i < totalSamples; i++ {
		for ch := 0; ch < int(channels); ch++ {
			var val float64
			switch {
			case audioFormat == 3 && bitsPerSample == 32: // IEEE Float
				var f float32
				binary.Read(reader, binary.LittleEndian, &f)
				val = float64(f)
			case bitsPerSample == 16: // 16-bit PCM
				var s int16
				binary.Read(reader, binary.LittleEndian, &s)
				val = float64(s) / 32768.0
			case bitsPerSample == 8: // 8-bit unsigned PCM
				b, _ := reader.ReadByte()
				val = (float64(b) - 128.0) / 128.0
			case bitsPerSample == 24: // 24-bit PCM
				var b [3]byte
				reader.Read(b[:])
				s := int32(b[0]) | (int32(b[1]) << 8) | (int32(b[2]) << 16)
				if s&0x800000 != 0 {
					s |= ^0xFFFFFF
				}
				val = float64(s) / 8388608.0
			case bitsPerSample == 32: // 32-bit integer PCM
				var s int32
				binary.Read(reader, binary.LittleEndian, &s)
				val = float64(s) / 2147483648.0
			}
			buf.samples[ch][i] = math.Max(-1.0, math.Min(1.0, val))
		}
	}
	return buf, nil
}

func encodeWAV(buf *audioBufferValue, bitDepth int) ([]byte, error) {
	if bitDepth != 16 && bitDepth != 24 && bitDepth != 32 {
		bitDepth = 16
	}
	channels := buf.channels
	numSamples := buf.numSamples()
	bytesPerSample := bitDepth / 8
	dataSize := uint32(numSamples * channels * bytesPerSample)

	var out bytes.Buffer
	// RIFF Header
	out.WriteString("RIFF")
	binary.Write(&out, binary.LittleEndian, uint32(36+dataSize))
	out.WriteString("WAVE")

	// fmt chunk
	out.WriteString("fmt ")
	binary.Write(&out, binary.LittleEndian, uint32(16))             // chunk size
	binary.Write(&out, binary.LittleEndian, uint16(1))              // PCM format
	binary.Write(&out, binary.LittleEndian, uint16(channels))       // channels
	binary.Write(&out, binary.LittleEndian, uint32(buf.sampleRate)) // sample rate
	byteRate := uint32(buf.sampleRate * channels * bytesPerSample)
	binary.Write(&out, binary.LittleEndian, byteRate) // byte rate
	blockAlign := uint16(channels * bytesPerSample)
	binary.Write(&out, binary.LittleEndian, blockAlign)       // block align
	binary.Write(&out, binary.LittleEndian, uint16(bitDepth)) // bits per sample

	// data chunk
	out.WriteString("data")
	binary.Write(&out, binary.LittleEndian, dataSize)

	for i := 0; i < numSamples; i++ {
		for ch := 0; ch < channels; ch++ {
			sample := math.Max(-1.0, math.Min(1.0, buf.samples[ch][i]))
			switch bitDepth {
			case 16:
				val := int16(sample * 32767.0)
				binary.Write(&out, binary.LittleEndian, val)
			case 24:
				val := int32(sample * 8388607.0)
				out.WriteByte(byte(val & 0xFF))
				out.WriteByte(byte((val >> 8) & 0xFF))
				out.WriteByte(byte((val >> 16) & 0xFF))
			case 32:
				val := int32(sample * 2147483647.0)
				binary.Write(&out, binary.LittleEndian, val)
			}
		}
	}
	return out.Bytes(), nil
}

// -----------------------------------------------------------------------------
// DSP, FFT, Filters, Oscillators
// -----------------------------------------------------------------------------

// fftCooleyTukey computes in-place radix-2 Decimation-In-Time FFT.
func fftCooleyTukey(real, imag []float64, invert bool) {
	n := len(real)
	// Bit reversal permutation
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			real[i], real[j] = real[j], real[i]
			imag[i], imag[j] = imag[j], imag[i]
		}
		k := n / 2
		for k <= j {
			j -= k
			k /= 2
		}
		j += k
	}

	// Butterfly updates
	sign := -1.0
	if invert {
		sign = 1.0
	}
	for l := 2; l <= n; l <<= 1 {
		angle := sign * 2.0 * math.Pi / float64(l)
		wlenReal := math.Cos(angle)
		wlenImag := math.Sin(angle)
		halfL := l / 2
		for i := 0; i < n; i += l {
			wReal := 1.0
			wImag := 0.0
			for k := 0; k < halfL; k++ {
				uReal := real[i+k]
				uImag := imag[i+k]
				vReal := real[i+k+halfL]*wReal - imag[i+k+halfL]*wImag
				vImag := real[i+k+halfL]*wImag + imag[i+k+halfL]*wReal

				real[i+k] = uReal + vReal
				imag[i+k] = uImag + vImag
				real[i+k+halfL] = uReal - vReal
				imag[i+k+halfL] = uImag - vImag

				nextWReal := wReal*wlenReal - wImag*wlenImag
				nextWImag := wReal*wlenImag + wImag*wlenReal
				wReal = nextWReal
				wImag = nextWImag
			}
		}
	}

	if invert {
		for i := range real {
			real[i] /= float64(n)
			imag[i] /= float64(n)
		}
	}
}

// nextPowerOfTwo returns the next power of 2 >= n.
func nextPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}

func cleanBase64Audio(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, ","); idx != -1 && strings.Contains(s[:idx], ";base64") {
		s = s[idx+1:]
	}
	return strings.TrimSpace(s)
}

func decodeMP3(r io.Reader) (*audioBufferValue, error) {
	dec, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, fmt.Errorf("mp3 decode header failed: %w", err)
	}

	sampleRate := dec.SampleRate()
	var rawData bytes.Buffer
	if _, err := io.Copy(&rawData, dec); err != nil {
		return nil, fmt.Errorf("mp3 read samples failed: %w", err)
	}

	b := rawData.Bytes()
	numFrames := len(b) / 4
	buf := &audioBufferValue{
		channels:   2,
		sampleRate: sampleRate,
		samples:    [][]float64{make([]float64, numFrames), make([]float64, numFrames)},
	}

	for i := 0; i < numFrames; i++ {
		leftInt := int16(binary.LittleEndian.Uint16(b[i*4 : i*4+2]))
		rightInt := int16(binary.LittleEndian.Uint16(b[i*4+2 : i*4+4]))
		buf.samples[0][i] = float64(leftInt) / 32768.0
		buf.samples[1][i] = float64(rightInt) / 32768.0
	}

	return buf, nil
}

func decodeAudioAny(data []byte) (*audioBufferValue, error) {
	if len(data) >= 4 && string(data[:4]) == "RIFF" {
		return decodeWAV(bytes.NewReader(data))
	}
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		return decodeMP3(bytes.NewReader(data))
	}
	if len(data) >= 2 && data[0] == 0xFF && (data[1]&0xE0) == 0xE0 {
		if buf, err := decodeMP3(bytes.NewReader(data)); err == nil {
			return buf, nil
		}
	}
	if buf, err := decodeWAV(bytes.NewReader(data)); err == nil {
		return buf, nil
	}
	if buf, err := decodeMP3(bytes.NewReader(data)); err == nil {
		return buf, nil
	}
	return nil, fmt.Errorf("unsupported audio format (expected WAV or MP3)")
}

func resolveAudioBytes(val interpreter.Value) ([]byte, bool, error) {
	if ab, ok := val.(*audioBufferValue); ok {
		data, err := encodeWAV(ab, 16)
		return data, true, err
	}
	if bv, ok := val.(*interpreter.BlobValue); ok {
		isWav := len(bv.Data) >= 4 && string(bv.Data[:4]) == "RIFF"
		return bv.Data, isWav, nil
	}
	str := interpreter.CoerceToString(val)
	s := strings.TrimSpace(str)

	// HTTP/HTTPS URL
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(s)
		if err != nil {
			return nil, false, fmt.Errorf("failed to fetch audio URL %q: %w", s, err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read audio from URL %q: %w", s, err)
		}
		isWav := len(data) >= 4 && string(data[:4]) == "RIFF"
		return data, isWav, nil
	}

	// Base64
	cleaned := cleanBase64Audio(s)
	if b64, err := base64.StdEncoding.DecodeString(cleaned); err == nil && len(b64) >= 4 {
		isWav := string(b64[:4]) == "RIFF"
		isMp3 := (len(b64) >= 3 && string(b64[:3]) == "ID3") || (b64[0] == 0xFF && (b64[1]&0xE0) == 0xE0)
		if isWav || isMp3 {
			return b64, isWav, nil
		}
	}

	// File on disk
	if data, err := os.ReadFile(str); err == nil {
		isWav := len(data) >= 4 && string(data[:4]) == "RIFF"
		return data, isWav, nil
	}

	// Raw bytes in string
	raw := []byte(str)
	if len(raw) >= 4 && string(raw[:4]) == "RIFF" {
		return raw, true, nil
	}
	if len(raw) >= 3 && string(raw[:3]) == "ID3" {
		return raw, false, nil
	}

	return nil, false, fmt.Errorf("cannot resolve audio source %q", str)
}

// RegisterAudioBuiltins registers "std.audio" and "audio" into environment.
func RegisterAudioBuiltins(env *interpreter.Environment) {
	mod := interpreter.NewMap()

	reg := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		mod.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// audio.create_buffer(channels: int, num_samples: int, sample_rate?: int) -> AudioBuffer
	reg("create_buffer", func(args ...interpreter.Value) interpreter.Value {
		channels := 1
		samples := 0
		rate := 44100
		if len(args) >= 1 {
			channels = ltIntArg(args, 0, 1)
		}
		if len(args) >= 2 {
			samples = ltIntArg(args, 1, 0)
		}
		if len(args) >= 3 {
			rate = ltIntArg(args, 2, 44100)
		}
		if channels <= 0 {
			channels = 1
		}
		if samples < 0 {
			samples = 0
		}
		buf := &audioBufferValue{
			channels:   channels,
			sampleRate: rate,
			samples:    make([][]float64, channels),
		}
		for ch := 0; ch < channels; ch++ {
			buf.samples[ch] = make([]float64, samples)
		}
		return buf
	})

	// audio.read(source: string|blob) -> AudioBuffer (universal: file, url, base64)
	reg("read", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.read requires (path | url | base64)")
		}
		data, _, err := resolveAudioBytes(args[0])
		if err != nil {
			return interpreter.NewError(400, "audio.read failed: "+err.Error())
		}
		buf, err := decodeAudioAny(data)
		if err != nil {
			return interpreter.NewError(400, "audio.read: decode failed: "+err.Error())
		}
		return buf
	})

	// audio.decode(data: string|blob) -> AudioBuffer (universal: WAV or MP3 from bytes or base64)
	reg("decode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.decode requires bytes or base64")
		}
		data, _, err := resolveAudioBytes(args[0])
		if err != nil {
			return interpreter.NewError(400, "audio.decode failed: "+err.Error())
		}
		buf, err := decodeAudioAny(data)
		if err != nil {
			return interpreter.NewError(400, "audio.decode failed: "+err.Error())
		}
		return buf
	})

	// audio.decode_mp3(data: string|blob) -> AudioBuffer
	reg("decode_mp3", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.decode_mp3 requires bytes or base64")
		}
		data, _, err := resolveAudioBytes(args[0])
		if err != nil {
			return interpreter.NewError(400, "audio.decode_mp3 failed: "+err.Error())
		}
		buf, err := decodeMP3(bytes.NewReader(data))
		if err != nil {
			return interpreter.NewError(400, "audio.decode_mp3 failed: "+err.Error())
		}
		return buf
	})

	// audio.read_wav(path: string) -> AudioBuffer
	reg("read_wav", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.read_wav requires file path")
		}
		path := interpreter.CoerceToString(args[0])
		f, err := os.Open(path)
		if err != nil {
			return interpreter.NewError(500, fmt.Sprintf("audio: failed to open %q: %v", path, err))
		}
		defer f.Close()
		buf, err := decodeWAV(f)
		if err != nil {
			return interpreter.NewError(400, fmt.Sprintf("audio: decoding WAV %q failed: %v", path, err))
		}
		return buf
	})

	// audio.write_wav(path: string, buffer: AudioBuffer, bit_depth?: int) -> bool
	reg("write_wav", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "audio.write_wav requires (path, buffer)")
		}
		path := interpreter.CoerceToString(args[0])
		buf, err := asAudioBuffer(args[1])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		bitDepth := ltIntArg(args, 2, 16)
		wavData, err := encodeWAV(buf, bitDepth)
		if err != nil {
			return interpreter.NewError(500, "audio: encoding WAV failed: "+err.Error())
		}
		if err := os.WriteFile(path, wavData, 0644); err != nil {
			return interpreter.NewError(500, fmt.Sprintf("audio: writing file %q failed: %v", path, err))
		}
		return interpreter.TRUE
	})

	// audio.encode_wav(buffer: AudioBuffer, bit_depth?: int) -> string (bytes)
	reg("encode_wav", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.encode_wav requires buffer")
		}
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		bitDepth := ltIntArg(args, 1, 16)
		data, err := encodeWAV(buf, bitDepth)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: string(data)}
	})

	// audio.decode_wav(bytes: string) -> AudioBuffer
	reg("decode_wav", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.decode_wav requires bytes string")
		}
		var data []byte
		if bv, ok := args[0].(*interpreter.BlobValue); ok {
			data = bv.Data
		} else {
			data = []byte(interpreter.CoerceToString(args[0]))
		}
		buf, err := decodeWAV(bytes.NewReader(data))
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		return buf
	})

	// audio.decode_base64(b64: string) -> AudioBuffer
	reg("decode_base64", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.decode_base64 requires base64 string")
		}
		raw := cleanBase64Audio(interpreter.CoerceToString(args[0]))
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return interpreter.NewError(400, "audio.decode_base64: invalid base64 data: "+err.Error())
		}
		buf, err := decodeWAV(bytes.NewReader(data))
		if err != nil {
			return interpreter.NewError(400, "audio.decode_base64: invalid WAV: "+err.Error())
		}
		return buf
	})

	// audio.encode_base64(buffer: AudioBuffer, bit_depth?: int) -> string
	reg("encode_base64", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.encode_base64 requires buffer")
		}
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		bitDepth := ltIntArg(args, 1, 16)
		data, err := encodeWAV(buf, bitDepth)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: base64.StdEncoding.EncodeToString(data)}
	})

	// -------------------------------------------------------------------------
	// Oscillators & Synthesis
	// -------------------------------------------------------------------------

	makeOscillator := func(fn func(phase float64) float64) func(args ...interpreter.Value) interpreter.Value {
		return func(args ...interpreter.Value) interpreter.Value {
			freq := ltFloatArg(args, 0, 440.0)
			durSec := ltFloatArg(args, 1, 1.0)
			rate := ltIntArg(args, 2, 44100)
			amp := ltFloatArg(args, 3, 0.8)

			numSamples := int(durSec * float64(rate))
			buf := &audioBufferValue{
				channels:   1,
				sampleRate: rate,
				samples:    [][]float64{make([]float64, numSamples)},
			}
			for i := 0; i < numSamples; i++ {
				t := float64(i) / float64(rate)
				phase := t * freq
				phase = phase - math.Floor(phase) // in [0, 1)
				buf.samples[0][i] = fn(phase) * amp
			}
			return buf
		}
	}

	// audio.sine(freq: float, duration: float, sample_rate?: int, amplitude?: float) -> AudioBuffer
	reg("sine", makeOscillator(func(phase float64) float64 {
		return math.Sin(phase * 2.0 * math.Pi)
	}))

	// audio.square(freq, duration, sample_rate?, amplitude?, duty?: float) -> AudioBuffer
	reg("square", func(args ...interpreter.Value) interpreter.Value {
		freq := ltFloatArg(args, 0, 440.0)
		durSec := ltFloatArg(args, 1, 1.0)
		rate := ltIntArg(args, 2, 44100)
		amp := ltFloatArg(args, 3, 0.8)
		duty := ltFloatArg(args, 4, 0.5)

		numSamples := int(durSec * float64(rate))
		buf := &audioBufferValue{
			channels:   1,
			sampleRate: rate,
			samples:    [][]float64{make([]float64, numSamples)},
		}
		for i := 0; i < numSamples; i++ {
			t := float64(i) / float64(rate)
			phase := t * freq
			phase = phase - math.Floor(phase)
			if phase < duty {
				buf.samples[0][i] = amp
			} else {
				buf.samples[0][i] = -amp
			}
		}
		return buf
	})

	// audio.sawtooth(freq, duration, sample_rate?, amplitude?) -> AudioBuffer
	reg("sawtooth", makeOscillator(func(phase float64) float64 {
		return 2.0*phase - 1.0
	}))

	// audio.triangle(freq, duration, sample_rate?, amplitude?) -> AudioBuffer
	reg("triangle", makeOscillator(func(phase float64) float64 {
		if phase < 0.5 {
			return 4.0*phase - 1.0
		}
		return 3.0 - 4.0*phase
	}))

	// audio.noise(duration, sample_rate?, amplitude?, color?: string) -> AudioBuffer
	reg("noise", func(args ...interpreter.Value) interpreter.Value {
		durSec := ltFloatArg(args, 0, 1.0)
		rate := ltIntArg(args, 1, 44100)
		amp := ltFloatArg(args, 2, 0.5)
		color := "white"
		if len(args) >= 4 {
			color = strings.ToLower(interpreter.CoerceToString(args[3]))
		}

		numSamples := int(durSec * float64(rate))
		buf := &audioBufferValue{
			channels:   1,
			sampleRate: rate,
			samples:    [][]float64{make([]float64, numSamples)},
		}

		if color == "pink" {
			// Paul Kellet's filtered pink noise generator
			b0, b1, b2, b3, b4, b5, b6 := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
			for i := 0; i < numSamples; i++ {
				white := rand.Float64()*2.0 - 1.0
				b0 = 0.99886*b0 + white*0.0555179
				b1 = 0.99332*b1 + white*0.0750759
				b2 = 0.96900*b2 + white*0.1538520
				b3 = 0.86650*b3 + white*0.3104856
				b4 = 0.55000*b4 + white*0.5329522
				b5 = -0.7616*b5 - white*0.0168980
				pink := b0 + b1 + b2 + b3 + b4 + b5 + b6 + white*0.5362
				b6 = white * 0.115926
				buf.samples[0][i] = (pink * 0.11) * amp
			}
		} else {
			for i := 0; i < numSamples; i++ {
				buf.samples[0][i] = (rand.Float64()*2.0 - 1.0) * amp
			}
		}
		return buf
	})

	// audio.silence(duration, sample_rate?, channels?) -> AudioBuffer
	reg("silence", func(args ...interpreter.Value) interpreter.Value {
		durSec := ltFloatArg(args, 0, 1.0)
		rate := ltIntArg(args, 1, 44100)
		channels := ltIntArg(args, 2, 1)
		numSamples := int(durSec * float64(rate))
		buf := &audioBufferValue{
			channels:   channels,
			sampleRate: rate,
			samples:    make([][]float64, channels),
		}
		for ch := 0; ch < channels; ch++ {
			buf.samples[ch] = make([]float64, numSamples)
		}
		return buf
	})

	// audio.chirp(start_freq, end_freq, duration, sample_rate?, amplitude?) -> AudioBuffer
	reg("chirp", func(args ...interpreter.Value) interpreter.Value {
		f0 := ltFloatArg(args, 0, 20.0)
		f1 := ltFloatArg(args, 1, 20000.0)
		durSec := ltFloatArg(args, 2, 2.0)
		rate := ltIntArg(args, 3, 44100)
		amp := ltFloatArg(args, 4, 0.8)

		numSamples := int(durSec * float64(rate))
		buf := &audioBufferValue{
			channels:   1,
			sampleRate: rate,
			samples:    [][]float64{make([]float64, numSamples)},
		}
		k := (f1 - f0) / durSec
		for i := 0; i < numSamples; i++ {
			t := float64(i) / float64(rate)
			phase := 2.0 * math.Pi * (f0*t + 0.5*k*t*t)
			buf.samples[0][i] = math.Sin(phase) * amp
		}
		return buf
	})

	// -------------------------------------------------------------------------
	// DSP, Filters, Envelope, Effects
	// -------------------------------------------------------------------------

	// audio.gain(buffer, factor: float) -> AudioBuffer
	reg("gain", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		factor := ltFloatArg(args, 1, 1.0)
		cp := buf.Clone().(*audioBufferValue)
		for ch := 0; ch < cp.channels; ch++ {
			for i := range cp.samples[ch] {
				cp.samples[ch][i] = math.Max(-1.0, math.Min(1.0, cp.samples[ch][i]*factor))
			}
		}
		return cp
	})

	// audio.normalize(buffer, target_peak?: float) -> AudioBuffer
	reg("normalize", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		targetPeak := ltFloatArg(args, 1, 0.98)
		maxPeak := 0.0
		for ch := 0; ch < buf.channels; ch++ {
			for _, s := range buf.samples[ch] {
				if a := math.Abs(s); a > maxPeak {
					maxPeak = a
				}
			}
		}
		cp := buf.Clone().(*audioBufferValue)
		if maxPeak > 1e-9 {
			scale := targetPeak / maxPeak
			for ch := 0; ch < cp.channels; ch++ {
				for i := range cp.samples[ch] {
					cp.samples[ch][i] *= scale
				}
			}
		}
		return cp
	})

	// audio.mix(buf1, buf2, ratio1?: float, ratio2?: float) -> AudioBuffer
	reg("mix", func(args ...interpreter.Value) interpreter.Value {
		b1, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		b2, err := asAudioBuffer(args[1])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		r1 := ltFloatArg(args, 2, 0.5)
		r2 := ltFloatArg(args, 3, 0.5)

		maxChannels := int(math.Max(float64(b1.channels), float64(b2.channels)))
		maxSamples := int(math.Max(float64(b1.numSamples()), float64(b2.numSamples())))

		out := &audioBufferValue{
			channels:   maxChannels,
			sampleRate: b1.sampleRate,
			samples:    make([][]float64, maxChannels),
		}
		for ch := 0; ch < maxChannels; ch++ {
			out.samples[ch] = make([]float64, maxSamples)
			for i := 0; i < maxSamples; i++ {
				s1 := 0.0
				if ch < b1.channels && i < b1.numSamples() {
					s1 = b1.samples[ch][i]
				}
				s2 := 0.0
				if ch < b2.channels && i < b2.numSamples() {
					s2 = b2.samples[ch][i]
				}
				out.samples[ch][i] = math.Max(-1.0, math.Min(1.0, s1*r1+s2*r2))
			}
		}
		return out
	})

	// audio.pan(buffer, pan_factor: float) -> AudioBuffer (converts mono to stereo with pan in [-1.0, 1.0])
	reg("pan", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		pan := math.Max(-1.0, math.Min(1.0, ltFloatArg(args, 1, 0.0)))
		leftGain := math.Cos((pan + 1.0) * math.Pi / 4.0)
		rightGain := math.Sin((pan + 1.0) * math.Pi / 4.0)

		numSamples := buf.numSamples()
		out := &audioBufferValue{
			channels:   2,
			sampleRate: buf.sampleRate,
			samples:    [][]float64{make([]float64, numSamples), make([]float64, numSamples)},
		}
		for i := 0; i < numSamples; i++ {
			mono := buf.samples[0][i]
			if buf.channels > 1 {
				mono = (buf.samples[0][i] + buf.samples[1][i]) * 0.5
			}
			out.samples[0][i] = mono * leftGain
			out.samples[1][i] = mono * rightGain
		}
		return out
	})

	// audio.delay(buffer, delay_sec: float, feedback: float, mix: float) -> AudioBuffer
	reg("delay", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		delaySec := ltFloatArg(args, 1, 0.3)
		feedback := math.Max(0.0, math.Min(0.95, ltFloatArg(args, 2, 0.4)))
		wetMix := math.Max(0.0, math.Min(1.0, ltFloatArg(args, 3, 0.5)))

		delaySamples := int(delaySec * float64(buf.sampleRate))
		if delaySamples < 1 {
			delaySamples = 1
		}

		cp := buf.Clone().(*audioBufferValue)
		for ch := 0; ch < cp.channels; ch++ {
			delayLine := make([]float64, delaySamples)
			dptr := 0
			for i := 0; i < len(cp.samples[ch]); i++ {
				in := cp.samples[ch][i]
				delayed := delayLine[dptr]
				delayLine[dptr] = in + delayed*feedback
				dptr = (dptr + 1) % delaySamples
				cp.samples[ch][i] = math.Max(-1.0, math.Min(1.0, in*(1.0-wetMix)+delayed*wetMix))
			}
		}
		return cp
	})

	// audio.lowpass(buffer, cutoff_hz: float, q?: float) -> AudioBuffer (Biquad Filter)
	reg("lowpass", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		cutoff := ltFloatArg(args, 1, 1000.0)
		q := ltFloatArg(args, 2, 0.7071)

		omega := 2.0 * math.Pi * cutoff / float64(buf.sampleRate)
		alpha := math.Sin(omega) / (2.0 * q)
		cosw := math.Cos(omega)

		b0 := (1.0 - cosw) / 2.0
		b1 := 1.0 - cosw
		b2 := (1.0 - cosw) / 2.0
		a0 := 1.0 + alpha
		a1 := -2.0 * cosw
		a2 := 1.0 - alpha

		cp := buf.Clone().(*audioBufferValue)
		for ch := 0; ch < cp.channels; ch++ {
			var x1, x2, y1, y2 float64
			for i, x := range cp.samples[ch] {
				y := (b0/a0)*x + (b1/a0)*x1 + (b2/a0)*x2 - (a1/a0)*y1 - (a2/a0)*y2
				x2, x1 = x1, x
				y2, y1 = y1, y
				cp.samples[ch][i] = math.Max(-1.0, math.Min(1.0, y))
			}
		}
		return cp
	})

	// audio.highpass(buffer, cutoff_hz: float, q?: float) -> AudioBuffer
	reg("highpass", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		cutoff := ltFloatArg(args, 1, 300.0)
		q := ltFloatArg(args, 2, 0.7071)

		omega := 2.0 * math.Pi * cutoff / float64(buf.sampleRate)
		alpha := math.Sin(omega) / (2.0 * q)
		cosw := math.Cos(omega)

		b0 := (1.0 + cosw) / 2.0
		b1 := -(1.0 + cosw)
		b2 := (1.0 + cosw) / 2.0
		a0 := 1.0 + alpha
		a1 := -2.0 * cosw
		a2 := 1.0 - alpha

		cp := buf.Clone().(*audioBufferValue)
		for ch := 0; ch < cp.channels; ch++ {
			var x1, x2, y1, y2 float64
			for i, x := range cp.samples[ch] {
				y := (b0/a0)*x + (b1/a0)*x1 + (b2/a0)*x2 - (a1/a0)*y1 - (a2/a0)*y2
				x2, x1 = x1, x
				y2, y1 = y1, y
				cp.samples[ch][i] = math.Max(-1.0, math.Min(1.0, y))
			}
		}
		return cp
	})

	// audio.adsr(buffer, attack_sec, decay_sec, sustain_level, release_sec) -> AudioBuffer
	reg("adsr", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		aSec := ltFloatArg(args, 1, 0.05)
		dSec := ltFloatArg(args, 2, 0.1)
		sLevel := math.Max(0.0, math.Min(1.0, ltFloatArg(args, 3, 0.7)))
		rSec := ltFloatArg(args, 4, 0.2)

		totalSamples := buf.numSamples()
		rate := float64(buf.sampleRate)
		aSamples := int(aSec * rate)
		dSamples := int(dSec * rate)
		rSamples := int(rSec * rate)
		sSamples := totalSamples - (aSamples + dSamples + rSamples)
		if sSamples < 0 {
			sSamples = 0
		}

		cp := buf.Clone().(*audioBufferValue)
		for i := 0; i < totalSamples; i++ {
			env := 0.0
			switch {
			case i < aSamples:
				env = float64(i) / float64(aSamples)
			case i < aSamples+dSamples:
				p := float64(i-aSamples) / float64(dSamples)
				env = 1.0 - p*(1.0-sLevel)
			case i < aSamples+dSamples+sSamples:
				env = sLevel
			default:
				p := float64(i-(aSamples+dSamples+sSamples)) / float64(rSamples)
				env = sLevel * (1.0 - math.Min(1.0, p))
			}
			for ch := 0; ch < cp.channels; ch++ {
				cp.samples[ch][i] *= env
			}
		}
		return cp
	})

	// -------------------------------------------------------------------------
	// FFT & Frequency Analysis
	// -------------------------------------------------------------------------

	// audio.fft(buffer_or_list, channel?: int) -> map{real, imag, magnitude, frequencies}
	reg("fft", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.fft requires audio buffer or samples list")
		}
		var samples []float64
		sampleRate := 44100
		if ab, ok := args[0].(*audioBufferValue); ok {
			ch := ltIntArg(args, 1, 0)
			if ch < 0 || ch >= ab.channels {
				ch = 0
			}
			samples = ab.samples[ch]
			sampleRate = ab.sampleRate
		} else if lv, ok := args[0].(*interpreter.ListValue); ok {
			elems := lv.ElementsSlice()
			samples = make([]float64, len(elems))
			for i, e := range elems {
				samples[i] = ltScalarFloat(e)
			}
		} else {
			return interpreter.NewError(400, "expected AudioBuffer or list of float samples")
		}

		if len(samples) == 0 {
			return interpreter.NewMap()
		}

		// Pad to next power of two
		n := nextPowerOfTwo(len(samples))
		real := make([]float64, n)
		imag := make([]float64, n)
		copy(real, samples)

		fftCooleyTukey(real, imag, false)

		halfN := n / 2
		mReal := make([]interpreter.Value, halfN)
		mImag := make([]interpreter.Value, halfN)
		mMagnitude := make([]interpreter.Value, halfN)
		mFreqs := make([]interpreter.Value, halfN)

		for i := 0; i < halfN; i++ {
			mReal[i] = &interpreter.FloatValue{Val: real[i]}
			mImag[i] = &interpreter.FloatValue{Val: imag[i]}
			mag := math.Sqrt(real[i]*real[i] + imag[i]*imag[i])
			mMagnitude[i] = &interpreter.FloatValue{Val: mag}
			freq := float64(i) * float64(sampleRate) / float64(n)
			mFreqs[i] = &interpreter.FloatValue{Val: freq}
		}

		out := interpreter.NewMap()
		out.Set("real", interpreter.NewList(mReal...))
		out.Set("imag", interpreter.NewList(mImag...))
		out.Set("magnitude", interpreter.NewList(mMagnitude...))
		out.Set("frequencies", interpreter.NewList(mFreqs...))
		return out
	})

	// audio.dominant_frequency(buffer, channel?: int) -> float
	reg("dominant_frequency", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		ch := ltIntArg(args, 1, 0)
		if ch < 0 || ch >= buf.channels {
			ch = 0
		}
		samples := buf.samples[ch]
		if len(samples) == 0 {
			return &interpreter.FloatValue{Val: 0}
		}
		n := nextPowerOfTwo(len(samples))
		real := make([]float64, n)
		imag := make([]float64, n)
		copy(real, samples)
		fftCooleyTukey(real, imag, false)

		maxMag := 0.0
		domFreq := 0.0
		halfN := n / 2
		for i := 1; i < halfN; i++ { // skip DC component at 0
			mag := real[i]*real[i] + imag[i]*imag[i]
			if mag > maxMag {
				maxMag = mag
				domFreq = float64(i) * float64(buf.sampleRate) / float64(n)
			}
		}
		return &interpreter.FloatValue{Val: domFreq}
	})

	// audio.rms(buffer, channel?: int) -> float
	reg("rms", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		ch := ltIntArg(args, 1, 0)
		if ch < 0 || ch >= buf.channels {
			ch = 0
		}
		sumSq := 0.0
		for _, s := range buf.samples[ch] {
			sumSq += s * s
		}
		rms := math.Sqrt(sumSq / float64(buf.numSamples()))
		return &interpreter.FloatValue{Val: rms}
	})

	// audio.peak(buffer, channel?: int) -> float
	reg("peak", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		ch := ltIntArg(args, 1, 0)
		if ch < 0 || ch >= buf.channels {
			ch = 0
		}
		peak := 0.0
		for _, s := range buf.samples[ch] {
			if a := math.Abs(s); a > peak {
				peak = a
			}
		}
		return &interpreter.FloatValue{Val: peak}
	})

	// audio.zero_crossings(buffer, channel?: int) -> int
	reg("zero_crossings", func(args ...interpreter.Value) interpreter.Value {
		buf, err := asAudioBuffer(args[0])
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		ch := ltIntArg(args, 1, 0)
		if ch < 0 || ch >= buf.channels {
			ch = 0
		}
		crossings := int64(0)
		samples := buf.samples[ch]
		for i := 1; i < len(samples); i++ {
			if (samples[i-1] >= 0 && samples[i] < 0) || (samples[i-1] < 0 && samples[i] >= 0) {
				crossings++
			}
		}
		return &interpreter.IntegerValue{Val: crossings}
	})

	// -------------------------------------------------------------------------
	// Real-time Playback / Output
	// -------------------------------------------------------------------------

	// audio.play(buffer_or_path_or_url_or_b64, async?: bool) -> bool
	reg("play", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.play requires (buffer | file_path | url | base64 | bytes)")
		}
		asyncPlay := ltBoolArg(args, 1, true)
		data, isWav, err := resolveAudioBytes(args[0])
		if err != nil {
			return interpreter.NewError(400, "audio.play failed: "+err.Error())
		}
		if err := playAudioSource(data, asyncPlay, isWav); err != nil {
			return interpreter.NewError(500, "audio: playback failed: "+err.Error())
		}
		return interpreter.TRUE
	})

	// audio.play_base64(b64_string: string, async?: bool) -> bool
	reg("play_base64", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "audio.play_base64 requires base64 string")
		}
		asyncPlay := ltBoolArg(args, 1, true)
		raw := cleanBase64Audio(interpreter.CoerceToString(args[0]))
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return interpreter.NewError(400, "audio.play_base64: invalid base64 string: "+err.Error())
		}
		isWav := len(data) >= 4 && string(data[:4]) == "RIFF"
		if err := playAudioSource(data, asyncPlay, isWav); err != nil {
			return interpreter.NewError(500, "audio: playback failed: "+err.Error())
		}
		return interpreter.TRUE
	})

	// audio.pause() -> null
	reg("pause", func(args ...interpreter.Value) interpreter.Value {
		pauseAudio()
		return interpreter.NULL
	})

	// audio.resume() -> null
	reg("resume", func(args ...interpreter.Value) interpreter.Value {
		resumeAudio()
		return interpreter.NULL
	})

	// audio.volume(vol_0_to_100: int) -> null
	reg("volume", func(args ...interpreter.Value) interpreter.Value {
		vol := ltIntArg(args, 0, 100)
		setAudioVolume(vol)
		return interpreter.NULL
	})

	// audio.stop() -> null
	reg("stop", func(args ...interpreter.Value) interpreter.Value {
		stopAudio()
		return interpreter.NULL
	})

	// audio.beep(freq_hz?: int, duration_ms?: int) -> null
	reg("beep", func(args ...interpreter.Value) interpreter.Value {
		freq := ltIntArg(args, 0, 750)
		dur := ltIntArg(args, 1, 300)
		systemBeep(freq, dur)
		return interpreter.NULL
	})

	env.Set("audio", mod)
	env.Set("std.audio", mod)
}
