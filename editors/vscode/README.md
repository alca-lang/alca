# Pawl Language Support for Visual Studio Code & Cursor

Official Visual Studio Code and Cursor extension for the **Pawl Programming Language** (`.al`).

## 🚀 Features

- **Complete syntax highlighting**: every Pawl keyword, including classes and inheritance (`class`, `extends`, `public`, `private`, `const`), control flow, loops, concurrency (`ba`, `threads`, `wait`, `watch`), tests (`test`, `mock`, `assert`), transactions (`tx`), errors (`err`), and imports.
- **Rich literal support**: single/double/triple-quoted strings, raw strings (`r"..."`, backticks), formatted strings (`f"Hello {name}!"`, `rf"..."`) with interpolation highlighting, escapes, durations (`500ms`, `2s`), sizes (`64KB`, `10MB`), numbers with separators, and all stream operators (`>>`, `>!`, `>>>`, `?>>>`, `<<`, `|>`).
- **LSP-powered editor support**: completion, hover, and live diagnostics from `pawl lsp` (syntax errors, type errors, class members, standard modules, and built-ins).
- **CLI fallback diagnostics**: if the language server is unavailable, the extension runs `pawl check` on the in-editor buffer so unsaved edits are checked too.
- **Formatting**: `Format Document` and `Pawl: Format File` run `pawl fmt`; formatting is idempotent and never rewrites a file with syntax errors.
- **Code snippets**: functions, classes, inheritance, methods, public/private/const fields, nested structs, lambdas, variables (`:=`, `T | null`), if/elif/else, loops, `match`, baroutines, errors (`err` / `?>>>`), threads, tests, SQLite (`std.db`), Prometheus metrics, and structured logging.
- **Status bar**: shows whether the language server or CLI diagnostics are active. Click it to check the current file.
- **Explorer and editor context menus** for running, testing, building, checking, and formatting `.al` files.

## 📋 Commands

- `Pawl: Run Script (Interpreter)`
- `Pawl: Run on Bytecode VM`
- `Pawl: Build Standalone Executable`
- `Pawl: Check Syntax & Types`
- `Pawl: Run Tests`
- `Pawl: Format File`
- `Pawl: Start REPL`
- `Pawl: Show Output`
- `Pawl: Restart Language Server`

## ⚙️ Settings

| Setting | Default | Description |
| --- | --- | --- |
| `pawl.executablePath` | `pawl` | Path to the Pawl CLI. Set an absolute path if `pawl` is not on `PATH`. |
| `pawl.useLanguageServer` | `true` | Use `pawl lsp` for completion, hover, and live diagnostics. |
| `pawl.diagnostics.onOpen` | `true` | Run CLI diagnostics when a file is opened (fallback mode). |
| `pawl.diagnostics.onSave` | `true` | Run CLI diagnostics when a file is saved (fallback mode). |
| `pawl.diagnostics.onType` | `true` | Run CLI diagnostics while typing (fallback mode). |
| `pawl.diagnostics.delay` | `600` | Delay in milliseconds before CLI diagnostics run after a change. |
| `pawl.saveBeforeRun` | `true` | Save before running, testing, checking, or building. |
| `pawl.showStatusBar` | `true` | Show the Pawl status bar item for `.al` files. |

## 📦 Installation

### Quick Install (Local)

Copy the extension folder into your VS Code extensions directory:

**Windows PowerShell:**
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.vscode\extensions\pawl-lang" -ErrorAction SilentlyContinue
Copy-Item -Recurse -Force "editors\vscode" "$env:USERPROFILE\.vscode\extensions\pawl-lang"
```

**Linux / macOS:**
```bash
cp -r editors/vscode ~/.vscode/extensions/pawl-lang
```

Reload VS Code (`Ctrl+Shift+P` → `Developer: Reload Window`).

The extension expects the `pawl` CLI on your `PATH` (`pawl version` to verify). If it is installed elsewhere, set `pawl.executablePath`.

## 📄 License

MIT License
