// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Chunk types in Android Binary XML.
const (
	ResXMLType            uint16 = 0x0003
	ResStringPoolType     uint16 = 0x0001
	ResXMLResourceMapType uint16 = 0x0180
	ResXMLStartNamespace  uint16 = 0x0100
	ResXMLEndNamespace    uint16 = 0x0101
	ResXMLStartElement    uint16 = 0x0102
	ResXMLEndElement      uint16 = 0x0103
	ResXMLCData           uint16 = 0x0104

	TypeReference uint8 = 0x01
	TypeString    uint8 = 0x03
	TypeIntDec    uint8 = 0x10
	TypeIntHex    uint8 = 0x11
	TypeIntBool   uint8 = 0x12

	AndroidNS = "http://schemas.android.com/apk/res/android"
)

// Known Android resource IDs for common manifest attributes.
var knownAttrResIDs = map[string]uint32{
	"theme":                0x01010000,
	"label":                0x01010001,
	"icon":                 0x01010002,
	"name":                 0x01010003,
	"permission":           0x01010006,
	"exported":             0x01010010,
	"minSdkVersion":        0x0101020c,
	"versionCode":          0x0101021b,
	"versionName":          0x0101021c,
	"targetSdkVersion":     0x01010270,
	"allowBackup":          0x01010280,
	"supportsRtl":          0x010103af,
	"screenOrientation":    0x0101001e,
	"configChanges":        0x0101001f,
	"usesCleartextTraffic": 0x010104ec,
	"hardwareAccelerated":  0x010102d3,
	"windowSoftInputMode":  0x0101022b,
	"scheme":               0x01010027,
	"host":                 0x01010028,
	"path":                 0x01010029,
	"mimeType":             0x01010026,
	"value":                0x01010024,
	"required":             0x0101028e,
}

// Known Android framework theme resource IDs for NoTitleBar and NoActionBar themes.
var knownThemeResIDs = map[string]uint32{
	"@android:style/Theme.NoTitleBar":                           0x01030005,
	"@android:style/Theme.NoTitleBar.Fullscreen":                0x01030007,
	"@android:style/Theme.Black.NoTitleBar":                     0x01030008,
	"@android:style/Theme.Black.NoTitleBar.Fullscreen":          0x01030009,
	"@android:style/Theme.DeviceDefault.NoActionBar":            0x010301f0,
	"@android:style/Theme.DeviceDefault.NoActionBar.Fullscreen": 0x010301f1,
	"@android:style/Theme.Material.NoActionBar":                 0x01030224,
	"@android:style/Theme.Material.NoActionBar.Fullscreen":      0x01030225,
	"@android:style/Theme.Translucent.NoTitleBar":               0x0103000f,
}

// XMLElement is a node in the parsed XML tree.
type XMLElement struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Children []*XMLElement
}

// ParseXMLTree parses XML data into an XMLElement hierarchy.
func ParseXMLTree(data []byte) (*XMLElement, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *XMLElement
	var stack []*XMLElement

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			elem := &XMLElement{
				Name:  t.Name,
				Attrs: t.Attr,
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, elem)
			} else if root == nil {
				root = elem
			}
			stack = append(stack, elem)

		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if root == nil {
		return nil, fmt.Errorf("empty XML document")
	}
	return root, nil
}

// stringPoolBuilder manages strings collected for AXML string pool.
type stringPoolBuilder struct {
	strings []string
	index   map[string]uint32
}

func newStringPoolBuilder() *stringPoolBuilder {
	return &stringPoolBuilder{
		index: make(map[string]uint32),
	}
}

func (sp *stringPoolBuilder) add(s string) uint32 {
	if idx, exists := sp.index[s]; exists {
		return idx
	}
	idx := uint32(len(sp.strings))
	sp.strings = append(sp.strings, s)
	sp.index[s] = idx
	return idx
}

func isXMLNamespaceAttr(attr xml.Attr) bool {
	return attr.Name.Local == "xmlns" || attr.Name.Space == "xmlns" || strings.HasPrefix(attr.Name.Local, "xmlns:")
}

// CompileAXML compiles a standard XML string (like AndroidManifest.xml) into Android Binary XML (AXML).
func CompileAXML(xmlData []byte) ([]byte, error) {
	root, err := ParseXMLTree(xmlData)
	if err != nil {
		return nil, fmt.Errorf("parsing XML: %w", err)
	}

	sp := newStringPoolBuilder()

	// Ensure standard strings are registered at fixed indexes 0 and 1
	sp.add(AndroidNS)
	sp.add("android")

	// Collect all strings from elements and attributes in advance
	var collectStrings func(node *XMLElement)
	collectStrings = func(node *XMLElement) {
		sp.add(node.Name.Local)
		for _, attr := range node.Attrs {
			if isXMLNamespaceAttr(attr) {
				continue
			}
			localName := strings.TrimPrefix(attr.Name.Local, "android:")
			sp.add(localName)
			sp.add(attr.Value)
		}
		for _, child := range node.Children {
			collectStrings(child)
		}
	}
	collectStrings(root)

	// Build String Pool Chunk (UTF-16 format for maximum Android compatibility)
	var spBuf bytes.Buffer
	stringCount := uint32(len(sp.strings))
	offsets := make([]uint32, stringCount)
	var stringData bytes.Buffer

	for i, s := range sp.strings {
		offsets[i] = uint32(stringData.Len())
		runes := []rune(s)
		// UTF-16 length prefix (uint16)
		_ = binary.Write(&stringData, binary.LittleEndian, uint16(len(runes)))
		for _, r := range runes {
			_ = binary.Write(&stringData, binary.LittleEndian, uint16(r))
		}
		// Null terminator (uint16)
		_ = binary.Write(&stringData, binary.LittleEndian, uint16(0))
	}

	// Align stringData to 4 bytes
	for stringData.Len()%4 != 0 {
		stringData.WriteByte(0)
	}

	headerSize := uint16(28)
	stringsStart := uint32(headerSize) + stringCount*4
	poolChunkSize := stringsStart + uint32(stringData.Len())

	// Write String Pool Header
	_ = binary.Write(&spBuf, binary.LittleEndian, ResStringPoolType)
	_ = binary.Write(&spBuf, binary.LittleEndian, headerSize)
	_ = binary.Write(&spBuf, binary.LittleEndian, poolChunkSize)
	_ = binary.Write(&spBuf, binary.LittleEndian, stringCount)
	_ = binary.Write(&spBuf, binary.LittleEndian, uint32(0)) // styleCount
	_ = binary.Write(&spBuf, binary.LittleEndian, uint32(0)) // flags (0 = UTF-16)
	_ = binary.Write(&spBuf, binary.LittleEndian, stringsStart)
	_ = binary.Write(&spBuf, binary.LittleEndian, uint32(0)) // stylesStart

	// Write string offsets
	for _, off := range offsets {
		_ = binary.Write(&spBuf, binary.LittleEndian, off)
	}
	// Write string bytes
	spBuf.Write(stringData.Bytes())

	// Build Resource Map Chunk
	var resMapBuf bytes.Buffer
	var resIDs []uint32
	for _, s := range sp.strings {
		if id, ok := knownAttrResIDs[s]; ok {
			resIDs = append(resIDs, id)
		} else {
			resIDs = append(resIDs, 0)
		}
	}
	resMapChunkSize := uint32(8 + len(resIDs)*4)
	_ = binary.Write(&resMapBuf, binary.LittleEndian, ResXMLResourceMapType)
	_ = binary.Write(&resMapBuf, binary.LittleEndian, uint16(8))
	_ = binary.Write(&resMapBuf, binary.LittleEndian, resMapChunkSize)
	for _, id := range resIDs {
		_ = binary.Write(&resMapBuf, binary.LittleEndian, id)
	}

	// Build XML Tree
	var treeBuf bytes.Buffer
	line := uint32(1)

	// Start Namespace (xmlns:android="http://schemas.android.com/apk/res/android")
	prefixIdx := uint32(1) // "android"
	uriIdx := uint32(0)    // AndroidNS

	writeChunkHeader(&treeBuf, ResXMLStartNamespace, 0x10, 0x18)
	_ = binary.Write(&treeBuf, binary.LittleEndian, line)
	_ = binary.Write(&treeBuf, binary.LittleEndian, uint32(0xFFFFFFFF)) // comment
	_ = binary.Write(&treeBuf, binary.LittleEndian, prefixIdx)
	_ = binary.Write(&treeBuf, binary.LittleEndian, uriIdx)

	// Write Element Tree recursively
	var writeElem func(elem *XMLElement)
	writeElem = func(elem *XMLElement) {
		line++
		elemNameIdx := sp.index[elem.Name.Local]
		elemUriIdx := uint32(0xFFFFFFFF)

		// Filter out xmlns namespace pseudo-attributes
		var validAttrs []xml.Attr
		for _, attr := range elem.Attrs {
			if isXMLNamespaceAttr(attr) {
				continue
			}
			validAttrs = append(validAttrs, attr)
		}

		attrCount := uint16(len(validAttrs))
		elemStartSize := uint32(36 + int(attrCount)*20)

		// START_ELEMENT
		writeChunkHeader(&treeBuf, ResXMLStartElement, 0x10, elemStartSize)
		_ = binary.Write(&treeBuf, binary.LittleEndian, line)
		_ = binary.Write(&treeBuf, binary.LittleEndian, uint32(0xFFFFFFFF))
		_ = binary.Write(&treeBuf, binary.LittleEndian, elemUriIdx)
		_ = binary.Write(&treeBuf, binary.LittleEndian, elemNameIdx)
		_ = binary.Write(&treeBuf, binary.LittleEndian, uint16(0x0014)) // attr_start
		_ = binary.Write(&treeBuf, binary.LittleEndian, uint16(0x0014)) // attr_size
		_ = binary.Write(&treeBuf, binary.LittleEndian, attrCount)
		_ = binary.Write(&treeBuf, binary.LittleEndian, uint16(0)) // id_index
		_ = binary.Write(&treeBuf, binary.LittleEndian, uint16(0)) // class_index
		_ = binary.Write(&treeBuf, binary.LittleEndian, uint16(0)) // style_index

		// Write attributes (20 bytes each)
		for _, attr := range validAttrs {
			localName := strings.TrimPrefix(attr.Name.Local, "android:")
			attrNameIdx := sp.index[localName]
			valIdx := sp.index[attr.Value]

			attrUriIdx := uint32(0xFFFFFFFF)
			if attr.Name.Space != "" || strings.HasPrefix(attr.Name.Local, "android:") {
				attrUriIdx = uriIdx
			}

			// Determine typed value
			var dataType uint8 = TypeString
			var dataVal uint32 = valIdx

			if themeRes, ok := knownThemeResIDs[attr.Value]; ok {
				dataType = TypeReference
				dataVal = themeRes
			} else if attr.Value == "true" {
				dataType = TypeIntBool
				dataVal = 0xFFFFFFFF
			} else if attr.Value == "false" {
				dataType = TypeIntBool
				dataVal = 0
			} else if num, err := strconv.ParseInt(attr.Value, 0, 32); err == nil && (localName == "versionCode" || localName == "minSdkVersion" || localName == "targetSdkVersion") {
				dataType = TypeIntDec
				dataVal = uint32(num)
			}

			_ = binary.Write(&treeBuf, binary.LittleEndian, attrUriIdx)
			_ = binary.Write(&treeBuf, binary.LittleEndian, attrNameIdx)
			_ = binary.Write(&treeBuf, binary.LittleEndian, valIdx)
			_ = binary.Write(&treeBuf, binary.LittleEndian, uint16(8)) // size
			_ = binary.Write(&treeBuf, binary.LittleEndian, uint8(0))  // res0
			_ = binary.Write(&treeBuf, binary.LittleEndian, dataType)
			_ = binary.Write(&treeBuf, binary.LittleEndian, dataVal)
		}

		// Children
		for _, child := range elem.Children {
			writeElem(child)
		}

		// END_ELEMENT
		line++
		writeChunkHeader(&treeBuf, ResXMLEndElement, 0x10, 0x18)
		_ = binary.Write(&treeBuf, binary.LittleEndian, line)
		_ = binary.Write(&treeBuf, binary.LittleEndian, uint32(0xFFFFFFFF))
		_ = binary.Write(&treeBuf, binary.LittleEndian, elemUriIdx)
		_ = binary.Write(&treeBuf, binary.LittleEndian, elemNameIdx)
	}

	writeElem(root)

	// End Namespace
	line++
	writeChunkHeader(&treeBuf, ResXMLEndNamespace, 0x10, 0x18)
	_ = binary.Write(&treeBuf, binary.LittleEndian, line)
	_ = binary.Write(&treeBuf, binary.LittleEndian, uint32(0xFFFFFFFF))
	_ = binary.Write(&treeBuf, binary.LittleEndian, prefixIdx)
	_ = binary.Write(&treeBuf, binary.LittleEndian, uriIdx)

	// Combine all into final AXML
	var finalBuf bytes.Buffer
	totalSize := uint32(8 + spBuf.Len() + resMapBuf.Len() + treeBuf.Len())

	// File Header
	writeChunkHeader(&finalBuf, ResXMLType, 8, totalSize)
	finalBuf.Write(spBuf.Bytes())
	finalBuf.Write(resMapBuf.Bytes())
	finalBuf.Write(treeBuf.Bytes())

	return finalBuf.Bytes(), nil
}

func writeChunkHeader(buf *bytes.Buffer, chunkType uint16, headerSize uint16, chunkSize uint32) {
	_ = binary.Write(buf, binary.LittleEndian, chunkType)
	_ = binary.Write(buf, binary.LittleEndian, headerSize)
	_ = binary.Write(buf, binary.LittleEndian, chunkSize)
}
