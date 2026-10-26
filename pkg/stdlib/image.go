// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// imageValue wraps an RGBA image as an interpreter value.
type imageValue struct {
	img *image.RGBA
}

func (v *imageValue) Type() interpreter.ValueType { return "IMAGE" }
func (v *imageValue) Inspect() string {
	b := v.img.Bounds()
	return fmt.Sprintf("<image width=%d height=%d>", b.Dx(), b.Dy())
}
func (v *imageValue) Clone() interpreter.Value {
	b := v.img.Bounds()
	cp := image.NewRGBA(b)
	copy(cp.Pix, v.img.Pix)
	return &imageValue{img: cp}
}

// CallMethod implements interpreter.MethodProvider.
func (v *imageValue) CallMethod(name string, args []interpreter.Value) (interpreter.Value, error) {
	b := v.img.Bounds()
	w, h := b.Dx(), b.Dy()

	switch name {
	case "width":
		return &interpreter.IntegerValue{Val: int64(w)}, nil
	case "height":
		return &interpreter.IntegerValue{Val: int64(h)}, nil
	case "bounds", "size":
		return interpreter.NewList(&interpreter.IntegerValue{Val: int64(w)}, &interpreter.IntegerValue{Val: int64(h)}), nil

	case "pixel":
		if len(args) < 2 {
			return interpreter.NewError(400, "pixel requires (x, y)"), nil
		}
		x := int(ltIntArg(args, 0, 0))
		y := int(ltIntArg(args, 1, 0))
		if x < 0 || x >= w || y < 0 || y >= h {
			return interpreter.NewError(400, fmt.Sprintf("pixel coordinate (%d, %d) out of bounds (%dx%d)", x, y, w, h)), nil
		}
		c := v.img.RGBAAt(x, y)
		return interpreter.NewList(
			&interpreter.IntegerValue{Val: int64(c.R)},
			&interpreter.IntegerValue{Val: int64(c.G)},
			&interpreter.IntegerValue{Val: int64(c.B)},
			&interpreter.IntegerValue{Val: int64(c.A)},
		), nil

	case "set_pixel":
		if len(args) < 5 {
			return interpreter.NewError(400, "set_pixel requires (x, y, r, g, b, [a])"), nil
		}
		x := int(ltIntArg(args, 0, 0))
		y := int(ltIntArg(args, 1, 0))
		if x < 0 || x >= w || y < 0 || y >= h {
			return interpreter.NewError(400, fmt.Sprintf("pixel coordinate (%d, %d) out of bounds", x, y)), nil
		}
		r := uint8(ltIntArg(args, 2, 0))
		g := uint8(ltIntArg(args, 3, 0))
		b_val := uint8(ltIntArg(args, 4, 0))
		a := uint8(255)
		if len(args) >= 6 {
			a = uint8(ltIntArg(args, 5, 255))
		}
		v.img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b_val, A: a})
		return interpreter.TRUE, nil

	case "resize":
		if len(args) < 2 {
			return interpreter.NewError(400, "resize requires (new_width, new_height)"), nil
		}
		nw := ltIntArg(args, 0, w)
		nh := ltIntArg(args, 1, h)
		if nw <= 0 || nh <= 0 {
			return interpreter.NewError(400, "dimensions must be positive"), nil
		}
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		xdraw.CatmullRom.Scale(dst, dst.Rect, v.img, v.img.Bounds(), draw.Over, nil)
		return &imageValue{img: dst}, nil

	case "crop":
		if len(args) < 4 {
			return interpreter.NewError(400, "crop requires (x, y, width, height)"), nil
		}
		x := int(ltIntArg(args, 0, 0))
		y := int(ltIntArg(args, 1, 0))
		cw := int(ltIntArg(args, 2, 0))
		ch := int(ltIntArg(args, 3, 0))
		if cw <= 0 || ch <= 0 {
			return interpreter.NewError(400, "crop dimensions must be positive"), nil
		}
		dst := image.NewRGBA(image.Rect(0, 0, cw, ch))
		draw.Draw(dst, dst.Rect, v.img, image.Pt(x, y), draw.Src)
		return &imageValue{img: dst}, nil

	case "grayscale":
		dst := image.NewRGBA(v.img.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := v.img.RGBAAt(x, y)
				gray := uint8(0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B))
				dst.SetRGBA(x, y, color.RGBA{R: gray, G: gray, B: gray, A: c.A})
			}
		}
		return &imageValue{img: dst}, nil

	case "invert":
		dst := image.NewRGBA(v.img.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := v.img.RGBAAt(x, y)
				dst.SetRGBA(x, y, color.RGBA{R: 255 - c.R, G: 255 - c.G, B: 255 - c.B, A: c.A})
			}
		}
		return &imageValue{img: dst}, nil

	case "flip_h":
		dst := image.NewRGBA(v.img.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(w-1-x, y, v.img.RGBAAt(x, y))
			}
		}
		return &imageValue{img: dst}, nil

	case "flip_v":
		dst := image.NewRGBA(v.img.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(x, h-1-y, v.img.RGBAAt(x, y))
			}
		}
		return &imageValue{img: dst}, nil

	case "rotate":
		angle := int(ltIntArg(args, 0, 90)) % 360
		if angle < 0 {
			angle += 360
		}
		switch angle {
		case 90:
			dst := image.NewRGBA(image.Rect(0, 0, h, w))
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					dst.SetRGBA(h-1-y, x, v.img.RGBAAt(x, y))
				}
			}
			return &imageValue{img: dst}, nil
		case 180:
			dst := image.NewRGBA(image.Rect(0, 0, w, h))
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					dst.SetRGBA(w-1-x, h-1-y, v.img.RGBAAt(x, y))
				}
			}
			return &imageValue{img: dst}, nil
		case 270:
			dst := image.NewRGBA(image.Rect(0, 0, h, w))
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					dst.SetRGBA(y, w-1-x, v.img.RGBAAt(x, y))
				}
			}
			return &imageValue{img: dst}, nil
		default:
			return v.Clone(), nil
		}

	case "brightness":
		delta := ltFloatArg(args, 0, 0.0)
		dst := image.NewRGBA(v.img.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := v.img.RGBAAt(x, y)
				r := uint8(math.Max(0, math.Min(255, float64(c.R)+delta)))
				g := uint8(math.Max(0, math.Min(255, float64(c.G)+delta)))
				bv := uint8(math.Max(0, math.Min(255, float64(c.B)+delta)))
				dst.SetRGBA(x, y, color.RGBA{R: r, G: g, B: bv, A: c.A})
			}
		}
		return &imageValue{img: dst}, nil

	case "contrast":
		factor := ltFloatArg(args, 0, 1.0)
		dst := image.NewRGBA(v.img.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := v.img.RGBAAt(x, y)
				r := uint8(math.Max(0, math.Min(255, (float64(c.R)-128.0)*factor+128.0)))
				g := uint8(math.Max(0, math.Min(255, (float64(c.G)-128.0)*factor+128.0)))
				bv := uint8(math.Max(0, math.Min(255, (float64(c.B)-128.0)*factor+128.0)))
				dst.SetRGBA(x, y, color.RGBA{R: r, G: g, B: bv, A: c.A})
			}
		}
		return &imageValue{img: dst}, nil

	case "to_base64":
		format := "png"
		if len(args) >= 1 {
			format = strings.ToLower(interpreter.CoerceToString(args[0]))
		}
		data, err := encodeImageBytes(v.img, format, 90)
		if err != nil {
			return interpreter.NewError(500, err.Error()), nil
		}
		return &interpreter.StringValue{Val: base64.StdEncoding.EncodeToString(data)}, nil

	default:
		return nil, fmt.Errorf("image has no method %q", name)
	}
}

// cleanBase64Image removes data:image/...;base64, header if present.
func cleanBase64Image(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, ","); idx != -1 && strings.Contains(s[:idx], ";base64") {
		s = s[idx+1:]
	}
	return strings.TrimSpace(s)
}

func decodeImageAny(data []byte) (*image.RGBA, error) {
	// Try standard image.Decode
	img, _, err := image.Decode(bytes.NewReader(data))
	if err == nil {
		return toRGBA(img), nil
	}

	// Try WebP decoder
	if webpImg, err := webp.Decode(bytes.NewReader(data)); err == nil {
		return toRGBA(webpImg), nil
	}

	// Try BMP decoder
	if bmpImg, err := bmp.Decode(bytes.NewReader(data)); err == nil {
		return toRGBA(bmpImg), nil
	}

	return nil, fmt.Errorf("image: unsupported or corrupted image format")
}

func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Rect, src, b.Min, draw.Src)
	return dst
}

func encodeImageBytes(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer
	format = strings.ToLower(strings.TrimPrefix(format, "."))

	switch format {
	case "jpeg", "jpg":
		if quality < 1 {
			quality = 85
		}
		if quality > 100 {
			quality = 100
		}
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
		return buf.Bytes(), err

	case "gif":
		err := gif.Encode(&buf, img, nil)
		return buf.Bytes(), err

	case "bmp":
		err := bmp.Encode(&buf, img)
		return buf.Bytes(), err

	case "png", "":
		err := png.Encode(&buf, img)
		return buf.Bytes(), err

	default:
		return nil, fmt.Errorf("image: unsupported export format %q (use png, jpeg, gif, bmp)", format)
	}
}

func resolveImageBytes(val interpreter.Value) ([]byte, error) {
	if iv, ok := val.(*imageValue); ok {
		return encodeImageBytes(iv.img, "png", 90)
	}
	if bv, ok := val.(*interpreter.BlobValue); ok {
		return bv.Data, nil
	}
	str := interpreter.CoerceToString(val)
	s := strings.TrimSpace(str)

	// URL
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(s)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch image URL %q: %w", s, err)
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}

	// Base64
	cleaned := cleanBase64Image(s)
	if b64, err := base64.StdEncoding.DecodeString(cleaned); err == nil && len(b64) >= 8 {
		return b64, nil
	}

	// File on disk
	if data, err := os.ReadFile(str); err == nil {
		return data, nil
	}

	// Raw bytes in string
	return []byte(str), nil
}

// RegisterImageBuiltins registers "std.image" and "image" into environment.
func RegisterImageBuiltins(env *interpreter.Environment) {
	mod := interpreter.NewMap()

	reg := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		mod.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// image.create(width: int, height: int, fill_color?: list[int]) -> Image
	reg("create", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "image.create requires (width, height)")
		}
		w := int(ltIntArg(args, 0, 100))
		h := int(ltIntArg(args, 1, 100))
		if w <= 0 || h <= 0 {
			return interpreter.NewError(400, "image dimensions must be positive")
		}
		img := image.NewRGBA(image.Rect(0, 0, w, h))

		if len(args) >= 3 {
			if colList, ok := args[2].(*interpreter.ListValue); ok {
				elems := colList.ElementsSlice()
				r := uint8(0)
				g := uint8(0)
				b_val := uint8(0)
				a := uint8(255)
				if len(elems) > 0 {
					if iv, ok := elems[0].(*interpreter.IntegerValue); ok {
						r = uint8(iv.Val)
					}
				}
				if len(elems) > 1 {
					if iv, ok := elems[1].(*interpreter.IntegerValue); ok {
						g = uint8(iv.Val)
					}
				}
				if len(elems) > 2 {
					if iv, ok := elems[2].(*interpreter.IntegerValue); ok {
						b_val = uint8(iv.Val)
					}
				}
				if len(elems) > 3 {
					if iv, ok := elems[3].(*interpreter.IntegerValue); ok {
						a = uint8(iv.Val)
					}
				}
				draw.Draw(img, img.Rect, &image.Uniform{C: color.RGBA{R: r, G: g, B: b_val, A: a}}, image.Point{}, draw.Src)
			}
		}

		return &imageValue{img: img}
	})

	// image.read(source: string|blob) -> Image (universal: file, url, base64)
	reg("read", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "image.read requires (path | url | base64)")
		}
		data, err := resolveImageBytes(args[0])
		if err != nil {
			return interpreter.NewError(400, "image.read failed: "+err.Error())
		}
		img, err := decodeImageAny(data)
		if err != nil {
			return interpreter.NewError(400, "image.read: decode failed: "+err.Error())
		}
		return &imageValue{img: img}
	})

	// image.decode(data: string|blob) -> Image
	reg("decode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "image.decode requires bytes or base64")
		}
		data, err := resolveImageBytes(args[0])
		if err != nil {
			return interpreter.NewError(400, "image.decode failed: "+err.Error())
		}
		img, err := decodeImageAny(data)
		if err != nil {
			return interpreter.NewError(400, "image.decode failed: "+err.Error())
		}
		return &imageValue{img: img}
	})

	// image.write(img: Image, filepath: string, quality?: int) -> bool
	reg("write", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "image.write requires (image, filepath)")
		}
		iv, ok := args[0].(*imageValue)
		if !ok {
			return interpreter.NewError(400, "image.write: expected Image as first argument")
		}
		path := interpreter.CoerceToString(args[1])
		quality := int(ltIntArg(args, 2, 85))

		format := "png"
		if idx := strings.LastIndex(path, "."); idx != -1 {
			format = path[idx+1:]
		}

		data, err := encodeImageBytes(iv.img, format, quality)
		if err != nil {
			return interpreter.NewError(500, "image.write encode failed: "+err.Error())
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return interpreter.NewError(500, fmt.Sprintf("image.write: failed to save to %q: %v", path, err))
		}
		return interpreter.TRUE
	})

	// image.encode(img: Image, format?: string, quality?: int) -> string (bytes)
	reg("encode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "image.encode requires image")
		}
		iv, ok := args[0].(*imageValue)
		if !ok {
			return interpreter.NewError(400, "image.encode: expected Image")
		}
		format := "png"
		if len(args) >= 2 {
			format = interpreter.CoerceToString(args[1])
		}
		quality := int(ltIntArg(args, 2, 85))

		data, err := encodeImageBytes(iv.img, format, quality)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: string(data)}
	})

	// image.to_base64(img: Image, format?: string) -> string
	reg("to_base64", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewError(400, "image.to_base64 requires image")
		}
		iv, ok := args[0].(*imageValue)
		if !ok {
			return interpreter.NewError(400, "image.to_base64: expected Image")
		}
		format := "png"
		if len(args) >= 2 {
			format = interpreter.CoerceToString(args[1])
		}
		data, err := encodeImageBytes(iv.img, format, 85)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: base64.StdEncoding.EncodeToString(data)}
	})

	// image.screenshot(x?: int, y?: int, w?: int, h?: int) -> Image
	reg("screenshot", func(args ...interpreter.Value) interpreter.Value {
		x := 0
		y := 0
		w := 0
		h := 0
		if len(args) >= 4 {
			x = int(ltIntArg(args, 0, 0))
			y = int(ltIntArg(args, 1, 0))
			w = int(ltIntArg(args, 2, 0))
			h = int(ltIntArg(args, 3, 0))
		}
		img, err := captureScreen(x, y, w, h)
		if err != nil {
			return interpreter.NewError(500, "image.screenshot failed: "+err.Error())
		}
		return &imageValue{img: img}
	})

	env.Set("image", mod)
	env.Set("std.image", mod)
}
