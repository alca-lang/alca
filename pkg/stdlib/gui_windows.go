//go:build windows

// Package stdlib provides the GUI standard library module for the Alca programming language.
// It supports all 3 graphical modes:
// 1. Native System Dialogs (MessageBox, Confirm, Prompt, File Open/Save, Notify) with zero dependencies.
// 2. Desktop WebView UI (HTML/CSS/JS frontend in app-window mode with Alca IPC bridge).
// 3. Native Windows & Widgets (Desktop Window with buttons, labels, inputs, checkboxes, and event callbacks).
package stdlib

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	htmlstd "html"
	"math"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/alca-lang/alca/pkg/interpreter"
	webview2 "github.com/jchv/go-webview2"
)

var (
	// Windows user32.dll procedures for zero-dependency native dialogs.
	modUser32               *syscall.LazyDLL
	procMessageBoxW         *syscall.LazyProc
	procOpenDesktopW        *syscall.LazyProc
	procSetThreadDesktop    *syscall.LazyProc
	modShell32              *syscall.LazyDLL
	procShellExecuteW       *syscall.LazyProc
	procShellExecuteExW     *syscall.LazyProc
	modKernel32             *syscall.LazyDLL
	procWaitForSingleObject *syscall.LazyProc
	procCloseHandle         *syscall.LazyProc
	procGetSystemMetrics    *syscall.LazyProc
	procLoadWebviewIcon     *syscall.LazyProc
	procSendWebviewIcon     *syscall.LazyProc
	procDestroyWebviewIcon  *syscall.LazyProc
)

type shellExecuteInfoW struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     uintptr
}

const (
	seeMaskNoCloseProcess = 0x00000040
	swShowNormal          = 1
	winInfinite           = 0xFFFFFFFF
)

func init() {
	if runtime.GOOS == "windows" {
		modUser32 = syscall.NewLazyDLL("user32.dll")
		procMessageBoxW = modUser32.NewProc("MessageBoxW")
		procOpenDesktopW = modUser32.NewProc("OpenDesktopW")
		procSetThreadDesktop = modUser32.NewProc("SetThreadDesktop")
		procGetSystemMetrics = modUser32.NewProc("GetSystemMetrics")
		procLoadWebviewIcon = modUser32.NewProc("LoadImageW")
		procSendWebviewIcon = modUser32.NewProc("SendMessageW")
		procDestroyWebviewIcon = modUser32.NewProc("DestroyIcon")
		modShell32 = syscall.NewLazyDLL("shell32.dll")
		procShellExecuteW = modShell32.NewProc("ShellExecuteW")
		procShellExecuteExW = modShell32.NewProc("ShellExecuteExW")
		modKernel32 = syscall.NewLazyDLL("kernel32.dll")
		procWaitForSingleObject = modKernel32.NewProc("WaitForSingleObject")
		procCloseHandle = modKernel32.NewProc("CloseHandle")
	}
}

// attachToDefaultDesktop attaches the current thread to the interactive "Default" desktop if running in an isolated subshell.
func attachToDefaultDesktop() {
	if runtime.GOOS == "windows" && procOpenDesktopW != nil && procSetThreadDesktop != nil {
		deskName, _ := syscall.UTF16PtrFromString("Default")
		hDesk, _, _ := procOpenDesktopW.Call(uintptr(unsafe.Pointer(deskName)), 0, 0, 0x1FF)
		if hDesk != 0 {
			procSetThreadDesktop.Call(hDesk)
		}
	}
}

// launchViaShellExecute launches a command via Windows desktop shell (explorer.exe) on the active user desktop.
func launchViaShellExecute(file, params string, wait bool) (uintptr, error) {
	if runtime.GOOS != "windows" || procShellExecuteExW == nil {
		return 0, fmt.Errorf("ShellExecuteEx not supported on this platform")
	}

	verb, _ := syscall.UTF16PtrFromString("open")
	fStr, _ := syscall.UTF16PtrFromString(file)
	pStr, _ := syscall.UTF16PtrFromString(params)

	var sei shellExecuteInfoW
	sei.cbSize = uint32(unsafe.Sizeof(sei))
	sei.fMask = seeMaskNoCloseProcess
	sei.lpVerb = verb
	sei.lpFile = fStr
	sei.lpParameters = pStr
	sei.nShow = swShowNormal

	ret, _, err := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if ret == 0 {
		return 0, fmt.Errorf("ShellExecuteEx failed: %w", err)
	}

	if wait && sei.hProcess != 0 {
		if procWaitForSingleObject != nil {
			procWaitForSingleObject.Call(sei.hProcess, winInfinite)
		}
		if procCloseHandle != nil {
			procCloseHandle.Call(sei.hProcess)
		}
		return 0, nil
	}

	return sei.hProcess, nil
}

// -----------------------------------------------------------------------------
// Mode 1: Native Dialogs (Win32 API on Windows, Zenity / KDialog on Linux)
// -----------------------------------------------------------------------------

// ShowAlert displays an informative message dialog with an OK button.
func ShowAlert(title, message string) {
	if runtime.GOOS == "windows" && procMessageBoxW != nil {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer wg.Done()

			attachToDefaultDesktop()
			t, _ := syscall.UTF16PtrFromString(title)
			m, _ := syscall.UTF16PtrFromString(message)
			// MB_OK (0x00) | MB_ICONINFORMATION (0x40) | MB_TOPMOST (0x40000) | MB_SETFOREGROUND (0x10000)
			procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00050040)
		}()
		wg.Wait()
		return
	}

	// Linux fallback: zenity, kdialog, or xmessage
	if runCommand("zenity", "--info", "--title="+title, "--text="+message) == nil {
		return
	}
	if runCommand("kdialog", "--msgbox", message, "--title", title) == nil {
		return
	}
	_ = runCommand("xmessage", "-center", title+": "+message)
}

// ShowConfirm displays a confirmation dialog with Yes/No buttons. Returns true if confirmed.
func ShowConfirm(title, message string) bool {
	if runtime.GOOS == "windows" && procMessageBoxW != nil {
		var confirmed bool
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			defer wg.Done()

			attachToDefaultDesktop()
			t, _ := syscall.UTF16PtrFromString(title)
			m, _ := syscall.UTF16PtrFromString(message)
			// MB_YESNO (0x04) | MB_ICONQUESTION (0x20) | MB_TOPMOST (0x40000) | MB_SETFOREGROUND (0x10000)
			ret, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x00050024)
			confirmed = (ret == 6) // IDYES = 6
		}()
		wg.Wait()
		return confirmed
	}

	// Linux fallback: zenity or kdialog
	if err := runCommand("zenity", "--question", "--title="+title, "--text="+message); err == nil {
		return true
	}
	if err := runCommand("kdialog", "--yesno", message, "--title", title); err == nil {
		return true
	}
	return false
}

// ShowPrompt displays a text entry dialog and returns the entered string.
func ShowPrompt(title, message, defaultVal string) string {
	if runtime.GOOS == "windows" {
		psScript := fmt.Sprintf(
			`[void][Reflection.Assembly]::LoadWithPartialName('Microsoft.VisualBasic'); [Microsoft.VisualBasic.Interaction]::InputBox('%s', '%s', '%s')`,
			escapePS(message), escapePS(title), escapePS(defaultVal),
		)
		out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", psScript).Output()
		if err == nil {
			return strings.TrimRight(string(out), "\r\n")
		}
	}

	// Linux fallback: zenity or kdialog
	out, err := exec.Command("zenity", "--entry", "--title="+title, "--text="+message, "--entry-text="+defaultVal).Output()
	if err == nil {
		return strings.TrimRight(string(out), "\r\n")
	}
	out, err = exec.Command("kdialog", "--inputbox", message, defaultVal, "--title", title).Output()
	if err == nil {
		return strings.TrimRight(string(out), "\r\n")
	}
	return defaultVal
}

// ShowOpenFile opens a native file picker dialog and returns the selected path.
func ShowOpenFile(title, filter string) string {
	if runtime.GOOS == "windows" {
		psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.OpenFileDialog
$f.Title = '%s'
if ('%s' -ne '') { $f.Filter = '%s' }
if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $f.FileName }
`, escapePS(title), escapePS(filter), escapePS(filter))
		out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", psScript).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}

	// Linux fallback: zenity or kdialog
	out, err := exec.Command("zenity", "--file-selection", "--title="+title).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	out, err = exec.Command("kdialog", "--getopenfilename", ".", filter).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// ShowSaveFile opens a native save file picker dialog and returns the selected destination.
func ShowSaveFile(title, defaultName, filter string) string {
	if runtime.GOOS == "windows" {
		psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.SaveFileDialog
$f.Title = '%s'
$f.FileName = '%s'
if ('%s' -ne '') { $f.Filter = '%s' }
if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $f.FileName }
`, escapePS(title), escapePS(defaultName), escapePS(filter), escapePS(filter))
		out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", psScript).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}

	// Linux fallback: zenity or kdialog
	out, err := exec.Command("zenity", "--file-selection", "--save", "--confirm-overwrite", "--title="+title, "--filename="+defaultName).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	out, err = exec.Command("kdialog", "--getsavefilename", defaultName, filter).Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// ShowNotification displays a desktop notification / balloon alert.
func ShowNotification(title, message string) {
	if runtime.GOOS == "windows" {
		psScript := fmt.Sprintf(`
[void][Reflection.Assembly]::LoadWithPartialName('System.Windows.Forms')
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.ShowBalloonTip(4000, '%s', '%s', [System.Windows.Forms.ToolTipIcon]::Info)
Start-Sleep -Seconds 1
$n.Dispose()
`, escapePS(title), escapePS(message))
		_ = exec.Command("powershell", "-NoProfile", "-STA", "-Command", psScript).Start()
		return
	}

	// Linux fallback: notify-send
	_ = exec.Command("notify-send", title, message).Run()
}

// GetScreenSize returns the primary display width and height in pixels.
func GetScreenSize() (int, int) {
	if runtime.GOOS == "windows" && procGetSystemMetrics != nil {
		w, _, _ := procGetSystemMetrics.Call(0) // SM_CXSCREEN = 0
		h, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN = 1
		if w > 0 && h > 0 {
			return int(w), int(h)
		}
	}
	// Fallback via PowerShell on Windows
	if runtime.GOOS == "windows" {
		out, err := exec.Command("powershell", "-NoProfile", "-Command", "[System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Width.ToString() + ' ' + [System.Windows.Forms.Screen]::PrimaryScreen.Bounds.Height.ToString()").Output()
		if err == nil {
			parts := strings.Fields(strings.TrimSpace(string(out)))
			if len(parts) == 2 {
				w, _ := strconv.Atoi(parts[0])
				h, _ := strconv.Atoi(parts[1])
				if w > 0 && h > 0 {
					return w, h
				}
			}
		}
	} else {
		// Linux fallback: xdpyinfo
		out, err := exec.Command("xdpyinfo").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "dimensions:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						dim := strings.Split(fields[1], "x")
						if len(dim) == 2 {
							w, _ := strconv.Atoi(dim[0])
							h, _ := strconv.Atoi(dim[1])
							return w, h
						}
					}
				}
			}
		}
	}
	return 1920, 1080
}

// CaptureScreen captures the primary screen and saves it as an image file (PNG/JPG/BMP).
func CaptureScreen(destPath string) (string, error) {
	if destPath == "" {
		destPath = fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
	}
	absPath, err := filepath.Abs(destPath)
	if err == nil {
		destPath = absPath
	}

	// Ensure directory exists
	dir := filepath.Dir(destPath)
	_ = os.MkdirAll(dir, 0755)

	if runtime.GOOS == "windows" {
		psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms,System.Drawing
$bounds = [System.Windows.Forms.Screen]::PrimaryScreen.Bounds
$bmp = New-Object System.Drawing.Bitmap $bounds.Width, $bounds.Height
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
$bmp.Save('%s')
$g.Dispose()
$bmp.Dispose()
`, escapePS(destPath))

		cmd := exec.Command("powershell", "-NoProfile", "-STA", "-Command", psScript)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("screenshot failed: %w: %s", err, string(out))
		}
		return destPath, nil
	}

	// Linux fallback: scrot, import (imagemagick), or maim
	if err := runCommand("scrot", destPath); err == nil {
		return destPath, nil
	}
	if err := runCommand("import", "-window", "root", destPath); err == nil {
		return destPath, nil
	}
	if err := runCommand("maim", destPath); err == nil {
		return destPath, nil
	}
	return "", fmt.Errorf("no screen capture utility found (install scrot, maim or imagemagick)")
}

// -----------------------------------------------------------------------------
// Mode 2: Desktop WebView Application (HTML/CSS/JS with Alca IPC Bridge)
// -----------------------------------------------------------------------------

// WebviewApp represents a desktop application window rendering HTML/CSS/JS.
type WebviewApp struct {
	Title        string
	Width        int
	Height       int
	Engine       string
	ServerURL    string
	listener     net.Listener
	server       *http.Server
	serverDone   chan struct{}
	cmd          *exec.Cmd
	embedded     webview2.WebView
	embeddedDone chan struct{}
	iconHandle   uintptr
	hProcess     uintptr
	events       chan string
	mu           sync.Mutex
}

type WebviewOptions struct {
	Title  string
	Icon   string
	Engine string
	Width  int
	Height int
}

const defaultWebviewIcon = "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Crect width='64' height='64' rx='14' fill='%239be564'/%3E%3Ctext x='32' y='46' text-anchor='middle' font-family='Arial,sans-serif' font-size='46' font-weight='700' fill='%23101820'%3EA%3C/text%3E%3C/svg%3E"

// LaunchWebview starts an embedded local HTTP server and opens a dedicated borderless
// app window using an installed desktop browser.
func LaunchWebview(title, htmlOrURL string, width, height int, configs ...WebviewOptions) (*WebviewApp, error) {
	var options WebviewOptions
	if len(configs) > 0 {
		options = configs[0]
	}
	engine, err := normalizeWebviewEngine(options.Engine)
	if err != nil {
		return nil, err
	}
	if options.Title != "" {
		title = options.Title
	}
	if options.Width > 0 {
		width = options.Width
	}
	if options.Height > 0 {
		height = options.Height
	}
	if width <= 0 {
		width = 900
	}
	if height <= 0 {
		height = 650
	}
	browserPath, selectedEngine, err := resolveWebviewBrowser(engine, runtime.GOOS, exec.LookPath, os.Stat)
	if err != nil {
		return nil, err
	}

	app := &WebviewApp{
		Title:  title,
		Width:  width,
		Height: height,
		Engine: selectedEngine,
		events: make(chan string, 64),
	}

	targetURL := htmlOrURL
	// If input is raw HTML code or file path, serve it via embedded HTTP server
	if strings.Contains(htmlOrURL, "<") || !strings.HasPrefix(htmlOrURL, "http://") && !strings.HasPrefix(htmlOrURL, "https://") {
		var content []byte
		if strings.Contains(htmlOrURL, "<") {
			content = []byte(htmlOrURL)
		} else {
			var err error
			content, err = os.ReadFile(htmlOrURL)
			if err != nil {
				return nil, fmt.Errorf("cannot read html file %q: %w", htmlOrURL, err)
			}
		}

		// Inject lightweight Alca JavaScript IPC bridge into HTML if needed
		htmlStr := string(content)
		iconURL, err := webviewIconURL(options.Icon)
		if err != nil {
			return nil, err
		}
		htmlStr = injectWebviewMetadata(htmlStr, title, iconURL)
		if !strings.Contains(htmlStr, "window.al") {
			bridgeScript := `
<script>
window.al = {
  send: function(msg) {
    fetch('/__alca_ipc__', {method: 'POST', body: String(msg)});
  }
};
</script>
`
			if strings.Contains(htmlStr, "</body>") {
				htmlStr = strings.Replace(htmlStr, "</body>", bridgeScript+"</body>", 1)
			} else {
				htmlStr += bridgeScript
			}
		}
		closeScript := `
<script>
window.addEventListener('pagehide', function() {
  navigator.sendBeacon('/__alca_close__', '');
});
</script>
`
		if strings.Contains(htmlStr, "</body>") {
			htmlStr = strings.Replace(htmlStr, "</body>", closeScript+"</body>", 1)
		} else {
			htmlStr += closeScript
		}

		// Start embedded local app server
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("failed to bind local webview server: %w", err)
		}
		app.listener = ln
		app.serverDone = make(chan struct{})
		port := ln.Addr().(*net.TCPAddr).Port
		targetURL = fmt.Sprintf("http://127.0.0.1:%d", port)

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(htmlStr))
		})
		mux.HandleFunc("/api/processes", func(w http.ResponseWriter, r *http.Request) {
			procs, _ := SystemProcesses()
			data, _ := json.Marshal(procs)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		})
		mux.HandleFunc("/api/sysinfo", func(w http.ResponseWriter, r *http.Request) {
			tot, free, used, pct, _ := RamUsageStats()
			info := map[string]interface{}{
				"cpu_count":  runtime.NumCPU(),
				"platform":   runtime.GOOS,
				"arch":       runtime.GOARCH,
				"uptime_sec": int64(SystemUptime()),
				"ram": map[string]interface{}{
					"total_bytes":  tot,
					"used_bytes":   used,
					"free_bytes":   free,
					"percent_used": pct,
					"total_gb":     math.Round((float64(tot)/(1024*1024*1024))*10) / 10,
					"used_gb":      math.Round((float64(used)/(1024*1024*1024))*10) / 10,
				},
			}
			data, _ := json.Marshal(info)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		})
		mux.HandleFunc("/api/kill", func(w http.ResponseWriter, r *http.Request) {
			pidStr := r.URL.Query().Get("pid")
			pid, _ := strconv.Atoi(pidStr)
			err := KillProcess(pid)
			w.Header().Set("Content-Type", "application/json")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"ok":false,"error":%q}`, err.Error())))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		})
		mux.HandleFunc("/__alca_ipc__", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		mux.HandleFunc("/__alca_close__", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
			go func() {
				_ = app.server.Close()
			}()
		})

		app.server = &http.Server{Handler: mux}
		go func() {
			_ = app.server.Serve(ln)
			close(app.serverDone)
		}()
	}

	app.ServerURL = targetURL
	if selectedEngine == "webview2" {
		app.embeddedDone = make(chan struct{})
		iconPath := options.Icon
		if strings.HasPrefix(iconPath, "data:image/") || strings.HasPrefix(iconPath, "http://") || strings.HasPrefix(iconPath, "https://") {
			iconPath = ""
		}
		if err := app.startWebView2(title, targetURL, iconPath); err != nil {
			if app.server != nil {
				_ = app.server.Close()
			}
			return nil, err
		}
		return app, nil
	}

	args := []string{
		fmt.Sprintf("--app=%s", targetURL),
		fmt.Sprintf("--window-size=%d,%d", width, height),
	}
	if runtime.GOOS == "windows" {
		args = append(args, fmt.Sprintf("--app-window-title=%s", title))
	}
	cmd := exec.Command(browserPath, args...)

	app.cmd = cmd
	if err := cmd.Start(); err != nil {
		if app.server != nil {
			_ = app.server.Close()
		}
		return nil, fmt.Errorf("failed to open desktop webview: %w", err)
	}

	return app, nil
}

func normalizeWebviewEngine(engine string) (string, error) {
	engine = strings.ToLower(strings.TrimSpace(engine))
	if engine == "" {
		engine = "auto"
	}
	switch engine {
	case "auto", "chrome", "chromium", "webview2":
		return engine, nil
	default:
		return "", fmt.Errorf("unsupported WebView engine %q: choose auto, chrome, chromium, or webview2", engine)
	}
}

func webviewBrowserCandidates(engine, goos string) []string {
	var candidates []string
	addChrome := func() {
		switch goos {
		case "windows":
			for _, dir := range []string{os.Getenv("PROGRAMFILES"), os.Getenv("PROGRAMFILES(X86)"), os.Getenv("LOCALAPPDATA")} {
				if dir != "" {
					candidates = append(candidates, filepath.Join(dir, "Google", "Chrome", "Application", "chrome.exe"))
				}
			}
			candidates = append(candidates, "chrome.exe")
		case "darwin":
			candidates = append(candidates,
				"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
				"google-chrome",
			)
		default:
			candidates = append(candidates, "google-chrome", "google-chrome-stable", "chrome")
		}
	}
	addChromium := func() {
		switch goos {
		case "windows":
			for _, dir := range []string{os.Getenv("PROGRAMFILES"), os.Getenv("LOCALAPPDATA")} {
				if dir != "" {
					candidates = append(candidates, filepath.Join(dir, "Chromium", "Application", "chrome.exe"))
				}
			}
			candidates = append(candidates, "chromium.exe")
		case "darwin":
			candidates = append(candidates,
				"/Applications/Chromium.app/Contents/MacOS/Chromium",
				"chromium",
			)
		default:
			candidates = append(candidates, "chromium", "chromium-browser")
		}
	}

	switch engine {
	case "chrome":
		addChrome()
	case "chromium":
		addChromium()
	default:
		addChrome()
		addChromium()
	}
	return candidates
}

func resolveWebviewBrowser(engine, goos string, lookPath func(string) (string, error), stat func(string) (os.FileInfo, error)) (string, string, error) {
	normalized, err := normalizeWebviewEngine(engine)
	if err != nil {
		return "", "", err
	}
	if normalized == "webview2" {
		if goos != "windows" {
			return "", "", fmt.Errorf("WebView2 is only available on Windows")
		}
		return "", "webview2", nil
	}
	for _, candidate := range webviewBrowserCandidates(normalized, goos) {
		if _, err := stat(candidate); err == nil {
			selected := normalized
			if selected == "auto" {
				selected = "chrome"
				if strings.Contains(strings.ToLower(candidate), "chromium") {
					selected = "chromium"
				}
			}
			return candidate, selected, nil
		}
		if path, err := lookPath(candidate); err == nil {
			selected := normalized
			if selected == "auto" {
				selected = "chrome"
				if strings.Contains(strings.ToLower(candidate), "chromium") {
					selected = "chromium"
				}
			}
			return path, selected, nil
		}
	}
	if normalized == "auto" {
		return "", "", fmt.Errorf("no supported WebView engine found: install Google Chrome or Chromium")
	}
	return "", "", fmt.Errorf("WebView engine %q is not installed", normalized)
}

func (app *WebviewApp) startWebView2(title, targetURL, iconPath string) error {
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		view := webview2.NewWithOptions(webview2.WebViewOptions{
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title:  title,
				Width:  uint(app.Width),
				Height: uint(app.Height),
				Center: true,
			},
		})
		if view == nil {
			ready <- fmt.Errorf("failed to initialize WebView2: install or repair the Microsoft Edge WebView2 Runtime")
			close(app.embeddedDone)
			return
		}
		if iconPath != "" {
			handle, err := loadWebviewWindowIcon(view.Window(), iconPath)
			if err != nil {
				ready <- err
				view.Destroy()
				view.Run()
				view.Destroy()
				close(app.embeddedDone)
				return
			}
			app.iconHandle = handle
		}

		app.mu.Lock()
		app.embedded = view
		app.mu.Unlock()
		view.Navigate(targetURL)
		ready <- nil
		view.Run()
		view.Destroy()
		if app.iconHandle != 0 && procDestroyWebviewIcon != nil {
			procDestroyWebviewIcon.Call(app.iconHandle)
			app.iconHandle = 0
		}

		app.mu.Lock()
		app.embedded = nil
		app.mu.Unlock()
		close(app.embeddedDone)
	}()
	return <-ready
}

func loadWebviewWindowIcon(hwnd unsafe.Pointer, path string) (uintptr, error) {
	loadPath := path
	cleanup := func() {}
	if strings.HasPrefix(path, "embed:") {
		data, err := ReadEmbeddedResource(strings.TrimPrefix(path, "embed:"))
		if err != nil {
			return 0, err
		}
		loadPath, cleanup, err = writeWebviewIconTemp(data)
		if err != nil {
			return 0, err
		}
	} else if data, resourceErr := ReadEmbeddedResource(path); resourceErr == nil {
		tempPath, tempCleanup, tempErr := writeWebviewIconTemp(data)
		if tempErr != nil {
			return 0, tempErr
		}
		loadPath, cleanup = tempPath, tempCleanup
	} else if _, statErr := os.Stat(path); statErr != nil {
		return 0, fmt.Errorf("cannot read WebView2 window icon %q: %w", path, statErr)
	}
	defer cleanup()
	if err := validateWebviewWindowIcon(loadPath); err != nil {
		return 0, err
	}
	if procLoadWebviewIcon == nil || procSendWebviewIcon == nil {
		return 0, fmt.Errorf("native window icon support is unavailable")
	}
	iconPath, err := syscall.UTF16PtrFromString(loadPath)
	if err != nil {
		return 0, fmt.Errorf("invalid WebView2 window icon path: %w", err)
	}
	icon, _, callErr := procLoadWebviewIcon.Call(
		0,
		uintptr(unsafe.Pointer(iconPath)),
		1,
		0,
		0,
		0x10|0x40,
	)
	if icon == 0 {
		return 0, fmt.Errorf("cannot load WebView2 window icon %q: %w", path, callErr)
	}
	window := uintptr(hwnd)
	procSendWebviewIcon.Call(window, 0x80, 0, icon)
	procSendWebviewIcon.Call(window, 0x80, 1, icon)
	return icon, nil
}

func writeWebviewIconTemp(data []byte) (string, func(), error) {
	temp, err := os.CreateTemp("", "alca-window-icon-*.ico")
	if err != nil {
		return "", func() {}, fmt.Errorf("cannot prepare embedded window icon: %w", err)
	}
	loadPath := temp.Name()
	cleanup := func() { _ = os.Remove(loadPath) }
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("cannot write embedded window icon: %w", err)
	}
	if err := temp.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("cannot close embedded window icon: %w", err)
	}
	return loadPath, cleanup, nil
}

func validateWebviewWindowIcon(path string) error {
	if strings.ToLower(filepath.Ext(path)) != ".ico" {
		return fmt.Errorf("WebView2 window icon %q must be an .ico file", path)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("cannot read WebView2 window icon %q: %w", path, err)
	}
	return nil
}

func webviewIconURL(icon string) (string, error) {
	if icon == "" {
		return defaultWebviewIcon, nil
	}
	if strings.HasPrefix(icon, "data:image/") || strings.HasPrefix(icon, "http://") || strings.HasPrefix(icon, "https://") {
		return icon, nil
	}
	var data []byte
	var err error
	if strings.HasPrefix(icon, "embed:") {
		data, err = ReadEmbeddedResource(strings.TrimPrefix(icon, "embed:"))
	} else {
		data, err = ReadEmbeddedResource(icon)
		if err != nil {
			data, err = os.ReadFile(icon)
		}
	}
	if err != nil {
		return "", fmt.Errorf("cannot read WebView icon %q: %w", icon, err)
	}
	contentType := mime.TypeByExtension(filepath.Ext(icon))
	if contentType == "" || !strings.HasPrefix(contentType, "image/") {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("WebView icon %q is not a supported image", icon)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func injectWebviewMetadata(source, title, iconURL string) string {
	var metadata strings.Builder
	if title != "" {
		metadata.WriteString("<title>")
		metadata.WriteString(htmlstd.EscapeString(title))
		metadata.WriteString("</title>")
	}
	metadata.WriteString(`<link rel="icon" href="`)
	metadata.WriteString(htmlstd.EscapeString(iconURL))
	metadata.WriteString(`">`)

	headStart := strings.Index(strings.ToLower(source), "<head")
	if headStart >= 0 {
		if headEnd := strings.Index(source[headStart:], ">"); headEnd >= 0 {
			insertAt := headStart + headEnd + 1
			return source[:insertAt] + metadata.String() + source[insertAt:]
		}
	}
	return metadata.String() + source
}

// Wait blocks until the webview window is closed by the user.
func (app *WebviewApp) Wait() {
	if app.Engine == "webview2" && app.embeddedDone != nil {
		<-app.embeddedDone
		if app.server != nil {
			_ = app.server.Close()
		}
	} else if app.serverDone != nil {
		<-app.serverDone
		if app.hProcess != 0 && procCloseHandle != nil {
			procCloseHandle.Call(app.hProcess)
			app.hProcess = 0
		}
		if app.cmd != nil {
			_ = app.cmd.Wait()
		}
	} else if app.hProcess != 0 && procWaitForSingleObject != nil {
		procWaitForSingleObject.Call(app.hProcess, winInfinite)
		if procCloseHandle != nil {
			procCloseHandle.Call(app.hProcess)
		}
		app.hProcess = 0
	} else if app.cmd != nil {
		_ = app.cmd.Wait()
	}
}

// Close shuts down the webview application window and local server.
func (app *WebviewApp) Close() {
	app.mu.Lock()
	defer app.mu.Unlock()

	if app.embedded != nil {
		app.embedded.Terminate()
	}
	if app.hProcess != 0 {
		modKernel32 := syscall.NewLazyDLL("kernel32.dll")
		procTerminateProcess := modKernel32.NewProc("TerminateProcess")
		procTerminateProcess.Call(app.hProcess, 0)
		if procCloseHandle != nil {
			procCloseHandle.Call(app.hProcess)
		}
		app.hProcess = 0
	}
	if app.cmd != nil && app.cmd.Process != nil {
		_ = app.cmd.Process.Kill()
	}
	if app.server != nil {
		_ = app.server.Close()
	}
}

// -----------------------------------------------------------------------------
// Mode 3: Native Desktop Windows & Controls (Pure Native Desktop Widgets)
// -----------------------------------------------------------------------------

// RunNativeTaskManager launches a pure native Windows Forms / Win32 Task Manager window with zero webview overhead.
func RunNativeTaskManager(title string, width, height int) error {
	return RunTrueNativeWin32TaskManager(title, width, height)
}

// -----------------------------------------------------------------------------
// Registration into Alca Environment
// -----------------------------------------------------------------------------

// RegisterGUIBuiltins registers all GUI functions into the Alca interpreter environment.
func RegisterGUIBuiltins(env *interpreter.Environment) {
	guiModule := interpreter.NewMap()

	register := func(name string, fn func(args ...interpreter.Value) interpreter.Value) {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		guiModule.Set(name, builtin)
		env.Set("gui_"+name, builtin)
	}

	// 1. Native Dialogs
	register("alert", func(args ...interpreter.Value) interpreter.Value {
		title := "Alca Alert"
		msg := ""
		if len(args) == 1 {
			msg = interpreter.CoerceToString(args[0])
		} else if len(args) >= 2 {
			title = interpreter.CoerceToString(args[0])
			msg = interpreter.CoerceToString(args[1])
		}
		ShowAlert(title, msg)
		return interpreter.NULL
	})

	register("msgbox", func(args ...interpreter.Value) interpreter.Value {
		title := "Alca Message"
		msg := ""
		if len(args) >= 2 {
			title = interpreter.CoerceToString(args[0])
			msg = interpreter.CoerceToString(args[1])
		} else if len(args) == 1 {
			msg = interpreter.CoerceToString(args[0])
		}
		ShowAlert(title, msg)
		return interpreter.NULL
	})

	register("confirm", func(args ...interpreter.Value) interpreter.Value {
		title := "Confirm"
		msg := "Are you sure?"
		if len(args) == 1 {
			msg = interpreter.CoerceToString(args[0])
		} else if len(args) >= 2 {
			title = interpreter.CoerceToString(args[0])
			msg = interpreter.CoerceToString(args[1])
		}
		if ShowConfirm(title, msg) {
			return interpreter.TRUE
		}
		return interpreter.FALSE
	})

	register("prompt", func(args ...interpreter.Value) interpreter.Value {
		title := "Input"
		msg := "Enter value:"
		def := ""
		if len(args) >= 1 {
			title = interpreter.CoerceToString(args[0])
		}
		if len(args) >= 2 {
			msg = interpreter.CoerceToString(args[1])
		}
		if len(args) >= 3 {
			def = interpreter.CoerceToString(args[2])
		}
		res := ShowPrompt(title, msg, def)
		return &interpreter.StringValue{Val: res}
	})

	register("open_file", func(args ...interpreter.Value) interpreter.Value {
		title := "Open File"
		filter := ""
		if len(args) >= 1 {
			title = interpreter.CoerceToString(args[0])
		}
		if len(args) >= 2 {
			filter = interpreter.CoerceToString(args[1])
		}
		res := ShowOpenFile(title, filter)
		return &interpreter.StringValue{Val: res}
	})

	register("save_file", func(args ...interpreter.Value) interpreter.Value {
		title := "Save File"
		name := ""
		filter := ""
		if len(args) >= 1 {
			title = interpreter.CoerceToString(args[0])
		}
		if len(args) >= 2 {
			name = interpreter.CoerceToString(args[1])
		}
		if len(args) >= 3 {
			filter = interpreter.CoerceToString(args[2])
		}
		res := ShowSaveFile(title, name, filter)
		return &interpreter.StringValue{Val: res}
	})

	register("notify", func(args ...interpreter.Value) interpreter.Value {
		title := "Alca Notification"
		msg := ""
		if len(args) >= 2 {
			title = interpreter.CoerceToString(args[0])
			msg = interpreter.CoerceToString(args[1])
		} else if len(args) == 1 {
			msg = interpreter.CoerceToString(args[0])
		}
		ShowNotification(title, msg)
		return interpreter.NULL
	})

	register("screen_size", func(args ...interpreter.Value) interpreter.Value {
		w, h := GetScreenSize()
		m := interpreter.NewMap()
		m.Set("width", &interpreter.IntegerValue{Val: int64(w)})
		m.Set("height", &interpreter.IntegerValue{Val: int64(h)})
		return m
	})

	register("screenshot", func(args ...interpreter.Value) interpreter.Value {
		path := ""
		if len(args) > 0 {
			path = interpreter.CoerceToString(args[0])
		}
		saved, err := CaptureScreen(path)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return &interpreter.StringValue{Val: saved}
	})

	// 2. Desktop WebView
	register("webview", func(args ...interpreter.Value) interpreter.Value {
		if len(args) < 2 {
			return interpreter.NewError(400, "webview requires (title, htmlOrURL) arguments")
		}
		title := interpreter.CoerceToString(args[0])
		target := interpreter.CoerceToString(args[1])
		options := WebviewOptions{}
		w := 900
		h := 650
		if len(args) >= 3 {
			if iv, ok := args[2].(*interpreter.IntegerValue); ok {
				w = int(iv.Val)
			}
		}
		if len(args) >= 4 {
			if iv, ok := args[3].(*interpreter.IntegerValue); ok {
				h = int(iv.Val)
			}
		}
		if len(args) >= 5 {
			optionsMap, ok := args[4].(*interpreter.MapValue)
			if !ok {
				return interpreter.NewError(400, "webview options must be a map")
			}
			for key, dest := range map[string]*string{
				"title":  &options.Title,
				"icon":   &options.Icon,
				"engine": &options.Engine,
			} {
				if value, exists := optionsMap.Get(key); exists {
					text, ok := value.(*interpreter.StringValue)
					if !ok {
						return interpreter.NewError(400, "webview option '"+key+"' must be a string")
					}
					*dest = text.Val
				}
			}
			for key, dest := range map[string]*int{
				"width":  &options.Width,
				"height": &options.Height,
			} {
				if value, exists := optionsMap.Get(key); exists {
					number, ok := value.(*interpreter.IntegerValue)
					if !ok || number.Val < 1 || number.Val > 8192 {
						return interpreter.NewError(400, "webview option '"+key+"' must be an integer between 1 and 8192")
					}
					*dest = int(number.Val)
				}
			}
		}

		app, err := LaunchWebview(title, target, w, h, options)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}

		webviewObj := interpreter.NewMap()
		webviewObj.Set("url", &interpreter.StringValue{Val: app.ServerURL})
		webviewObj.Set("title", &interpreter.StringValue{Val: app.Title})
		webviewObj.Set("engine", &interpreter.StringValue{Val: app.Engine})
		webviewObj.Set("close", &interpreter.BuiltinFunction{
			Name: "close",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				app.Close()
				return interpreter.NULL
			},
		})
		webviewObj.Set("wait", &interpreter.BuiltinFunction{
			Name: "wait",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				app.Wait()
				return interpreter.NULL
			},
		})
		return webviewObj
	})

	registerTaskMgr := func(name string) {
		register(name, func(args ...interpreter.Value) interpreter.Value {
			title := "Alca Task Manager"
			w := 1040
			h := 680
			if len(args) >= 1 {
				title = interpreter.CoerceToString(args[0])
			}
			if len(args) >= 2 {
				if iv, ok := args[1].(*interpreter.IntegerValue); ok {
					w = int(iv.Val)
				}
			}
			if len(args) >= 3 {
				if iv, ok := args[2].(*interpreter.IntegerValue); ok {
					h = int(iv.Val)
				}
			}

			if err := RunTrueNativeWin32TaskManager(title, w, h); err != nil {
				return interpreter.NewError(500, err.Error())
			}
			return interpreter.TRUE
		})
	}
	registerTaskMgr("task_manager")
	registerTaskMgr("native_task_manager")

	// 3. Native Windows & Widgets
	makeWindow := func(forced string, args ...interpreter.Value) interpreter.Value {
		title, width, height, backend := parseWindowArgs(args)
		if forced != "" {
			backend = forced
		}
		return windowValue(title, width, height, backend)
	}
	register("window", func(args ...interpreter.Value) interpreter.Value {
		return makeWindow("", args...)
	})
	register("tk_window", func(args ...interpreter.Value) interpreter.Value {
		return makeWindow("tk", args...)
	})
	register("qt_window", func(args ...interpreter.Value) interpreter.Value {
		return makeWindow("qt", args...)
	})

	// 4. GUI backend selection and capabilities
	register("set_backend", func(args ...interpreter.Value) interpreter.Value {
		if len(args) > 0 {
			SetDefaultGUIBackend(interpreter.CoerceToString(args[0]))
		}
		return interpreter.TRUE
	})

	register("backends", func(args ...interpreter.Value) interpreter.Value {
		result := interpreter.NewMap()
		for name, available := range BackendInfo() {
			switch v := available.(type) {
			case bool:
				if v {
					result.Set(name, interpreter.TRUE)
				} else {
					result.Set(name, interpreter.FALSE)
				}
			case string:
				result.Set(name, &interpreter.StringValue{Val: v})
			}
		}
		return result
	})

	register("width", func(args ...interpreter.Value) interpreter.Value {
		w, _ := GetScreenSize()
		return &interpreter.IntegerValue{Val: int64(w)}
	})
	register("height", func(args ...interpreter.Value) interpreter.Value {
		_, h := GetScreenSize()
		return &interpreter.IntegerValue{Val: int64(h)}
	})
	env.Set("gui", guiModule)
	env.Set("std.gui", guiModule)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func escapePS(s string) string {
	r := strings.ReplaceAll(s, `'`, `''`)
	return strings.ReplaceAll(r, "`", "``")
}
