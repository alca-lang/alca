//go:build windows

package stdlib

import (
	htmlstd "html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectWebviewMetadata(t *testing.T) {
	source := "<html><head></head><body>App</body></html>"
	got := injectWebviewMetadata(source, `A "quoted" title`, defaultWebviewIcon)

	for _, want := range []string{
		"<title>A &#34;quoted&#34; title</title>",
		`<link rel="icon" href="` + htmlstd.EscapeString(defaultWebviewIcon) + `">`,
		"<body>App</body>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("metadata output missing %q: %s", want, got)
		}
	}
}

func TestWebviewIconURLDefaultsToAlcaIcon(t *testing.T) {
	got, err := webviewIconURL("")
	if err != nil {
		t.Fatal(err)
	}
	if got != defaultWebviewIcon || !strings.Contains(got, "%3EA%3C") {
		t.Fatalf("default icon does not contain the Alca letter A: %q", got)
	}
}

func TestValidateWebviewWindowIcon(t *testing.T) {
	icon := filepath.Join(t.TempDir(), "app.ico")
	if err := os.WriteFile(icon, []byte("icon"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateWebviewWindowIcon(icon); err != nil {
		t.Fatalf("valid ICO path rejected: %v", err)
	}
	if err := validateWebviewWindowIcon(strings.TrimSuffix(icon, ".ico") + ".png"); err == nil {
		t.Fatal("expected non-ICO window icon to be rejected")
	}
	if err := validateWebviewWindowIcon(filepath.Join(t.TempDir(), "missing.ico")); err == nil {
		t.Fatal("expected missing icon file to be rejected")
	}
}

func TestNormalizeWebviewEngine(t *testing.T) {
	for input, want := range map[string]string{
		"":         "auto",
		" AUTO ":   "auto",
		"Chrome":   "chrome",
		"chromium": "chromium",
		"WebView2": "webview2",
	} {
		got, err := normalizeWebviewEngine(input)
		if err != nil || got != want {
			t.Errorf("normalizeWebviewEngine(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := normalizeWebviewEngine("firefox"); err == nil {
		t.Fatal("expected unsupported engine to return an error")
	}
}

func TestResolveWebView2Engine(t *testing.T) {
	path, engine, err := resolveWebviewBrowser("webview2", "windows", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" || engine != "webview2" {
		t.Fatalf("resolved (%q, %q); want embedded WebView2", path, engine)
	}
	if _, _, err := resolveWebviewBrowser("webview2", "linux", nil, nil); err == nil {
		t.Fatal("expected WebView2 to be rejected outside Windows")
	}
}

func TestResolveWebviewBrowserHonorsSelectedEngine(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "chromium" {
			return "C:\\tools\\chromium.exe", nil
		}
		return "", os.ErrNotExist
	}
	stat := func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}

	path, engine, err := resolveWebviewBrowser("chromium", "linux", lookPath, stat)
	if err != nil {
		t.Fatal(err)
	}
	if path != "C:\\tools\\chromium.exe" || engine != "chromium" {
		t.Fatalf("resolved (%q, %q); want Chromium binary and engine", path, engine)
	}

	_, _, err = resolveWebviewBrowser("chrome", "linux", lookPath, stat)
	if err == nil || !strings.Contains(err.Error(), `"chrome" is not installed`) {
		t.Fatalf("explicitly selected missing engine should fail, got %v", err)
	}
}
