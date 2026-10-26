// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alca-lang/alca/pkg/lexer"
	"github.com/alca-lang/alca/pkg/parser"
	"github.com/alca-lang/alca/pkg/typecheck"
)

var (
	usesPermissionRegex = regexp.MustCompile(`(?i)<uses-permission\b[^>]*android:name=["']([^"']+)["'][^>]*>`)
	usesPermissionTag   = regexp.MustCompile(`(?i)<uses-permission\b[^>]*>(?:</uses-permission>)?`)
)

func extractManifestPermissions(manifestXML []byte) []string {
	matches := usesPermissionRegex.FindAllSubmatch(manifestXML, -1)
	var result []string
	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			perm := normalizePermission(string(m[1]))
			if perm != "" && !seen[perm] {
				seen[perm] = true
				result = append(result, perm)
			}
		}
	}
	return result
}

func syncManifestPermissions(manifestXML []byte, permissions []string) []byte {
	// Remove all existing uses-permission tags and their lines
	lineRegex := regexp.MustCompile(`(?m)^\s*<uses-permission\b[^>]*>(?:</uses-permission>)?\s*\r?\n`)
	cleaned := lineRegex.ReplaceAll(manifestXML, nil)
	cleaned = usesPermissionTag.ReplaceAll(cleaned, nil)

	cleanStr := string(cleaned)
	for strings.Contains(cleanStr, "\n\n\n") {
		cleanStr = strings.ReplaceAll(cleanStr, "\n\n\n", "\n\n")
	}

	var permBlock strings.Builder
	seen := make(map[string]bool)
	for _, p := range permissions {
		norm := normalizePermission(p)
		if norm != "" && !seen[norm] {
			seen[norm] = true
			permBlock.WriteString(fmt.Sprintf("    <uses-permission android:name=\"%s\" />\n", norm))
		}
	}

	appIdx := strings.Index(cleanStr, "<application")
	if appIdx != -1 {
		lineStart := strings.LastIndex(cleanStr[:appIdx], "\n")
		if lineStart == -1 {
			lineStart = 0
		} else {
			lineStart++
		}
		return []byte(cleanStr[:lineStart] + permBlock.String() + cleanStr[lineStart:])
	}
	endIdx := strings.Index(cleanStr, "</manifest>")
	if endIdx != -1 {
		return []byte(cleanStr[:endIdx] + permBlock.String() + cleanStr[endIdx:])
	}
	return []byte(cleanStr + "\n" + permBlock.String())
}

// BuildOptions holds options for building an Android APK.
type BuildOptions struct {
	ProjectDir string
	OutputPath string
	Debug      bool
}

// BuildAPK compiles an Alca Android project into a 4-byte zipaligned, v1 + v2 dual-signed Android APK.
func BuildAPK(opts BuildOptions) (string, error) {
	projDir := opts.ProjectDir
	if projDir == "" {
		projDir = "."
	}
	absProjDir, err := filepath.Abs(projDir)
	if err != nil {
		return "", fmt.Errorf("resolving project directory: %w", err)
	}

	manifestFile := filepath.Join(absProjDir, "AndroidManifest.xml")
	manifestData, err := os.ReadFile(manifestFile)
	if err != nil {
		return "", fmt.Errorf("missing AndroidManifest.xml in %s: %w", absProjDir, err)
	}

	cfg, err := LoadConfig(absProjDir)
	if err != nil {
		return "", fmt.Errorf("loading project config: %w", err)
	}

	// Determine authoritative permissions
	var finalPermissions []string
	if cfg.PermissionsExplicit {
		finalPermissions = cfg.Permissions
		// Synchronize AndroidManifest.xml with configured permissions
		manifestData = syncManifestPermissions(manifestData, finalPermissions)
		_ = os.WriteFile(manifestFile, manifestData, 0644)
	} else {
		// Fallback: extract permissions from existing AndroidManifest.xml
		finalPermissions = extractManifestPermissions(manifestData)
		if len(finalPermissions) == 0 {
			finalPermissions = []string{
				"android.permission.INTERNET",
				"android.permission.ACCESS_NETWORK_STATE",
				"android.permission.VIBRATE",
			}
		}
	}

	// 1. Compile AndroidManifest.xml into Android Binary XML (AXML)
	axmlBytes, err := CompileAXML(manifestData)
	if err != nil {
		return "", fmt.Errorf("compiling AndroidManifest.xml to AXML: %w", err)
	}

	// 2. Generate classes.dex
	activityName := cfg.Activity
	if strings.HasPrefix(activityName, ".") {
		activityName = cfg.PackageName + activityName
	}
	if activityName == "" {
		activityName = "alca.runtime.MainActivity"
	}

	dexBytes, err := BuildLauncherDEX(activityName, finalPermissions...)
	if err != nil {
		return "", fmt.Errorf("building classes.dex: %w", err)
	}

	// Map of APK entries: relative path in APK -> bytes
	entries := make(map[string][]byte)
	entries["AndroidManifest.xml"] = axmlBytes
	entries["classes.dex"] = dexBytes

	// 3. Collect assets from assets/
	assetsDir := filepath.Join(absProjDir, "assets")
	if fi, err := os.Stat(assetsDir); err == nil && fi.IsDir() {
		err = filepath.WalkDir(assetsDir, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(assetsDir, p)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("reading asset %s: %w", p, err)
			}
			apkPath := "assets/" + filepath.ToSlash(rel)
			entries[apkPath] = data
			return nil
		})
		if err != nil {
			return "", err
		}
	}

	// 4. Collect resources from res/
	resDir := filepath.Join(absProjDir, "res")
	if fi, err := os.Stat(resDir); err == nil && fi.IsDir() {
		err = filepath.WalkDir(resDir, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(resDir, p)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("reading resource %s: %w", p, err)
			}
			apkPath := "res/" + filepath.ToSlash(rel)
			entries[apkPath] = data
			return nil
		})
		if err != nil {
			return "", err
		}
	}

	// 5. Bundle Alca sources from src/ into assets/alca/
	srcDir := filepath.Join(absProjDir, "src")
	if fi, err := os.Stat(srcDir); err == nil && fi.IsDir() {
		// Transpile src/main.al into assets/alca/runtime.js for WebView execution
		mainAlPath := filepath.Join(srcDir, "main.al")
		mainData, err := os.ReadFile(mainAlPath)
		if err != nil {
			return "", fmt.Errorf("reading entrypoint %s: %w", mainAlPath, err)
		}

		// Run syntax and type checks before transpilation
		l := lexer.New(string(mainData))
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors()) > 0 {
			return "", fmt.Errorf("syntax errors in %s:\n%s", mainAlPath, strings.Join(p.Errors(), "\n"))
		}

		checker := typecheck.NewChecker(mainAlPath, string(mainData))
		if typeErrors := checker.Check(prog); len(typeErrors) > 0 {
			var errMsgs []string
			for _, te := range typeErrors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s:%d:%d: %s", mainAlPath, te.Line, te.Column, te.Message))
			}
			return "", fmt.Errorf("typecheck errors in %s:\n%s", mainAlPath, strings.Join(errMsgs, "\n"))
		}

		jsRuntime, err := TranspileProgram(prog)
		if err != nil {
			return "", fmt.Errorf("transpiling %s: %w", mainAlPath, err)
		}
		entries["assets/alca/runtime.js"] = []byte(jsRuntime)

		err = filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(srcDir, p)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("reading source %s: %w", p, err)
			}
			apkPath := "assets/alca/" + filepath.ToSlash(rel)
			entries[apkPath] = data
			return nil
		})
		if err != nil {
			return "", err
		}
	}

	// Bundle alca.toml if present
	if tomlData, err := os.ReadFile(filepath.Join(absProjDir, "alca.toml")); err == nil {
		entries["assets/alca/alca.toml"] = tomlData
	}

	// 6. Sign APK with v1 JAR signature (generate META-INF/MANIFEST.MF, CERT.SF, CERT.RSA)
	manifestMF, certSF, certRSA, err := SignAPK(entries)
	if err != nil {
		return "", fmt.Errorf("generating v1 signature: %w", err)
	}

	entries["META-INF/MANIFEST.MF"] = manifestMF
	entries["META-INF/CERT.SF"] = certSF
	entries["META-INF/CERT.RSA"] = certRSA

	// 7. Pack into 4-byte zipaligned ZIP archive
	alignedZipBytes, err := BuildAlignedZip(entries)
	if err != nil {
		return "", fmt.Errorf("building zipaligned archive: %w", err)
	}

	// 8. Sign with APK Signature Scheme v2 (for Android 11..14+ support)
	v2SignedAPK, err := ApplyAPKSignatureSchemeV2(alignedZipBytes)
	if err != nil {
		return "", fmt.Errorf("applying APK Signature Scheme v2: %w", err)
	}

	// 9. Write to output file
	outPath := opts.OutputPath
	if outPath == "" {
		appName := filepath.Base(absProjDir)
		if appName == "." || appName == "/" || appName == "\\" {
			appName = "app"
		}
		buildDir := filepath.Join(absProjDir, "build")
		_ = os.MkdirAll(buildDir, 0o755)
		outPath = filepath.Join(buildDir, fmt.Sprintf("%s.apk", strings.ToLower(appName)))
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	if err := os.WriteFile(outPath, v2SignedAPK, 0o644); err != nil {
		return "", fmt.Errorf("writing output APK file %s: %w", outPath, err)
	}

	return outPath, nil
}

// ValidateProject checks if a directory has a valid Alca Android project structure.
func ValidateProject(dir string) error {
	manifest := filepath.Join(dir, "AndroidManifest.xml")
	if _, err := os.Stat(manifest); err != nil {
		return fmt.Errorf("missing AndroidManifest.xml (not an Alca Android project root)")
	}

	srcMain := filepath.Join(dir, "src", "main.al")
	if _, err := os.Stat(srcMain); err != nil {
		return fmt.Errorf("missing src/main.al entrypoint")
	}

	return nil
}
