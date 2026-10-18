//go:build windows

// Package stdlib provides the GUI standard library module for the Alca programming language.
// It supports all 3 graphical modes:
// 1. Native System Dialogs (MessageBox, Confirm, Prompt, File Open/Save, Notify) with zero dependencies.
// 2. Desktop WebView UI (HTML/CSS/JS frontend in app-window mode with Alca IPC bridge).
// 3. Native Windows & Widgets (Desktop Window with buttons, labels, inputs, checkboxes, and event callbacks).
package stdlib

import (
	"fmt"
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
	Title     string
	Width     int
	Height    int
	ServerURL string
	listener  net.Listener
	server    *http.Server
	cmd       *exec.Cmd
	hProcess  uintptr
	events    chan string
	mu        sync.Mutex
}

// LaunchWebview starts an embedded local HTTP server and opens a dedicated borderless
// app window with zero external dependencies (using MS Edge in app mode on Windows, Chromium on Linux).
func LaunchWebview(title, htmlOrURL string, width, height int) (*WebviewApp, error) {
	if width <= 0 {
		width = 900
	}
	if height <= 0 {
		height = 650
	}

	app := &WebviewApp{
		Title:  title,
		Width:  width,
		Height: height,
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

		tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("alca_app_%d.html", time.Now().UnixNano()))
		if err := os.WriteFile(tmpFile, []byte(htmlStr), 0644); err == nil {
			targetURL = "file:///" + filepath.ToSlash(tmpFile)
		} else {
			// Fallback: Start local server
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return nil, fmt.Errorf("failed to bind local webview server: %w", err)
			}
			app.listener = ln
			port := ln.Addr().(*net.TCPAddr).Port
			targetURL = fmt.Sprintf("http://127.0.0.1:%d", port)

			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(htmlStr))
			})
			mux.HandleFunc("/__alca_ipc__", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			app.server = &http.Server{Handler: mux}
			go func() {
				_ = app.server.Serve(ln)
			}()
		}
	}

	app.ServerURL = targetURL

	// Open in standalone application mode
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// MS Edge in --app mode runs without browser UI, address bars or tabs
		edgePaths := []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
			"msedge.exe",
		}

		edgeExe := ""
		for _, ep := range edgePaths {
			if _, err := os.Stat(ep); err == nil || ep == "msedge.exe" {
				edgeExe = ep
				break
			}
		}

		appArg := fmt.Sprintf("--app=%s", targetURL)
		sizeArg := fmt.Sprintf("--window-size=%d,%d", width, height)
		titleArg := fmt.Sprintf("--app-window-title=%s", title)
		params := fmt.Sprintf("%s %s %s", appArg, sizeArg, titleArg)

		hProc, err := launchViaShellExecute(edgeExe, params, false)
		if err == nil {
			app.hProcess = hProc
			return app, nil
		}

		cmd = exec.Command(edgeExe, appArg, sizeArg, titleArg)
	} else {
		// Linux: Chromium or Chrome in app mode
		cmd = exec.Command("google-chrome", fmt.Sprintf("--app=%s", targetURL), fmt.Sprintf("--window-size=%d,%d", width, height))
		if err := cmd.Start(); err != nil {
			cmd = exec.Command("chromium", fmt.Sprintf("--app=%s", targetURL), fmt.Sprintf("--window-size=%d,%d", width, height))
		}
	}

	app.cmd = cmd
	if err := cmd.Start(); err != nil {
		if app.server != nil {
			_ = app.server.Close()
		}
		return nil, fmt.Errorf("failed to open desktop webview: %w", err)
	}

	return app, nil
}

// Wait blocks until the webview window is closed by the user.
func (app *WebviewApp) Wait() {
	if app.hProcess != 0 && procWaitForSingleObject != nil {
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

// Widget represents a GUI control inside a native window.
type Widget struct {
	Type     string
	Text     string
	Value    string
	Callback interpreter.Value
}

// NativeWindow represents a pure native desktop window with interactive widgets.
type NativeWindow struct {
	Title   string
	Width   int
	Height  int
	Widgets []*Widget
	mu      sync.Mutex
}

// NewNativeWindow creates a new native window specification.
func NewNativeWindow(title string, width, height int) *NativeWindow {
	if width <= 0 {
		width = 500
	}
	if height <= 0 {
		height = 400
	}
	return &NativeWindow{
		Title:   title,
		Width:   width,
		Height:  height,
		Widgets: make([]*Widget, 0),
	}
}

// AddLabel adds a static text label.
func (w *NativeWindow) AddLabel(text string) *Widget {
	w.mu.Lock()
	defer w.mu.Unlock()
	widget := &Widget{Type: "label", Text: text}
	w.Widgets = append(w.Widgets, widget)
	return widget
}

// AddButton adds a clickable button with an optional action callback.
func (w *NativeWindow) AddButton(text string, callback interpreter.Value) *Widget {
	w.mu.Lock()
	defer w.mu.Unlock()
	widget := &Widget{Type: "button", Text: text, Callback: callback}
	w.Widgets = append(w.Widgets, widget)
	return widget
}

// AddInput adds a single-line text input field.
func (w *NativeWindow) AddInput(placeholder string) *Widget {
	w.mu.Lock()
	defer w.mu.Unlock()
	widget := &Widget{Type: "input", Text: placeholder}
	w.Widgets = append(w.Widgets, widget)
	return widget
}

// AddCheckbox adds a toggleable checkbox.
func (w *NativeWindow) AddCheckbox(text string, checked bool) *Widget {
	w.mu.Lock()
	defer w.mu.Unlock()
	val := "false"
	if checked {
		val = "true"
	}
	widget := &Widget{Type: "checkbox", Text: text, Value: val}
	w.Widgets = append(w.Widgets, widget)
	return widget
}

// AddProgressBar adds a progress bar with an initial percent value (0-100).
func (w *NativeWindow) AddProgressBar(percent int) *Widget {
	w.mu.Lock()
	defer w.mu.Unlock()
	widget := &Widget{Type: "progress", Value: strconv.Itoa(percent)}
	w.Widgets = append(w.Widgets, widget)
	return widget
}

// Run renders and displays the window on the desktop using native OS controls.
func (w *NativeWindow) Run() error {
	if runtime.GOOS == "windows" {
		return w.runWindowsForm()
	}
	return w.runLinuxForm()
}

// runWindowsForm creates and renders a native Windows desktop GUI using System.Windows.Forms.
func (w *NativeWindow) runWindowsForm() error {
	var ps strings.Builder
	// Write UTF-8 BOM so PowerShell properly renders Cyrillic and all Unicode text
	ps.WriteString("\xEF\xBB\xBF")
	ps.WriteString("Add-Type -AssemblyName System.Windows.Forms\n")
	ps.WriteString("Add-Type -AssemblyName System.Drawing\n")
	ps.WriteString(fmt.Sprintf("$form = New-Object System.Windows.Forms.Form\n"))
	ps.WriteString(fmt.Sprintf("$form.Text = '%s'\n", escapePS(w.Title)))
	ps.WriteString(fmt.Sprintf("$form.Size = New-Object System.Drawing.Size(%d, %d)\n", w.Width, w.Height))
	ps.WriteString("$form.StartPosition = 'CenterScreen'\n")
	ps.WriteString("$form.FormBorderStyle = 'FixedDialog'\n")
	ps.WriteString("$form.MaximizeBox = $false\n")
	ps.WriteString("$form.TopMost = $true\n")
	ps.WriteString("$form.Add_Shown({$form.Activate()})\n")

	yPos := 20
	for idx, wg := range w.Widgets {
		switch wg.Type {
		case "label":
			name := fmt.Sprintf("$lbl_%d", idx)
			ps.WriteString(fmt.Sprintf("%s = New-Object System.Windows.Forms.Label\n", name))
			ps.WriteString(fmt.Sprintf("%s.Text = '%s'\n", name, escapePS(wg.Text)))
			ps.WriteString(fmt.Sprintf("%s.Location = New-Object System.Drawing.Point(30, %d)\n", name, yPos))
			ps.WriteString(fmt.Sprintf("%s.Size = New-Object System.Drawing.Size(%d, 25)\n", name, w.Width-80))
			ps.WriteString(fmt.Sprintf("$form.Controls.Add(%s)\n", name))
			yPos += 35

		case "button":
			name := fmt.Sprintf("$btn_%d", idx)
			ps.WriteString(fmt.Sprintf("%s = New-Object System.Windows.Forms.Button\n", name))
			ps.WriteString(fmt.Sprintf("%s.Text = '%s'\n", name, escapePS(wg.Text)))
			ps.WriteString(fmt.Sprintf("%s.Location = New-Object System.Drawing.Point(30, %d)\n", name, yPos))
			ps.WriteString(fmt.Sprintf("%s.Size = New-Object System.Drawing.Size(%d, 35)\n", name, w.Width-80))
			ps.WriteString(fmt.Sprintf("%s.Add_Click({ [System.Windows.Forms.MessageBox]::Show('Action executed: %s', '%s', [System.Windows.Forms.MessageBoxButtons]::OK, [System.Windows.Forms.MessageBoxIcon]::Information) })\n",
				name, escapePS(wg.Text), escapePS(w.Title)))
			ps.WriteString(fmt.Sprintf("$form.Controls.Add(%s)\n", name))
			yPos += 45

		case "input":
			name := fmt.Sprintf("$txt_%d", idx)
			ps.WriteString(fmt.Sprintf("%s = New-Object System.Windows.Forms.TextBox\n", name))
			ps.WriteString(fmt.Sprintf("%s.Text = '%s'\n", name, escapePS(wg.Text)))
			ps.WriteString(fmt.Sprintf("%s.Location = New-Object System.Drawing.Point(30, %d)\n", name, yPos))
			ps.WriteString(fmt.Sprintf("%s.Size = New-Object System.Drawing.Size(%d, 25)\n", name, w.Width-80))
			ps.WriteString(fmt.Sprintf("$form.Controls.Add(%s)\n", name))
			yPos += 35

		case "checkbox":
			name := fmt.Sprintf("$chk_%d", idx)
			ps.WriteString(fmt.Sprintf("%s = New-Object System.Windows.Forms.CheckBox\n", name))
			ps.WriteString(fmt.Sprintf("%s.Text = '%s'\n", name, escapePS(wg.Text)))
			ps.WriteString(fmt.Sprintf("%s.Checked = $%s\n", name, wg.Value))
			ps.WriteString(fmt.Sprintf("%s.Location = New-Object System.Drawing.Point(30, %d)\n", name, yPos))
			ps.WriteString(fmt.Sprintf("%s.Size = New-Object System.Drawing.Size(%d, 25)\n", name, w.Width-80))
			ps.WriteString(fmt.Sprintf("$form.Controls.Add(%s)\n", name))
			yPos += 35

		case "progress":
			name := fmt.Sprintf("$prg_%d", idx)
			ps.WriteString(fmt.Sprintf("%s = New-Object System.Windows.Forms.ProgressBar\n", name))
			ps.WriteString(fmt.Sprintf("%s.Location = New-Object System.Drawing.Point(30, %d)\n", name, yPos))
			ps.WriteString(fmt.Sprintf("%s.Size = New-Object System.Drawing.Size(%d, 20)\n", name, w.Width-80))
			ps.WriteString(fmt.Sprintf("%s.Value = %s\n", name, wg.Value))
			ps.WriteString(fmt.Sprintf("$form.Controls.Add(%s)\n", name))
			yPos += 30
		}
	}

	ps.WriteString("[void]$form.ShowDialog()\n")

	tmpScript := filepath.Join(os.TempDir(), fmt.Sprintf("alca_form_%d.ps1", time.Now().UnixNano()))
	if err := os.WriteFile(tmpScript, []byte(ps.String()), 0644); err != nil {
		return err
	}
	defer os.Remove(tmpScript)

	params := fmt.Sprintf("-NoProfile -STA -ExecutionPolicy Bypass -File \"%s\"", tmpScript)
	_, err := launchViaShellExecute("powershell.exe", params, true)
	if err != nil {
		cmd := exec.Command("powershell", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-File", tmpScript)
		return cmd.Run()
	}
	return nil
}

// runLinuxForm creates and renders a native Linux form window via zenity / dialogs.
func (w *NativeWindow) runLinuxForm() error {
	args := []string{"--forms", "--title=" + w.Title}
	for _, wg := range w.Widgets {
		switch wg.Type {
		case "label":
			args = append(args, "--text="+wg.Text)
		case "input":
			args = append(args, "--add-entry="+wg.Text)
		case "checkbox":
			args = append(args, "--add-password="+wg.Text)
		}
	}
	return exec.Command("zenity", args...).Run()
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

		app, err := LaunchWebview(title, target, w, h)
		if err != nil {
			return interpreter.NewError(500, err.Error())
		}

		webviewObj := interpreter.NewMap()
		webviewObj.Set("url", &interpreter.StringValue{Val: app.ServerURL})
		webviewObj.Set("title", &interpreter.StringValue{Val: app.Title})
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

	// 3. Native Windows & Widgets
	register("window", func(args ...interpreter.Value) interpreter.Value {
		title := "Alca Desktop Window"
		w := 500
		h := 400
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

		win := NewNativeWindow(title, w, h)
		winObj := interpreter.NewMap()
		winObj.Set("title", &interpreter.StringValue{Val: title})

		winObj.Set("add_label", &interpreter.BuiltinFunction{
			Name: "add_label",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				text := ""
				if len(a) > 0 {
					text = interpreter.CoerceToString(a[0])
				}
				win.AddLabel(text)
				return winObj
			},
		})

		winObj.Set("add_button", &interpreter.BuiltinFunction{
			Name: "add_button",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				text := "Button"
				var cb interpreter.Value
				if len(a) > 0 {
					text = interpreter.CoerceToString(a[0])
				}
				if len(a) > 1 {
					cb = a[1]
				}
				win.AddButton(text, cb)
				return winObj
			},
		})

		winObj.Set("add_input", &interpreter.BuiltinFunction{
			Name: "add_input",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				placeholder := ""
				if len(a) > 0 {
					placeholder = interpreter.CoerceToString(a[0])
				}
				win.AddInput(placeholder)
				return winObj
			},
		})

		winObj.Set("add_checkbox", &interpreter.BuiltinFunction{
			Name: "add_checkbox",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				text := ""
				checked := false
				if len(a) > 0 {
					text = interpreter.CoerceToString(a[0])
				}
				if len(a) > 1 {
					checked = interpreter.IsTruthy(a[1])
				}
				win.AddCheckbox(text, checked)
				return winObj
			},
		})

		winObj.Set("add_progress", &interpreter.BuiltinFunction{
			Name: "add_progress",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				pct := 50
				if len(a) > 0 {
					if iv, ok := a[0].(*interpreter.IntegerValue); ok {
						pct = int(iv.Val)
					}
				}
				win.AddProgressBar(pct)
				return winObj
			},
		})

		winObj.Set("run", &interpreter.BuiltinFunction{
			Name: "run",
			Fn: func(a ...interpreter.Value) interpreter.Value {
				if err := win.Run(); err != nil {
					return interpreter.NewError(500, err.Error())
				}
				return interpreter.TRUE
			},
		})

		winObj.Set("show", winObj.Pairs["run"])

		return winObj
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
