// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package stdlib

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/alca-lang/alca/pkg/interpreter"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/ianaindex"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

// lookupEncoding returns the encoding.Encoding for a name/alias.
func lookupEncoding(name string) (encoding.Encoding, string, error) {
	norm := strings.ToLower(strings.TrimSpace(name))
	norm = strings.ReplaceAll(norm, "_", "-")

	switch norm {
	case "utf-8", "utf8":
		return encoding.Nop, "utf-8", nil
	case "utf-16", "utf-16le", "utf16", "utf16le":
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM), "utf-16le", nil
	case "utf-16be", "utf16be":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM), "utf-16be", nil
	case "windows-1251", "cp1251", "1251", "cp-1251":
		return charmap.Windows1251, "windows-1251", nil
	case "windows-1252", "cp1252", "1252", "cp-1252":
		return charmap.Windows1252, "windows-1252", nil
	case "windows-1250", "cp1250", "1250":
		return charmap.Windows1250, "windows-1250", nil
	case "windows-1253", "cp1253", "1253":
		return charmap.Windows1253, "windows-1253", nil
	case "windows-1254", "cp1254", "1254":
		return charmap.Windows1254, "windows-1254", nil
	case "windows-1255", "cp1255", "1255":
		return charmap.Windows1255, "windows-1255", nil
	case "windows-1256", "cp1256", "1256":
		return charmap.Windows1256, "windows-1256", nil
	case "windows-1257", "cp1257", "1257":
		return charmap.Windows1257, "windows-1257", nil
	case "windows-1258", "cp1258", "1258":
		return charmap.Windows1258, "windows-1258", nil
	case "cp866", "ibm866", "866":
		return charmap.CodePage866, "cp866", nil
	case "cp437", "ibm437", "437":
		return charmap.CodePage437, "cp437", nil
	case "cp850", "ibm850", "850":
		return charmap.CodePage850, "cp850", nil
	case "cp852", "ibm852", "852":
		return charmap.CodePage852, "cp852", nil
	case "koi8-r", "koi8r":
		return charmap.KOI8R, "koi8-r", nil
	case "koi8-u", "koi8u":
		return charmap.KOI8U, "koi8-u", nil
	case "iso-8859-1", "latin-1", "latin1":
		return charmap.ISO8859_1, "iso-8859-1", nil
	case "iso-8859-2", "latin-2", "latin2":
		return charmap.ISO8859_2, "iso-8859-2", nil
	case "iso-8859-5":
		return charmap.ISO8859_5, "iso-8859-5", nil
	case "iso-8859-15", "latin-9", "latin9":
		return charmap.ISO8859_15, "iso-8859-15", nil
	case "mac-cyrillic", "maccyrillic":
		return charmap.MacintoshCyrillic, "mac-cyrillic", nil
	case "macintosh", "mac":
		return charmap.Macintosh, "macintosh", nil
	case "shift-jis", "shift_jis", "sjis":
		return japanese.ShiftJIS, "shift-jis", nil
	case "euc-jp", "eucjp":
		return japanese.EUCJP, "euc-jp", nil
	case "iso-2022-jp":
		return japanese.ISO2022JP, "iso-2022-jp", nil
	case "gbk":
		return simplifiedchinese.GBK, "gbk", nil
	case "gb18030":
		return simplifiedchinese.GB18030, "gb18030", nil
	case "big5", "big-5":
		return traditionalchinese.Big5, "big5", nil
	case "euc-kr", "euckr":
		return korean.EUCKR, "euc-kr", nil
	}

	// Try IANA / MIME index
	if enc, err := ianaindex.IANA.Encoding(norm); err == nil && enc != nil {
		return enc, norm, nil
	}
	if enc, err := ianaindex.MIME.Encoding(norm); err == nil && enc != nil {
		return enc, norm, nil
	}

	return nil, "", fmt.Errorf("encoding: unsupported or unknown character set %q", name)
}

// supportedEncodingList returns all canonical names of supported charsets.
var supportedEncodingList = []string{
	"big5",
	"cp437",
	"cp850",
	"cp852",
	"cp866",
	"euc-jp",
	"euc-kr",
	"gb18030",
	"gbk",
	"iso-2022-jp",
	"iso-8859-1",
	"iso-8859-2",
	"iso-8859-3",
	"iso-8859-4",
	"iso-8859-5",
	"iso-8859-6",
	"iso-8859-7",
	"iso-8859-8",
	"iso-8859-9",
	"iso-8859-10",
	"iso-8859-13",
	"iso-8859-14",
	"iso-8859-15",
	"iso-8859-16",
	"koi8-r",
	"koi8-u",
	"mac-cyrillic",
	"macintosh",
	"shift-jis",
	"utf-16be",
	"utf-16le",
	"utf-8",
	"windows-1250",
	"windows-1251",
	"windows-1252",
	"windows-1253",
	"windows-1254",
	"windows-1255",
	"windows-1256",
	"windows-1257",
	"windows-1258",
}

// detectEncoding checks BOM or common patterns in raw bytes.
func detectEncoding(data []byte) string {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return "utf-8"
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return "utf-16le"
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return "utf-16be"
	}
	if utf8.Valid(data) {
		return "utf-8"
	}
	// Check for common Cyrillic Windows-1251 range (0xC0-0xFF)
	cyrillicScore1251 := 0
	cyrillicScore866 := 0
	cyrillicScoreKOI8 := 0
	for _, b := range data {
		if b >= 0xC0 && b <= 0xFF {
			cyrillicScore1251++
		}
		if (b >= 0x80 && b <= 0xAF) || (b >= 0xE0 && b <= 0xF1) {
			cyrillicScore866++
		}
		if b >= 0xC0 && b <= 0xFF {
			cyrillicScoreKOI8++
		}
	}
	if cyrillicScore1251 > 0 && cyrillicScore1251 >= cyrillicScore866 {
		return "windows-1251"
	}
	if cyrillicScore866 > 0 {
		return "cp866"
	}
	return "windows-1252"
}

// RegisterEncodingBuiltins registers "std.encoding" and "encoding" into environment.
func RegisterEncodingBuiltins(env *interpreter.Environment) {
	mod := interpreter.NewMap()

	reg := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		mod.Set(name, &interpreter.BuiltinFunction{Name: name, Fn: fn})
	}

	// encoding.decode(bytes: string|blob, charset: string) -> string
	reg("decode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "encoding.decode requires (bytes, charset)")
		}
		rawBytes := []byte(interpreter.CoerceToString(args[0]))
		charset := interpreter.CoerceToString(args[1])

		enc, _, err := lookupEncoding(charset)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		if enc == encoding.Nop {
			return &interpreter.StringValue{Val: string(rawBytes)}
		}

		reader := enc.NewDecoder().Reader(bytes.NewReader(rawBytes))
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return interpreter.NewError(400, fmt.Sprintf("encoding: decoding %s failed: %v", charset, err))
		}
		return &interpreter.StringValue{Val: string(decoded)}
	})

	// encoding.encode(text: string, charset: string) -> string (byte string)
	reg("encode", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "encoding.encode requires (text, charset)")
		}
		text := interpreter.CoerceToString(args[0])
		charset := interpreter.CoerceToString(args[1])

		enc, _, err := lookupEncoding(charset)
		if err != nil {
			return interpreter.NewError(400, err.Error())
		}
		if enc == encoding.Nop {
			return &interpreter.StringValue{Val: text}
		}

		var buf bytes.Buffer
		writer := enc.NewEncoder().Writer(&buf)
		if _, err := writer.Write([]byte(text)); err != nil {
			return interpreter.NewError(400, fmt.Sprintf("encoding: encoding %s failed: %v", charset, err))
		}
		return &interpreter.StringValue{Val: buf.String()}
	})

	// encoding.list_encodings() -> list[string]
	reg("list_encodings", func(args ...interpreter.Value) interpreter.Value {
		elems := make([]interpreter.Value, len(supportedEncodingList))
		for i, name := range supportedEncodingList {
			elems[i] = &interpreter.StringValue{Val: name}
		}
		return interpreter.NewList(elems...)
	})

	// encoding.detect(bytes: string) -> string
	reg("detect", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: "utf-8"}
		}
		raw := []byte(interpreter.CoerceToString(args[0]))
		return &interpreter.StringValue{Val: detectEncoding(raw)}
	})

	// encoding.to_utf16(text: string) -> list[int] (code points)
	reg("to_utf16", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return interpreter.NewList()
		}
		text := interpreter.CoerceToString(args[0])
		runes := []rune(text)
		u16 := utf16.Encode(runes)
		elems := make([]interpreter.Value, len(u16))
		for i, code := range u16 {
			elems[i] = &interpreter.IntegerValue{Val: int64(code)}
		}
		return interpreter.NewList(elems...)
	})

	// encoding.from_utf16(codes: list[int]) -> string
	reg("from_utf16", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 1 {
			return &interpreter.StringValue{Val: ""}
		}
		lv, ok := args[0].(*interpreter.ListValue)
		if !ok {
			return interpreter.NewError(400, "from_utf16 requires list of integer codes")
		}
		slice := lv.ElementsSlice()
		u16 := make([]uint16, len(slice))
		for i, elem := range slice {
			if iv, ok := elem.(*interpreter.IntegerValue); ok {
				u16[i] = uint16(iv.Val)
			}
		}
		runes := utf16.Decode(u16)
		return &interpreter.StringValue{Val: string(runes)}
	})

	sort.Strings(supportedEncodingList)
	env.Set("encoding", mod)
	env.Set("std.encoding", mod)
}
