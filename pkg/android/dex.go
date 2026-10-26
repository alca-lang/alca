// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash/adler32"
	"sort"
	"strings"
)

// Dalvik instruction opcodes.
const (
	OpMoveResultObject = 0x0c
	OpReturnVoid       = 0x0e
	OpConst4           = 0x12
	OpConst16          = 0x13
	OpConstString      = 0x1a
	OpNewInstance      = 0x22
	OpNewArray         = 0x23
	OpAputObject       = 0x4d
	OpInvokeVirtual    = 0x6e
	OpInvokeSuper      = 0x6f
	OpInvokeDirect     = 0x70

	AccPublic      = 0x0001
	AccConstructor = 0x10000
)

// DEX map item types.
const (
	TypeHeaderItem   = 0x0000
	TypeStringIdItem = 0x0001
	TypeIdItem       = 0x0002
	TypeProtoIdItem  = 0x0003
	TypeMethodIdItem = 0x0005
	TypeClassDefItem = 0x0006
	TypeMapList      = 0x1000
	TypeTypeList     = 0x1001
	TypeClassData    = 0x2000
	TypeCodeItem     = 0x2001
	TypeStringData   = 0x2002
)

// encodeInvoke35c encodes a format 35c Dalvik instruction: [A|G|op] BBBB [F|E|D|C]
func encodeInvoke35c(op byte, methodIdx uint16, args ...uint8) []uint16 {
	count := len(args)
	var regC, regD, regE, regF, regG uint8
	if count > 0 {
		regC = args[0]
	}
	if count > 1 {
		regD = args[1]
	}
	if count > 2 {
		regE = args[2]
	}
	if count > 3 {
		regF = args[3]
	}
	if count > 4 {
		regG = args[4]
	}

	w0 := uint16(op) | (uint16(regG&0xF) << 8) | (uint16(count&0xF) << 12)
	w1 := methodIdx
	w2 := uint16(regC&0xF) | (uint16(regD&0xF) << 4) | (uint16(regE&0xF) << 8) | (uint16(regF&0xF) << 12)
	return []uint16{w0, w1, w2}
}

// normalizePermission ensures the permission has the "android.permission." prefix.
func normalizePermission(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.Contains(p, ".") {
		return "android.permission." + p
	}
	return p
}

// isNormalInstallTimePermission returns true for permissions granted at install time
// that do not require runtime Activity.requestPermissions() popups.
func isNormalInstallTimePermission(perm string) bool {
	perm = normalizePermission(perm)
	switch perm {
	case "android.permission.INTERNET",
		"android.permission.ACCESS_NETWORK_STATE",
		"android.permission.ACCESS_WIFI_STATE",
		"android.permission.CHANGE_NETWORK_STATE",
		"android.permission.CHANGE_WIFI_STATE",
		"android.permission.VIBRATE",
		"android.permission.WAKE_LOCK",
		"android.permission.MODIFY_AUDIO_SETTINGS",
		"android.permission.NFC",
		"android.permission.BLUETOOTH",
		"android.permission.BLUETOOTH_ADMIN",
		"android.permission.FOREGROUND_SERVICE",
		"android.permission.RECEIVE_BOOT_COMPLETED",
		"android.permission.USE_BIOMETRIC",
		"android.permission.USE_FINGERPRINT",
		"android.permission.FLASHLIGHT",
		"android.permission.SET_WALLPAPER",
		"android.permission.EXPAND_STATUS_BAR",
		"android.permission.KILL_BACKGROUND_PROCESSES",
		"android.permission.REORDER_TASKS",
		"android.permission.SYSTEM_ALERT_WINDOW",
		"android.permission.WRITE_SETTINGS",
		"android.permission.MANAGE_EXTERNAL_STORAGE":
		return true
	default:
		return false
	}
}

// BuildLauncherDEX builds a valid, spec-compliant classes.dex Dalvik executable for the Alca Activity.
func BuildLauncherDEX(activityClassName string, requestedPermissions ...string) ([]byte, error) {
	if activityClassName == "" {
		activityClassName = "alca.runtime.MainActivity"
	}
	// Convert "alca.runtime.MainActivity" -> "Lalca/runtime/MainActivity;"
	activityDescriptor := "L" + strings.ReplaceAll(activityClassName, ".", "/") + ";"

	var permStrings []string
	seenPerms := make(map[string]bool)
	for _, p := range requestedPermissions {
		p = normalizePermission(p)
		if p != "" && !isNormalInstallTimePermission(p) && !seenPerms[p] {
			seenPerms[p] = true
			permStrings = append(permStrings, p)
		}
	}

	// 1. Strings List (must contain all strings referenced by types, methods, protos, code, and source)
	rawStrings := []string{
		"<init>",
		activityDescriptor,
		"Landroid/app/Activity;",
		"Landroid/content/Context;",
		"Landroid/os/Bundle;",
		"Landroid/view/View;",
		"Landroid/webkit/WebSettings;",
		"Landroid/webkit/WebView;",
		"Landroid/webkit/WebChromeClient;",
		"Landroid/webkit/WebViewClient;",
		"Ljava/lang/String;",
		"[Ljava/lang/String;",
		"MainActivity.al",
		"V",
		"Z",
		"I",
		"L",
		"VL",
		"VZ",
		"ZI",
		"VLI",
		"file:///android_asset/index.html",
		"getSettings",
		"loadUrl",
		"onCreate",
		"requestPermissions",
		"requestWindowFeature",
		"setAllowContentAccess",
		"setAllowFileAccess",
		"setAllowFileAccessFromFileURLs",
		"setAllowUniversalAccessFromFileURLs",
		"setContentView",
		"setDatabaseEnabled",
		"setDomStorageEnabled",
		"setGeolocationEnabled",
		"setJavaScriptEnabled",
		"setMediaPlaybackRequiresUserGesture",
		"setWebChromeClient",
		"setWebViewClient",
	}

	for _, p := range permStrings {
		rawStrings = append(rawStrings, p)
	}

	// Remove duplicates and sort lexicographically (required by DEX specification)
	stringSet := make(map[string]bool)
	var stringsList []string
	for _, s := range rawStrings {
		if !stringSet[s] {
			stringSet[s] = true
			stringsList = append(stringsList, s)
		}
	}
	sort.Strings(stringsList)

	stringMap := make(map[string]uint32)
	for i, s := range stringsList {
		stringMap[s] = uint32(i)
	}

	// 2. Types List
	rawTypes := []string{
		activityDescriptor,
		"Landroid/app/Activity;",
		"Landroid/content/Context;",
		"Landroid/os/Bundle;",
		"Landroid/view/View;",
		"Landroid/webkit/WebSettings;",
		"Landroid/webkit/WebView;",
		"Landroid/webkit/WebChromeClient;",
		"Landroid/webkit/WebViewClient;",
		"Ljava/lang/String;",
		"[Ljava/lang/String;",
		"V",
		"Z",
		"I",
	}

	typeSet := make(map[string]bool)
	var typesList []string
	for _, t := range rawTypes {
		if !typeSet[t] {
			typeSet[t] = true
			typesList = append(typesList, t)
		}
	}

	// Type IDs must be sorted by descriptor string_id index
	sort.Slice(typesList, func(i, j int) bool {
		return stringMap[typesList[i]] < stringMap[typesList[j]]
	})

	typeMap := make(map[string]uint32)
	for i, t := range typesList {
		typeMap[t] = uint32(i)
	}

	// 3. Proto definitions
	type protoDef struct {
		shorty     string
		returnType string
		params     []string
	}

	rawProtos := []protoDef{
		// ()V
		{"V", "V", nil},
		// ()Landroid/webkit/WebSettings; (returns object, 0 params -> shorty "L")
		{"L", "Landroid/webkit/WebSettings;", nil},
		// (Bundle)V
		{"VL", "V", []string{"Landroid/os/Bundle;"}},
		// (Context)V
		{"VL", "V", []string{"Landroid/content/Context;"}},
		// (View)V
		{"VL", "V", []string{"Landroid/view/View;"}},
		// (String)V
		{"VL", "V", []string{"Ljava/lang/String;"}},
		// (WebChromeClient)V
		{"VL", "V", []string{"Landroid/webkit/WebChromeClient;"}},
		// (WebViewClient)V
		{"VL", "V", []string{"Landroid/webkit/WebViewClient;"}},
		// (boolean)V
		{"VZ", "V", []string{"Z"}},
		// (int)Z
		{"ZI", "Z", []string{"I"}},
		// (String[], int)V
		{"VLI", "V", []string{"[Ljava/lang/String;", "I"}},
	}

	// Sort protos by return_type_idx, then parameter types (DEX spec requirement)
	protos := append([]protoDef(nil), rawProtos...)
	sort.Slice(protos, func(i, j int) bool {
		ri, rj := typeMap[protos[i].returnType], typeMap[protos[j].returnType]
		if ri != rj {
			return ri < rj
		}
		// Compare parameter lists
		p1, p2 := protos[i].params, protos[j].params
		minLen := len(p1)
		if len(p2) < minLen {
			minLen = len(p2)
		}
		for k := 0; k < minLen; k++ {
			t1, t2 := typeMap[p1[k]], typeMap[p2[k]]
			if t1 != t2 {
				return t1 < t2
			}
		}
		return len(p1) < len(p2)
	})

	findProtoIdx := func(ret string, params []string) uint16 {
		for i, p := range protos {
			if p.returnType == ret && sliceEqualStrings(p.params, params) {
				return uint16(i)
			}
		}
		panic(fmt.Sprintf("proto not found: ret=%s, params=%v", ret, params))
	}

	// 4. Method definitions
	type methodDef struct {
		class      string
		name       string
		returnType string
		params     []string
		protoIdx   uint16
	}

	rawMethods := []methodDef{
		{activityDescriptor, "<init>", "V", nil, 0},
		{activityDescriptor, "onCreate", "V", []string{"Landroid/os/Bundle;"}, 0},
		{"Landroid/app/Activity;", "<init>", "V", nil, 0},
		{"Landroid/app/Activity;", "onCreate", "V", []string{"Landroid/os/Bundle;"}, 0},
		{"Landroid/app/Activity;", "requestPermissions", "V", []string{"[Ljava/lang/String;", "I"}, 0},
		{"Landroid/app/Activity;", "requestWindowFeature", "Z", []string{"I"}, 0},
		{"Landroid/app/Activity;", "setContentView", "V", []string{"Landroid/view/View;"}, 0},
		{"Landroid/webkit/WebView;", "<init>", "V", []string{"Landroid/content/Context;"}, 0},
		{"Landroid/webkit/WebView;", "getSettings", "Landroid/webkit/WebSettings;", nil, 0},
		{"Landroid/webkit/WebView;", "loadUrl", "V", []string{"Ljava/lang/String;"}, 0},
		{"Landroid/webkit/WebView;", "setWebChromeClient", "V", []string{"Landroid/webkit/WebChromeClient;"}, 0},
		{"Landroid/webkit/WebView;", "setWebViewClient", "V", []string{"Landroid/webkit/WebViewClient;"}, 0},
		{"Landroid/webkit/WebChromeClient;", "<init>", "V", nil, 0},
		{"Landroid/webkit/WebViewClient;", "<init>", "V", nil, 0},
		{"Landroid/webkit/WebSettings;", "setAllowContentAccess", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setAllowFileAccess", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setAllowFileAccessFromFileURLs", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setAllowUniversalAccessFromFileURLs", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setDatabaseEnabled", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setDomStorageEnabled", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setGeolocationEnabled", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setJavaScriptEnabled", "V", []string{"Z"}, 0},
		{"Landroid/webkit/WebSettings;", "setMediaPlaybackRequiresUserGesture", "V", []string{"Z"}, 0},
	}

	for i := range rawMethods {
		rawMethods[i].protoIdx = findProtoIdx(rawMethods[i].returnType, rawMethods[i].params)
	}

	// Sort methods by class_idx, then name_idx, then proto_idx (DEX spec requirement)
	methods := append([]methodDef(nil), rawMethods...)
	sort.Slice(methods, func(i, j int) bool {
		ci, cj := typeMap[methods[i].class], typeMap[methods[j].class]
		if ci != cj {
			return ci < cj
		}
		ni, nj := stringMap[methods[i].name], stringMap[methods[j].name]
		if ni != nj {
			return ni < nj
		}
		return methods[i].protoIdx < methods[j].protoIdx
	})

	findMethodIdx := func(class, name string) uint16 {
		for i, m := range methods {
			if m.class == class && m.name == name {
				return uint16(i)
			}
		}
		panic(fmt.Sprintf("method not found: %s->%s", class, name))
	}

	// Method indices
	mInitSuper := findMethodIdx("Landroid/app/Activity;", "<init>")
	mOnCreateSuper := findMethodIdx("Landroid/app/Activity;", "onCreate")
	mRequestPermissions := findMethodIdx("Landroid/app/Activity;", "requestPermissions")
	mRequestWindowFeature := findMethodIdx("Landroid/app/Activity;", "requestWindowFeature")
	mSetContentView := findMethodIdx("Landroid/app/Activity;", "setContentView")
	mWebViewInit := findMethodIdx("Landroid/webkit/WebView;", "<init>")
	mWebViewGetSettings := findMethodIdx("Landroid/webkit/WebView;", "getSettings")
	mWebViewLoadUrl := findMethodIdx("Landroid/webkit/WebView;", "loadUrl")
	mSetWebChromeClient := findMethodIdx("Landroid/webkit/WebView;", "setWebChromeClient")
	mSetWebViewClient := findMethodIdx("Landroid/webkit/WebView;", "setWebViewClient")
	mWebChromeClientInit := findMethodIdx("Landroid/webkit/WebChromeClient;", "<init>")
	mWebViewClientInit := findMethodIdx("Landroid/webkit/WebViewClient;", "<init>")

	mSetJSEnabled := findMethodIdx("Landroid/webkit/WebSettings;", "setJavaScriptEnabled")
	mSetDomStorage := findMethodIdx("Landroid/webkit/WebSettings;", "setDomStorageEnabled")
	mSetDatabaseEnabled := findMethodIdx("Landroid/webkit/WebSettings;", "setDatabaseEnabled")
	mSetAllowFileAccess := findMethodIdx("Landroid/webkit/WebSettings;", "setAllowFileAccess")
	mSetAllowContentAccess := findMethodIdx("Landroid/webkit/WebSettings;", "setAllowContentAccess")
	mSetAllowFileAccessFromFileURLs := findMethodIdx("Landroid/webkit/WebSettings;", "setAllowFileAccessFromFileURLs")
	mSetAllowUniversalAccessFromFileURLs := findMethodIdx("Landroid/webkit/WebSettings;", "setAllowUniversalAccessFromFileURLs")
	mSetGeolocationEnabled := findMethodIdx("Landroid/webkit/WebSettings;", "setGeolocationEnabled")
	mSetMediaPlaybackRequiresUserGesture := findMethodIdx("Landroid/webkit/WebSettings;", "setMediaPlaybackRequiresUserGesture")

	// Type & String indices
	tWebView := uint16(typeMap["Landroid/webkit/WebView;"])
	tWebChromeClient := uint16(typeMap["Landroid/webkit/WebChromeClient;"])
	tWebViewClient := uint16(typeMap["Landroid/webkit/WebViewClient;"])
	tStringArray := uint16(typeMap["[Ljava/lang/String;"])
	sUrl := uint16(stringMap["file:///android_asset/index.html"])

	// 5. Dalvik Bytecode Construction
	//
	// Method 1: <init>()V
	// registers=1, ins=1, outs=1
	// Incoming 'this' is in v0.
	// invoke-direct {v0}, Landroid/app/Activity;-><init>()V
	// return-void
	var initCode []uint16
	initCode = append(initCode, encodeInvoke35c(OpInvokeDirect, mInitSuper, 0)...)
	initCode = append(initCode, OpReturnVoid)

	// Method 2: onCreate(Landroid/os/Bundle;)V
	// registers=6, ins=2, outs=outsSize
	// In Dalvik, incoming parameters are placed in the last `ins` registers:
	// v4 = 'this' (Activity)
	// v5 = 'savedInstanceState' (Bundle)
	// Locals:
	// v0 = webView / permissions array
	// v1 = webSettings / client / string param
	// v2 = boolean / index / requestCode literal
	// v3 = unused
	var onCreateCode []uint16

	// 1. invoke-super {v4, v5}, Activity->onCreate(Bundle)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeSuper, mOnCreateSuper, 4, 5)...)

	// 1b. requestWindowFeature(Window.FEATURE_NO_TITLE = 1)
	// const/4 v2, 1
	onCreateCode = append(onCreateCode, (1<<12)|(2<<8)|uint16(OpConst4))
	// invoke-virtual {v4, v2}, Activity->requestWindowFeature(I)Z
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mRequestWindowFeature, 4, 2)...)

	// 1c. requestPermissions(dangerous permissions array, 1) if any are requested
	if len(permStrings) > 0 {
		// const/16 v2, len(permStrings)
		onCreateCode = append(onCreateCode, (2<<8)|uint16(OpConst16), uint16(len(permStrings)))
		// new-array v0, v2, [Ljava/lang/String;
		onCreateCode = append(onCreateCode, uint16(OpNewArray)|(0<<8)|(2<<12), tStringArray)

		for idx, p := range permStrings {
			sIdx := uint16(stringMap[p])
			// const-string v1, perm
			onCreateCode = append(onCreateCode, (1<<8)|uint16(OpConstString), sIdx)
			// const/16 v2, idx
			onCreateCode = append(onCreateCode, (2<<8)|uint16(OpConst16), uint16(idx))
			// aput-object v1, v0, v2
			onCreateCode = append(onCreateCode, uint16(OpAputObject)|(1<<8), uint16(0)|(uint16(2)<<8))
		}

		// const/4 v2, 1 (requestCode = 1)
		onCreateCode = append(onCreateCode, (1<<12)|(2<<8)|uint16(OpConst4))
		// invoke-virtual {v4, v0, v2}, Activity->requestPermissions([Ljava/lang/String;I)V
		onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mRequestPermissions, 4, 0, 2)...)
	}

	// 2. new-instance v0, Landroid/webkit/WebView;
	onCreateCode = append(onCreateCode, (0<<8)|uint16(OpNewInstance), tWebView)

	// 3. invoke-direct {v0, v4}, WebView-><init>(Context)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeDirect, mWebViewInit, 0, 4)...)

	// 4. Set WebChromeClient (enables alert, confirm, file chooser, camera, geolocation)
	onCreateCode = append(onCreateCode, (1<<8)|uint16(OpNewInstance), tWebChromeClient)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeDirect, mWebChromeClientInit, 1)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetWebChromeClient, 0, 1)...)

	// 5. Set WebViewClient (keeps links in app)
	onCreateCode = append(onCreateCode, (1<<8)|uint16(OpNewInstance), tWebViewClient)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeDirect, mWebViewClientInit, 1)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetWebViewClient, 0, 1)...)

	// 6. invoke-virtual {v0}, WebView->getSettings()WebSettings;
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mWebViewGetSettings, 0)...)

	// 7. move-result-object v1
	onCreateCode = append(onCreateCode, (1<<8)|uint16(OpMoveResultObject))

	// 8. const/4 v2, 1 (true)
	onCreateCode = append(onCreateCode, (1<<12)|(2<<8)|uint16(OpConst4))

	// WebSettings configuration (Full Android HTML5 & Device Support)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetJSEnabled, 1, 2)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetDomStorage, 1, 2)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetDatabaseEnabled, 1, 2)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetAllowFileAccess, 1, 2)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetAllowContentAccess, 1, 2)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetAllowFileAccessFromFileURLs, 1, 2)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetAllowUniversalAccessFromFileURLs, 1, 2)...)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetGeolocationEnabled, 1, 2)...)

	// const/4 v2, 0 (false for user gesture requirement on media playback)
	onCreateCode = append(onCreateCode, (0<<12)|(2<<8)|uint16(OpConst4))
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetMediaPlaybackRequiresUserGesture, 1, 2)...)

	// 9. const-string v1, "file:///android_asset/index.html"
	onCreateCode = append(onCreateCode, (1<<8)|uint16(OpConstString), sUrl)

	// 10. invoke-virtual {v0, v1}, WebView->loadUrl(String)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mWebViewLoadUrl, 0, 1)...)

	// 11. invoke-virtual {v4, v0}, Activity->setContentView(View)
	onCreateCode = append(onCreateCode, encodeInvoke35c(OpInvokeVirtual, mSetContentView, 4, 0)...)

	// 12. return-void
	onCreateCode = append(onCreateCode, OpReturnVoid)

	// 6. Layout and Offsets
	headerSize := uint32(0x70)

	stringIdsOff := headerSize
	stringIdsSize := uint32(len(stringsList))

	typeIdsOff := stringIdsOff + stringIdsSize*4
	typeIdsSize := uint32(len(typesList))

	protoIdsOff := typeIdsOff + typeIdsSize*4
	protoIdsSize := uint32(len(protos))

	methodIdsOff := protoIdsOff + protoIdsSize*12
	methodIdsSize := uint32(len(methods))

	classDefsOff := methodIdsOff + methodIdsSize*8
	classDefsSize := uint32(1)

	dataStart := classDefsOff + classDefsSize*32

	// Dynamic data section assembly
	var dataBuf bytes.Buffer

	// A. Write parameter type lists
	protoParamsOff := make([]uint32, len(protos))
	firstTypeListOff := uint32(0)
	typeListCount := uint32(0)
	for i, p := range protos {
		if len(p.params) == 0 {
			protoParamsOff[i] = 0
			continue
		}
		align4(&dataBuf)
		off := dataStart + uint32(dataBuf.Len())
		if firstTypeListOff == 0 {
			firstTypeListOff = off
		}
		typeListCount++
		protoParamsOff[i] = off
		_ = binary.Write(&dataBuf, binary.LittleEndian, uint32(len(p.params)))
		for _, paramType := range p.params {
			_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(typeMap[paramType]))
		}
		// 4-byte padding if odd count
		if len(p.params)%2 != 0 {
			_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(0))
		}
	}

	// B. Write String data
	stringDataOffs := make([]uint32, len(stringsList))
	for i, s := range stringsList {
		stringDataOffs[i] = dataStart + uint32(dataBuf.Len())
		writeUleb128(&dataBuf, uint32(len(s)))
		dataBuf.WriteString(s)
		dataBuf.WriteByte(0)
	}

	// C. Write Code items (must be 4-byte aligned)
	align4(&dataBuf)
	initCodeOff := dataStart + uint32(dataBuf.Len())
	// <init>: registers=1, ins=1, outs=1, tries=0, debug=0
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(1)) // registers_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(1)) // ins_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(1)) // outs_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(0)) // tries_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint32(0)) // debug_info_off
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint32(len(initCode)))
	for _, inst := range initCode {
		_ = binary.Write(&dataBuf, binary.LittleEndian, inst)
	}

	align4(&dataBuf)
	onCreateCodeOff := dataStart + uint32(dataBuf.Len())
	outsSize := uint16(2)
	if len(permStrings) > 0 {
		outsSize = 3
	}
	// onCreate: registers=6, ins=2, outs=outsSize, tries=0, debug=0
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(6)) // registers_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(2)) // ins_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, outsSize)  // outs_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(0)) // tries_size
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint32(0)) // debug_info_off
	_ = binary.Write(&dataBuf, binary.LittleEndian, uint32(len(onCreateCode)))
	for _, inst := range onCreateCode {
		_ = binary.Write(&dataBuf, binary.LittleEndian, inst)
	}

	// D. Write Class data
	align4(&dataBuf)
	classDataOff := dataStart + uint32(dataBuf.Len())

	// static_fields=0, instance_fields=0, direct_methods=1 (<init>), virtual_methods=1 (onCreate)
	writeUleb128(&dataBuf, 0)
	writeUleb128(&dataBuf, 0)
	writeUleb128(&dataBuf, 1)
	writeUleb128(&dataBuf, 1)

	// direct method 0: <init>()V
	mInitIdx := uint32(findMethodIdx(activityDescriptor, "<init>"))
	writeUleb128(&dataBuf, mInitIdx)
	writeUleb128(&dataBuf, AccPublic|AccConstructor)
	writeUleb128(&dataBuf, initCodeOff)

	// virtual method 0: onCreate(Bundle)V
	mOnCreateIdx := uint32(findMethodIdx(activityDescriptor, "onCreate"))
	writeUleb128(&dataBuf, mOnCreateIdx)
	writeUleb128(&dataBuf, AccPublic)
	writeUleb128(&dataBuf, onCreateCodeOff)

	// E. Map list
	align4(&dataBuf)
	mapOff := dataStart + uint32(dataBuf.Len())

	type mapItem struct {
		typ  uint16
		size uint32
		off  uint32
	}

	mapItems := []mapItem{
		{TypeHeaderItem, 1, 0},
		{TypeStringIdItem, stringIdsSize, stringIdsOff},
		{TypeIdItem, typeIdsSize, typeIdsOff},
		{TypeProtoIdItem, protoIdsSize, protoIdsOff},
		{TypeMethodIdItem, methodIdsSize, methodIdsOff},
		{TypeClassDefItem, classDefsSize, classDefsOff},
		{TypeStringData, stringIdsSize, stringDataOffs[0]},
		{TypeCodeItem, 2, initCodeOff},
		{TypeClassData, 1, classDataOff},
		{TypeMapList, 1, mapOff},
	}

	if typeListCount > 0 {
		mapItems = append(mapItems, mapItem{TypeTypeList, typeListCount, firstTypeListOff})
	}

	// Map items must be sorted by offset ascending (DEX specification)
	sort.Slice(mapItems, func(i, j int) bool {
		return mapItems[i].off < mapItems[j].off
	})

	_ = binary.Write(&dataBuf, binary.LittleEndian, uint32(len(mapItems)))
	for _, item := range mapItems {
		_ = binary.Write(&dataBuf, binary.LittleEndian, item.typ)
		_ = binary.Write(&dataBuf, binary.LittleEndian, uint16(0)) // unused
		_ = binary.Write(&dataBuf, binary.LittleEndian, item.size)
		_ = binary.Write(&dataBuf, binary.LittleEndian, item.off)
	}

	totalDataSize := uint32(dataBuf.Len())
	totalFileSize := dataStart + totalDataSize

	// 7. Assemble Binary DEX
	var dexBuf bytes.Buffer

	// Header (112 bytes)
	dexBuf.WriteString("dex\n035\x00")                                 // magic (8 bytes)
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0))          // placeholder checksum (4 bytes)
	dexBuf.Write(make([]byte, 20))                                     // placeholder SHA-1 signature (20 bytes)
	_ = binary.Write(&dexBuf, binary.LittleEndian, totalFileSize)      // file size
	_ = binary.Write(&dexBuf, binary.LittleEndian, headerSize)         // header size (112)
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0x12345678)) // endian tag
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0))          // link size
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0))          // link off
	_ = binary.Write(&dexBuf, binary.LittleEndian, mapOff)             // map off
	_ = binary.Write(&dexBuf, binary.LittleEndian, stringIdsSize)      // string ids size
	_ = binary.Write(&dexBuf, binary.LittleEndian, stringIdsOff)       // string ids off
	_ = binary.Write(&dexBuf, binary.LittleEndian, typeIdsSize)        // type ids size
	_ = binary.Write(&dexBuf, binary.LittleEndian, typeIdsOff)         // type ids off
	_ = binary.Write(&dexBuf, binary.LittleEndian, protoIdsSize)       // proto ids size
	_ = binary.Write(&dexBuf, binary.LittleEndian, protoIdsOff)        // proto ids off
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0))          // field ids size
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0))          // field ids off
	_ = binary.Write(&dexBuf, binary.LittleEndian, methodIdsSize)      // method ids size
	_ = binary.Write(&dexBuf, binary.LittleEndian, methodIdsOff)       // method ids off
	_ = binary.Write(&dexBuf, binary.LittleEndian, classDefsSize)      // class defs size
	_ = binary.Write(&dexBuf, binary.LittleEndian, classDefsOff)       // class defs off
	_ = binary.Write(&dexBuf, binary.LittleEndian, totalDataSize)      // data size
	_ = binary.Write(&dexBuf, binary.LittleEndian, dataStart)          // data off

	// String IDs (uint32 offset to string_data)
	for _, off := range stringDataOffs {
		_ = binary.Write(&dexBuf, binary.LittleEndian, off)
	}

	// Type IDs (uint32 descriptor_idx into string_ids)
	for _, t := range typesList {
		_ = binary.Write(&dexBuf, binary.LittleEndian, stringMap[t])
	}

	// Proto IDs (shorty_idx uint32, return_type_idx uint32, parameters_off uint32)
	for i, p := range protos {
		_ = binary.Write(&dexBuf, binary.LittleEndian, stringMap[p.shorty])
		_ = binary.Write(&dexBuf, binary.LittleEndian, typeMap[p.returnType])
		_ = binary.Write(&dexBuf, binary.LittleEndian, protoParamsOff[i])
	}

	// Method IDs (class_idx uint16, proto_idx uint16, name_idx uint32)
	for _, m := range methods {
		_ = binary.Write(&dexBuf, binary.LittleEndian, uint16(typeMap[m.class]))
		_ = binary.Write(&dexBuf, binary.LittleEndian, m.protoIdx)
		_ = binary.Write(&dexBuf, binary.LittleEndian, stringMap[m.name])
	}

	// Class Defs (32 bytes per class)
	// class_idx (uint32), access_flags (uint32), superclass_idx (uint32), interfaces_off (uint32),
	// source_file_idx (uint32), annotations_off (uint32), class_data_off (uint32), static_values_off (uint32)
	_ = binary.Write(&dexBuf, binary.LittleEndian, typeMap[activityDescriptor])
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(AccPublic))
	_ = binary.Write(&dexBuf, binary.LittleEndian, typeMap["Landroid/app/Activity;"])
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0)) // interfaces_off (none)
	_ = binary.Write(&dexBuf, binary.LittleEndian, stringMap["MainActivity.al"])
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0)) // annotations_off (none)
	_ = binary.Write(&dexBuf, binary.LittleEndian, classDataOff)
	_ = binary.Write(&dexBuf, binary.LittleEndian, uint32(0)) // static_values_off (none)

	// Data Section
	dexBuf.Write(dataBuf.Bytes())

	res := dexBuf.Bytes()
	if uint32(len(res)) != totalFileSize {
		return nil, fmt.Errorf("DEX file size mismatch: expected %d, got %d", totalFileSize, len(res))
	}

	// Calculate and write SHA-1 signature (bytes 32..end into bytes 12..31)
	sig := sha1.Sum(res[32:])
	copy(res[12:32], sig[:])

	// Calculate and write Adler32 checksum (bytes 12..end into bytes 8..11)
	chk := adler32.Checksum(res[12:])
	binary.LittleEndian.PutUint32(res[8:12], chk)

	return res, nil
}

func align4(buf *bytes.Buffer) {
	for buf.Len()%4 != 0 {
		buf.WriteByte(0)
	}
}

func writeUleb128(buf *bytes.Buffer, val uint32) {
	for {
		b := byte(val & 0x7F)
		val >>= 7
		if val != 0 {
			buf.WriteByte(b | 0x80)
		} else {
			buf.WriteByte(b)
			break
		}
	}
}

func sliceEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
