package stdlib

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// Widget represents a GUI control inside a native window.
type Widget struct {
	Type     string
	Text     string
	Value    string
	Callback interpreter.Value
	Index    int
}

// NativeWindow represents a desktop window with interactive widgets. The
// backend field selects how it is rendered:
//
//	"auto"   - native Win32 on Windows, Tcl/Tk or Qt elsewhere (default)
//	"native" - native Win32 controls
//	"tk"     - Tcl/Tk (wish)
//	"qt"     - Qt via PySide6/PyQt or QML
type NativeWindow struct {
	Title   string
	Width   int
	Height  int
	Widgets []*Widget
	Backend string
	mu      sync.Mutex
}

var (
	guiBackendMu      sync.RWMutex
	defaultGUIBackend = "auto"
)

// NewNativeWindow creates a new window specification.
func NewNativeWindow(title string, width, height int) *NativeWindow {
	if width <= 0 {
		width = 500
	}
	if height <= 0 {
		height = 400
	}
	guiBackendMu.RLock()
	backend := defaultGUIBackend
	guiBackendMu.RUnlock()
	return &NativeWindow{
		Title:   title,
		Width:   width,
		Height:  height,
		Widgets: make([]*Widget, 0),
		Backend: backend,
	}
}

// SetBackend selects the rendering backend ("auto", "native", "tk", "qt").
func (w *NativeWindow) SetBackend(backend string) *NativeWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	if backend == "" {
		backend = "auto"
	}
	w.Backend = strings.ToLower(strings.TrimSpace(backend))
	return w
}

// SetDefaultGUIBackend changes the backend used by subsequently created
// windows. Accepted values: auto, native, tk, tcl, qt.
func SetDefaultGUIBackend(backend string) {
	guiBackendMu.Lock()
	defer guiBackendMu.Unlock()
	if backend == "" || strings.EqualFold(backend, "auto") {
		defaultGUIBackend = "auto"
		return
	}
	defaultGUIBackend = strings.ToLower(strings.TrimSpace(backend))
}

// parseWindowArgs decodes the shared gui.window argument list:
// (title?, width?, height?, backend?).
func parseWindowArgs(args []interpreter.Value) (string, int, int, string) {
	title := "Alca Desktop Window"
	width, height := 500, 400
	backend := ""
	if len(args) >= 1 {
		title = interpreter.CoerceToString(args[0])
	}
	if len(args) >= 2 {
		if iv, ok := args[1].(*interpreter.IntegerValue); ok {
			width = int(iv.Val)
		}
	}
	if len(args) >= 3 {
		if iv, ok := args[2].(*interpreter.IntegerValue); ok {
			height = int(iv.Val)
		}
	}
	if len(args) >= 4 {
		backend = interpreter.CoerceToString(args[3])
	}
	return title, width, height, backend
}

// windowValue builds the script-visible window object used by gui.window,
// gui.tk_window and gui.qt_window.
func windowValue(title string, width, height int, backend string) interpreter.Value {
	win := NewNativeWindow(title, width, height)
	if backend != "" {
		win.SetBackend(backend)
	}

	winObj := interpreter.NewMap()
	winObj.Set("title", &interpreter.StringValue{Val: title})

	method := func(name string, fn func(a ...interpreter.Value) interpreter.Value) *interpreter.BuiltinFunction {
		builtin := &interpreter.BuiltinFunction{Name: name, Fn: fn}
		winObj.Set(name, builtin)
		return builtin
	}

	method("add_label", func(a ...interpreter.Value) interpreter.Value {
		text := ""
		if len(a) > 0 {
			text = interpreter.CoerceToString(a[0])
		}
		win.AddLabel(text)
		return winObj
	})

	method("add_button", func(a ...interpreter.Value) interpreter.Value {
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
	})

	method("add_input", func(a ...interpreter.Value) interpreter.Value {
		placeholder := ""
		if len(a) > 0 {
			placeholder = interpreter.CoerceToString(a[0])
		}
		win.AddInput(placeholder)
		return winObj
	})

	method("add_checkbox", func(a ...interpreter.Value) interpreter.Value {
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
	})

	method("add_progress", func(a ...interpreter.Value) interpreter.Value {
		percent := 50
		if len(a) > 0 {
			if iv, ok := a[0].(*interpreter.IntegerValue); ok {
				percent = int(iv.Val)
			}
		}
		win.AddProgressBar(percent)
		return winObj
	})

	method("set_backend", func(a ...interpreter.Value) interpreter.Value {
		if len(a) > 0 {
			win.SetBackend(interpreter.CoerceToString(a[0]))
		}
		return winObj
	})

	method("backend", func(a ...interpreter.Value) interpreter.Value {
		win.mu.Lock()
		name := win.Backend
		win.mu.Unlock()
		return &interpreter.StringValue{Val: name}
	})

	run := method("run", func(a ...interpreter.Value) interpreter.Value {
		if err := win.Run(); err != nil {
			return interpreter.NewError(500, err.Error())
		}
		return interpreter.TRUE
	})
	winObj.Set("show", run)

	return winObj
}

func (w *NativeWindow) addWidget(widget *Widget) *Widget {
	w.mu.Lock()
	defer w.mu.Unlock()
	widget.Index = len(w.Widgets)
	w.Widgets = append(w.Widgets, widget)
	return widget
}

// AddLabel adds a static text label.
func (w *NativeWindow) AddLabel(text string) *Widget {
	return w.addWidget(&Widget{Type: "label", Text: text})
}

// AddButton adds a clickable button with an optional action callback.
func (w *NativeWindow) AddButton(text string, callback interpreter.Value) *Widget {
	return w.addWidget(&Widget{Type: "button", Text: text, Callback: callback})
}

// AddInput adds a single-line text input field.
func (w *NativeWindow) AddInput(placeholder string) *Widget {
	return w.addWidget(&Widget{Type: "input", Text: placeholder})
}

// AddCheckbox adds a toggleable checkbox.
func (w *NativeWindow) AddCheckbox(text string, checked bool) *Widget {
	val := "false"
	if checked {
		val = "true"
	}
	return w.addWidget(&Widget{Type: "checkbox", Text: text, Value: val})
}

// AddProgressBar adds a progress bar with an initial percent value (0-100).
func (w *NativeWindow) AddProgressBar(percent int) *Widget {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return w.addWidget(&Widget{Type: "progress", Value: strconv.Itoa(percent)})
}

// Run renders the window and blocks until the user closes it.
func (w *NativeWindow) Run() error {
	backend := strings.ToLower(strings.TrimSpace(w.Backend))
	if backend == "" {
		backend = "auto"
	}
	switch backend {
	case "native", "win32", "windows":
		if runtime.GOOS == "windows" {
			return w.runNativeWindows()
		}
		return w.runAuto()
	case "tk", "tcl", "tcl/tk", "tcltk":
		return w.runTk()
	case "qt", "qt5", "qt6":
		return w.runQt()
	case "auto":
		return w.runAuto()
	default:
		return fmt.Errorf("unknown GUI backend %q (supported: auto, native, tk, qt)", w.Backend)
	}
}

func (w *NativeWindow) runAuto() error {
	if runtime.GOOS == "windows" {
		if err := w.runNativeWindows(); err == nil {
			return nil
		}
		// Fall through to the portable backends if the native window could not
		// be created for any reason.
	}
	if BackendAvailable("tk") {
		return w.runTk()
	}
	if BackendAvailable("qt") {
		return w.runQt()
	}
	return fmt.Errorf("no GUI backend is available: install Tcl/Tk (wish) or Qt (PySide6/PyQt) to use gui.window")
}

// -----------------------------------------------------------------------------
// Tcl/Tk backend
// -----------------------------------------------------------------------------

// BackendAvailable reports whether a GUI backend can be started on this host.
func BackendAvailable(name string) bool {
	switch strings.ToLower(name) {
	case "native", "win32", "windows":
		return runtime.GOOS == "windows"
	case "tk", "tcl", "tcltk":
		_, err := findTkInterpreter()
		return err == nil
	case "qt", "qt5", "qt6":
		_, err := findQtRunner()
		return err == nil
	case "auto":
		return runtime.GOOS == "windows" || BackendAvailable("tk") || BackendAvailable("qt")
	}
	return false
}

// BackendInfo returns availability information for every supported backend.
func BackendInfo() map[string]interface{} {
	return map[string]interface{}{
		"native":   runtime.GOOS == "windows",
		"tk":       BackendAvailable("tk"),
		"qt":       BackendAvailable("qt"),
		"auto":     BackendAvailable("auto"),
		"platform": runtime.GOOS,
	}
}

func findTkInterpreter() (string, error) {
	candidates := []string{"wish", "wish.exe", "wish8.6", "tclsh", "tclsh.exe", "tclsh8.6"}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	// Common Windows install locations (Git for Windows ships Tcl/Tk).
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			`C:\Program Files\Git\usr\bin\wish.exe`,
			`C:\Program Files\Git\mingw64\bin\wish.exe`,
			`C:\Program Files\Git\ucrt64\bin\wish.exe`,
			`C:\Tcl\bin\wish.exe`,
			`C:\Program Files\Tcl\bin\wish.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("Tcl/Tk interpreter not found (install Tcl/Tk and make 'wish' available in PATH)")
}

func tclQuote(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		`[`, `\[`,
		`]`, `\]`,
		"\r", `\r`,
		"\n", `\n`,
		"\t", `\t`,
	)
	return `"` + r.Replace(s) + `"`
}

func (w *NativeWindow) tkScript() string {
	var b strings.Builder
	b.WriteString("package require Tk\n")
	b.WriteString("wm title . " + tclQuote(w.Title) + "\n")
	b.WriteString(fmt.Sprintf("wm geometry . %dx%d\n", w.Width, w.Height))
	b.WriteString("wm minsize . 320 200\n")
	b.WriteString("set __alca_f .__alca_frame\n")
	b.WriteString("frame $__alca_f -padx 12 -pady 12\n")
	b.WriteString("pack $__alca_f -fill both -expand 1\n")
	for i, wg := range w.Widgets {
		name := fmt.Sprintf("$__alca_f.w%d", i)
		switch wg.Type {
		case "label":
			b.WriteString(fmt.Sprintf("label %s -text %s -anchor w -justify left\n", name, tclQuote(wg.Text)))
			b.WriteString(fmt.Sprintf("pack %s -fill x -pady 3\n", name))
		case "input":
			b.WriteString(fmt.Sprintf("entry %s\n", name))
			b.WriteString(fmt.Sprintf("pack %s -fill x -pady 3\n", name))
			if wg.Text != "" {
				b.WriteString(fmt.Sprintf("%s insert 0 %s\n", name, tclQuote(wg.Text)))
			}
		case "checkbox":
			initial := "0"
			if wg.Value == "true" {
				initial = "1"
			}
			b.WriteString(fmt.Sprintf("checkbutton %s -text %s -anchor w -variable __alca_chk%d\n", name, tclQuote(wg.Text), i))
			b.WriteString(fmt.Sprintf("set __alca_chk%d %s\n", i, initial))
			b.WriteString(fmt.Sprintf("pack %s -fill x -pady 3\n", name))
		case "progress":
			b.WriteString(fmt.Sprintf("ttk::progressbar %s -maximum 100 -value %s\n", name, strconv.Itoa(atoiDefault(wg.Value, 50))))
			b.WriteString(fmt.Sprintf("pack %s -fill x -pady 3\n", name))
		case "button":
			command := ""
			if wg.Callback != nil {
				command = fmt.Sprintf(`{puts "ALCA_CALL:%d"; flush stdout}`, i)
			}
			b.WriteString(fmt.Sprintf("button %s -text %s", name, tclQuote(wg.Text)))
			if command != "" {
				b.WriteString(" -command " + command)
			}
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("pack %s -fill x -pady 5\n", name))
		}
	}
	b.WriteString("wm protocol . WM_DELETE_WINDOW {exit}\n")
	b.WriteString("focus .\n")
	return b.String()
}

func (w *NativeWindow) runTk() error {
	interp, err := findTkInterpreter()
	if err != nil {
		return err
	}
	script := w.tkScript()
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("alca_tk_%d.tcl", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, []byte(script), 0o600); err != nil {
		return fmt.Errorf("cannot write Tcl/Tk script: %w", err)
	}
	defer os.Remove(tmp)

	cmd := exec.Command(interp, tmp)
	return w.runExternalUI(cmd, "Tcl/Tk")
}

// -----------------------------------------------------------------------------
// Qt backend
// -----------------------------------------------------------------------------

type qtRunner struct {
	executable string
	kind       string // "python" or "qml"
}

func pythonString(s string) string {
	// strconv.Quote produces an escape format understood by Python as well.
	return strconv.Quote(s)
}

func findQtRunner() (qtRunner, error) {
	for _, py := range []string{"python", "python3", "py"} {
		if _, err := exec.LookPath(py); err != nil {
			continue
		}
		for _, module := range []string{"PySide6", "PyQt6", "PyQt5"} {
			check := exec.Command(py, "-c", "import "+module)
			if check.Run() == nil {
				return qtRunner{executable: py, kind: "python:" + module}, nil
			}
		}
	}
	for _, qml := range []string{"qmlscene", "qml"} {
		if _, err := exec.LookPath(qml); err == nil {
			return qtRunner{executable: qml, kind: "qml"}, nil
		}
	}
	return qtRunner{}, fmt.Errorf("Qt backend not found (install PySide6/PyQt5: pip install PySide6, or Qt Quick 'qmlscene')")
}

func (w *NativeWindow) qtPythonScript(module string) string {
	var b strings.Builder
	b.WriteString("import sys\n")
	b.WriteString("from " + module + ".QtWidgets import (QApplication, QWidget, QVBoxLayout, QLabel, QLineEdit, QCheckBox, QProgressBar, QPushButton)\n")
	b.WriteString("app = QApplication(sys.argv)\n")
	b.WriteString("w = QWidget()\n")
	b.WriteString("w.setWindowTitle(" + pythonString(w.Title) + ")\n")
	b.WriteString(fmt.Sprintf("w.resize(%d, %d)\n", w.Width, w.Height))
	b.WriteString("layout = QVBoxLayout(w)\n")
	for i, wg := range w.Widgets {
		switch wg.Type {
		case "label":
			b.WriteString("layout.addWidget(QLabel(" + pythonString(wg.Text) + "))\n")
		case "input":
			b.WriteString("__in" + strconv.Itoa(i) + " = QLineEdit()\n")
			if wg.Text != "" {
				b.WriteString("__in" + strconv.Itoa(i) + ".setPlaceholderText(" + pythonString(wg.Text) + ")\n")
			}
			b.WriteString("layout.addWidget(__in" + strconv.Itoa(i) + ")\n")
		case "checkbox":
			checked := "False"
			if wg.Value == "true" {
				checked = "True"
			}
			b.WriteString("__cb" + strconv.Itoa(i) + " = QCheckBox(" + pythonString(wg.Text) + ")\n")
			b.WriteString("__cb" + strconv.Itoa(i) + ".setChecked(" + checked + ")\n")
			b.WriteString("layout.addWidget(__cb" + strconv.Itoa(i) + ")\n")
		case "progress":
			b.WriteString("__pb" + strconv.Itoa(i) + " = QProgressBar()\n")
			b.WriteString("__pb" + strconv.Itoa(i) + ".setMaximum(100)\n")
			b.WriteString("__pb" + strconv.Itoa(i) + ".setValue(" + strconv.Itoa(atoiDefault(wg.Value, 50)) + ")\n")
			b.WriteString("layout.addWidget(__pb" + strconv.Itoa(i) + ")\n")
		case "button":
			b.WriteString("__btn" + strconv.Itoa(i) + " = QPushButton(" + pythonString(wg.Text) + ")\n")
			if wg.Callback != nil {
				b.WriteString("__btn" + strconv.Itoa(i) + ".clicked.connect(lambda _=False, i=" + strconv.Itoa(i) + ": (print('ALCA_CALL:%d' % i, flush=True)))\n")
			}
			b.WriteString("layout.addWidget(__btn" + strconv.Itoa(i) + ")\n")
		}
	}
	b.WriteString("w.show()\n")
	b.WriteString("sys.exit(app.exec())\n")
	return b.String()
}

func (w *NativeWindow) qtQMLScript() string {
	var b strings.Builder
	b.WriteString("import QtQuick 2.15\n")
	b.WriteString("import QtQuick.Controls 2.15\n")
	b.WriteString("import QtQuick.Layouts 1.15\n")
	b.WriteString("ApplicationWindow {\n")
	b.WriteString("    visible: true\n")
	b.WriteString(fmt.Sprintf("    width: %d\n    height: %d\n", w.Width, w.Height))
	b.WriteString("    title: " + jsQuote(w.Title) + "\n")
	b.WriteString("    ColumnLayout {\n")
	b.WriteString("        anchors.fill: parent\n")
	b.WriteString("        anchors.margins: 12\n")
	b.WriteString("        spacing: 8\n")
	for i, wg := range w.Widgets {
		switch wg.Type {
		case "label":
			b.WriteString("        Label { text: " + jsQuote(wg.Text) + "; wrapMode: Text.WordWrap; Layout.fillWidth: true }\n")
		case "input":
			b.WriteString("        TextField { placeholderText: " + jsQuote(wg.Text) + "; Layout.fillWidth: true }\n")
		case "checkbox":
			checked := "false"
			if wg.Value == "true" {
				checked = "true"
			}
			b.WriteString("        CheckBox { text: " + jsQuote(wg.Text) + "; checked: " + checked + "; Layout.fillWidth: true }\n")
		case "progress":
			b.WriteString("        ProgressBar { from: 0; to: 100; value: " + jsQuote(wg.Value) + "; Layout.fillWidth: true }\n")
		case "button":
			b.WriteString("        Button { text: " + jsQuote(wg.Text) + "; Layout.fillWidth: true")
			if wg.Callback != nil {
				b.WriteString(fmt.Sprintf("; onClicked: console.log(%s)", jsQuote(fmt.Sprintf("ALCA_CALL:%d", i))))
			}
			b.WriteString(" }\n")
		}
	}
	b.WriteString("    }\n}\n")
	return b.String()
}

func jsQuote(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`, `"`, `\"`, "\r", `\r`, "\n", `\n`, "\t", `\t`,
	)
	return `"` + r.Replace(s) + `"`
}

func (w *NativeWindow) runQt() error {
	runner, err := findQtRunner()
	if err != nil {
		return err
	}
	if strings.HasPrefix(runner.kind, "python:") {
		module := strings.TrimPrefix(runner.kind, "python:")
		script := w.qtPythonScript(module)
		tmp := filepath.Join(os.TempDir(), fmt.Sprintf("alca_qt_%d.py", time.Now().UnixNano()))
		if err := os.WriteFile(tmp, []byte(script), 0o600); err != nil {
			return fmt.Errorf("cannot write Qt script: %w", err)
		}
		defer os.Remove(tmp)
		return w.runExternalUI(exec.Command(runner.executable, tmp), "Qt")
	}

	script := w.qtQMLScript()
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("alca_qt_%d.qml", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, []byte(script), 0o600); err != nil {
		return fmt.Errorf("cannot write QML file: %w", err)
	}
	defer os.Remove(tmp)
	return w.runExternalUI(exec.Command(runner.executable, tmp), "Qt/QML")
}

// -----------------------------------------------------------------------------
// External-process backends (Tcl/Tk and Qt)
// -----------------------------------------------------------------------------

func (w *NativeWindow) runExternalUI(cmd *exec.Cmd, backend string) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s backend failed to start: %w", backend, err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		w.scanCallbackOutput(stdout, false)
	}()
	go func() {
		defer wg.Done()
		w.scanCallbackOutput(stderr, true)
	}()

	err = cmd.Wait()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("%s window exited with an error: %w", backend, err)
	}
	return nil
}

func (w *NativeWindow) scanCallbackOutput(r io.Reader, forward bool) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if idx, ok := parseCallbackLine(line); ok {
			w.invokeCallback(idx)
			continue
		}
		if forward && strings.TrimSpace(line) != "" {
			fmt.Fprintln(os.Stderr, line)
		}
	}
}

func parseCallbackLine(line string) (int, bool) {
	const prefix = "ALCA_CALL:"
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(line, prefix)
	if i := strings.IndexByte(rest, ' '); i >= 0 {
		rest = rest[:i]
	}
	idx, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return idx, true
}

func (w *NativeWindow) invokeCallback(idx int) {
	w.mu.Lock()
	var widget *Widget
	if idx >= 0 && idx < len(w.Widgets) {
		widget = w.Widgets[idx]
	}
	w.mu.Unlock()
	if widget == nil || widget.Callback == nil {
		return
	}
	res, err := interpreter.Invoke(widget.Callback, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gui callback error: %v\n", err)
		return
	}
	if ev, ok := res.(*interpreter.ErrorValue); ok {
		fmt.Fprintf(os.Stderr, "gui callback error: %s\n", ev.Message)
	}
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return n
	}
	return def
}
